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

func ParseNotWellformedTime(input string) time.Time {
	// we try incresingly less specific formats until something matches
	formats := []string{
		"2006-01-02T15:04:05Z07:00",
		"2006-01-02",
		"2006-01",
		"2006",
		"2006-1-2",
		"2006-1",
		"01/02/06",
		"1/2/06",
		"January 2, 2006",
		"January 2006",
	}

	for _, f := range formats {
		result, err := time.Parse(f, input)
		if err == nil {
			return result
		}
	}
	return time.Time{}
}

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
