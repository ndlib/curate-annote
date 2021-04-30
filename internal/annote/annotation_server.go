package annote

import (
	"bytes"
	"encoding/json"
	"errors"
	"log"
	"net/http"
	"time"
)

type SubmitRA struct {
	ID            string
	PID           string            `json:"pid"`
	Action        string            `json:"action"`
	RepositoryURL string            `json:"repository-url"`
	Title         string            `json:"title"`
	Description   string            `json:"description"`
	Owners        []string          `json:"owners"`
	DateCreated   time.Time         `json:"date-created"`
	Creators      []string          `json:"creators"`
	AccessView    []string          `json:"access-view"`
	AccessEdit    []string          `json:"access-edit"`
	Metadata      map[string]string `json:"metadata"`
}

type SubmitRAP struct {
	ID            string
	PID           string            `json:"pid"`
	Action        string            `json:"action"`
	RepositoryURL string            `json:"repository-url"`
	Title         string            `json:"title"`
	Description   string            `json:"description"`
	Owners        []string          `json:"owners"`
	DateCreated   time.Time         `json:"date-created"`
	Creators      []string          `json:"creators"`
	AccessView    []string          `json:"access-view"`
	AccessEdit    []string          `json:"access-edit"`
	Metadata      map[string]string `json:"metadata"`

	RA               string   `json:"ra"`
	Content          []string `json:"content"`
	ContentChecksums []string `json:"content-checksums"`
	ContentAccess    string   `json:"content-access"`
	Copyright        string   `json:"copyright"`
	License          string   `json:"license"`
}

type ItemUUID struct {
	Item     string
	Username string
	UUID     string
	Status   string
}

type AnnoStore struct {
	Host             string
	UsernamePassword string
	ImageViewerHost  string
	OurURL           string
}

var (
	AnnotationStore *AnnoStore
	ErrNoFiles      = errors.New("Item has no attached files")
)

func (as *AnnoStore) UploadItem(item CurateItem, uploader *User) (string, error) {
	files, err := Datasource.FindItemFiles(item.PID)
	if err != nil {
		return "", err
	}
	if len(files) == 0 {
		return "", ErrNoFiles
	}
	if uploader.ORCID == "" {
		return "Your profile doesn't have an OCRID. ORCID's are necessary to do annotations.", nil
	}
	rap := SubmitRAP{
		ID:            item.PID,
		PID:           item.FirstField("dc:identifier", "dc:identifier#doi"),
		Action:        "create",
		RepositoryURL: as.OurURL + "/show/" + item.PID,
		Title:         item.FirstField("dc:title"),
		Description:   item.FirstField("dc:description", "dc:abstract"),
		Owners:        []string{uploader.ORCID},
	}
	createdtext := item.FirstField("dc:created", "fedora-create")
	created := ParseNotWellformedTime(createdtext)
	if !created.IsZero() {
		rap.DateCreated = created
	}

	// todo: access permissions

	var content []string
	for _, file := range files {
		content = append(content, as.OurURL+"/downloads/"+file.PID)
	}
	rap.Content = content

	response, err := as.sendRAP(&rap)

	if err != nil || !response.Success {
		return response.Error, err
	}

	uuid := ItemUUID{
		Item:     item.PID,
		Username: uploader.Username,
		UUID:     response.Created.UUID,
		Status:   response.Created.Metadata.Status.Code,
	}

	err = Datasource.UpdateUUID(uuid)

	return "", err
}

func (as *AnnoStore) getJSON(url string, result interface{}) error {
	// HACK: return early if there is no annotation server
	if as.Host == "" {
		return nil
	}

	req, err := http.NewRequest("GET", url, nil)
	if err != nil {
		return err
	}
	req.Header.Set("Content-Type", "application/json")
	req.AddCookie(&http.Cookie{
		Name:  "s",
		Value: as.UsernamePassword,
	})

	if TimeoutClient == nil {
		TimeoutClient = &http.Client{
			Timeout: 60 * time.Minute,
		}
	}

	resp, err := TimeoutClient.Do(req)
	if err != nil {
		return err
	}
	defer resp.Body.Close()
	if resp.StatusCode != 200 {
		log.Println("GET", url, "returned", resp.Status)
	}
	decoder := json.NewDecoder(resp.Body)
	return decoder.Decode(result)
}

type rapResponse struct {
	Success bool       `json:"success"`
	Error   string     `json:"error"`
	Created rapCreated `json:"created"`
}

type rapCreated struct {
	UUID     string `json:"uuid"`
	Metadata struct {
		Status struct {
			Code        string `json:"code"`
			Description string `json:"desc"`
		} `json:"status"`
	} `json:"_metadata"`
}

func (as *AnnoStore) sendRAP(rap *SubmitRAP) (rapResponse, error) {
	var result rapResponse
	jsontext, err := json.Marshal(rap)

	body := bytes.NewReader(jsontext)
	url := as.Host + "/api/1/rap"
	req, err := http.NewRequest("POST", url, body)
	if err != nil {
		return result, err
	}
	req.Header.Set("Content-Type", "application/json")
	req.AddCookie(&http.Cookie{
		Name:  "s",
		Value: as.UsernamePassword,
	})

	if TimeoutClient == nil {
		TimeoutClient = &http.Client{
			Timeout: 60 * time.Minute,
		}
	}

	resp, err := TimeoutClient.Do(req)
	if err != nil {
		return result, err
	}
	defer resp.Body.Close()
	if resp.StatusCode != 200 {
		log.Println("POST", url, "returned", resp.Status)
	}
	// read the response...?
	decoder := json.NewDecoder(resp.Body)
	err = decoder.Decode(&result)

	return result, err
}

type RAPs struct {
	ID     string
	UUID   string `json:"uuid"`
	PID    string `json:"pid"`
	Status struct {
		Code        string `json:"code"`
		Description string `json:"desc"`
	} `json:"status"`
}

func (as *AnnoStore) RAPStatus() ([]RAPs, error) {
	var result []RAPs

	url := as.Host + "/api/1/rap"
	err := as.getJSON(url, &result)

	return result, err
}

func (as *AnnoStore) ViewerURL(uuid string) string {
	return as.ImageViewerHost + "/mirador/uuid/" + uuid
}

type QueryAnnotationList struct {
	Error       string `json:"error"`
	ID          string `json:"@id"`
	Annotations []struct {
		Type      string `json:"@type"`
		ID        string `json:"@id"`
		AnnoStore struct {
			RA string `json:"researchActivity"`
		} `json:"__annostore"`
		Author struct {
			ID   string `json:"@id"`
			Name string `json:"name"`
		} `json:"annotatedBy"`
		Created    time.Time `json:"annotatedAt"`
		Motivation []string  `json:"motivation"`
	} `json:"resources"`
}

func (as *AnnoStore) AnnotationListByUUID(uuid string) (QueryAnnotationList, error) {
	var result QueryAnnotationList

	url := as.Host + "/api/1/al/" + uuid
	err := as.getJSON(url, &result)

	return result, err
}

type rapWorkerResult struct {
	NumNotComplete int
	Err            error
}

// backgroundRAPchecker will poll the annotation server to track the statuses
// of RAP ingests. It adjusts its polling interval depending on whether there
// are any pending jobs. Send something on the channel c to notify the poller
// that there is a new pending job to monitor.
func (as *AnnoStore) BackgroundRAPchecker(c <-chan struct{}) {
	// We want to balance being timely with updates without polling the
	// annotation server too much. This is accomplished in two ways:
	// First, we only poll when there is an item that is not complete.
	// Second, we set a limit that we will not poll the annotation server
	// more frequently than.
	const minInterval = 30 * time.Second
	workerResult := make(chan rapWorkerResult)
	nextpoll := time.Now().Add(minInterval) // so even after restarts, we always wait the time period
	inDeepSleep := false
	for {
		d := minInterval
		if inDeepSleep {
			d = 24 * time.Hour
		}
	sleeploop:
		log.Println("@@@ sleeping", d)
		select {
		case <-c:
			// there is something we should monitor, so shorten our timeout
			inDeepSleep = false
			log.Println("@@@ receive signal")

		case <-time.After(d):
			log.Println("@@@ receive timer")
		}

		// if polling too soon, go back to sleep
		if time.Now().Before(nextpoll) {
			d = nextpoll.Sub(time.Now())
			goto sleeploop // so we can supply custom duration
		}

		// do a single poll....
		nextpoll = time.Now().Add(minInterval)
		go as.checkRAPs(workerResult)
		inDeepSleep = true
	keep_waiting:
		// wait for response
		log.Println("@@@ waiting for status")
		select {
		case <-c: // eat these so nothing else blocks
			log.Println("@@@@ receive signal")
			inDeepSleep = false
			goto keep_waiting
		case r := <-workerResult:
			if r.Err != nil || r.NumNotComplete > 0 {
				// need to do another poll
				inDeepSleep = false
			}
		}
	}
}

func (as *AnnoStore) checkRAPs(out chan<- rapWorkerResult) {
	log.Println("polling Annotation Store")
	raps, err := as.RAPStatus()
	if err != nil {
		log.Println(err)
		out <- rapWorkerResult{Err: err}
		return
	}
	UUIDs, err := Datasource.SearchItemUUID("", "", "")
	if err != nil {
		log.Println(err)
		out <- rapWorkerResult{Err: err}
		return
	}

	notComplete := 0
	var bigerr error
big_loop:
	for _, record := range UUIDs {
		if record.Status == "a" {
			// complete. skip it for now
			continue
		}
		// is there an entry for this UUID in that status update?
		log.Println("searching for", record)
		for i := range raps {
			if raps[i].UUID != record.UUID {
				continue
			}

			rap := raps[i]
			log.Println("found", rap)
			// also verify the ID...?
			if rap.Status.Code != "a" {
				notComplete++
			}
			if rap.Status.Code == record.Status {
				break // nothing to update
			}
			// update our record to the new status
			record.Status = rap.Status.Code
			err = Datasource.UpdateUUID(record)
			if err != nil {
				log.Println(err)
				bigerr = err
				continue big_loop
			}
			break
		}
	}

	out <- rapWorkerResult{NumNotComplete: notComplete, Err: bigerr}
}
