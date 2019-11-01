package annote

import (
	"fmt"
	"html/template"
	"log"
	"net/http"
	"path/filepath"
	"regexp"
	"strconv"
	"strings"
	"time"

	"github.com/julienschmidt/httprouter"
)

var (
	Templates      *template.Template
	Datasource     *MysqlDB
	TargetFedora   *RemoteFedora
	StaticFilePath string
	CurateURL      string
	unicodeEscape  = regexp.MustCompile(`\\u\w{4,6}`)
)

// isPID returns true if the given string has the form of a Curate PID.
func isPID(s string) bool {
	// we could be more detailed since the id has a specific numeral/letter
	// ordering, but that seems like overkill
	return strings.HasPrefix(s, "und:")
}

func isURL(s string) bool {
	return strings.HasPrefix(s, "http")
}

func isCompound(s string) bool {
	return strings.HasPrefix(s, "^^")
}

func decodeUnicode(s string) string {
	return unicodeEscape.ReplaceAllStringFunc(s, func(z string) string {
		z, _ = strconv.Unquote("'" + z + "'")
		return z
	})
}

func splitCompound(s string) map[string]string {
	if len(s) < 2 || s[:2] != "^^" {
		return nil
	}
	result := make(map[string]string)
	for _, piece := range strings.Split(s[2:], "^^") {
		i := strings.Index(piece, " ")
		if i < len(piece)+1 {
			result[piece[:i]] = piece[i+1:]
		}
	}
	return result
}

func AttachedFiles(pid string) []CurateItem {
	items, err := Datasource.FindItemFiles(pid)
	if err != nil {
		log.Println(err)
	}
	return items
}

func CollectionMembers(pid string) []CurateItem {
	items, err := Datasource.FindCollectionMembers(pid)
	if err != nil {
		log.Println(err)
	}
	return items
}

func firstField(c CurateItem, targets ...string) string {
	for _, target := range targets {
		for i := range c.Properties {
			if c.Properties[i].Predicate == target {
				return c.Properties[i].Object
			}
		}
	}
	return ""
}

func allFields(c CurateItem, targets ...string) []string {
	var result []string
	for i := range c.Properties {
		for _, target := range targets {
			if c.Properties[i].Predicate == target {
				result = append(result, c.Properties[i].Object)
			}
		}
	}
	return result
}

func configValue(key string) string {
	v, err := Datasource.ReadConfig(key)
	if err != nil {
		log.Println(key, err)
	}
	return v
}

// LoadTemplates will load and compile our templates into memory
func LoadTemplates(path string) error {
	t := template.New("")
	t = t.Funcs(template.FuncMap{
		"isPID":             isPID,
		"isURL":             isURL,
		"isCompound":        isCompound,
		"splitCompound":     splitCompound,
		"decodeUnicode":     decodeUnicode,
		"AttachedFiles":     AttachedFiles,
		"CollectionMembers": CollectionMembers,
		"FirstField":        firstField,
		"AllFields":         allFields,
		"ConfigValue":       configValue,
	})
	t, err := t.ParseGlob(filepath.Join(path, "*"))
	Templates = t
	return err
}

func notFound(w http.ResponseWriter) {
	w.WriteHeader(http.StatusNotFound)
	err := Templates.ExecuteTemplate(w, "404", nil)
	if err != nil {
		log.Println(err)
	}
}

func serverError(w http.ResponseWriter) {
	w.WriteHeader(http.StatusInternalServerError)
	err := Templates.ExecuteTemplate(w, "500", nil)
	if err != nil {
		log.Println(err)
	}
}

func NotImplemented(w http.ResponseWriter, r *http.Request, ps httprouter.Params) {
	w.WriteHeader(http.StatusNotImplemented)
	err := Templates.ExecuteTemplate(w, "500", nil)
	if err != nil {
		log.Println(err)
	}
}

// IndexHandler responds to the root route.
func IndexHandler(w http.ResponseWriter, r *http.Request, ps httprouter.Params) {
	err := Templates.ExecuteTemplate(w, "homepage", nil)
	if err != nil {
		log.Println(err)
	}
}

func GetObject(w http.ResponseWriter, r *http.Request, ps httprouter.Params) {
	pid := ps.ByName("id")
	item, err := Datasource.FindItem(pid)
	if err != nil {
		w.WriteHeader(500)
		fmt.Fprintln(w, err)
		return
	}
	err = Templates.ExecuteTemplate(w, "item", item)
	if err != nil {
		log.Println(err)
	}
}

func ConfigPage(w http.ResponseWriter, r *http.Request, ps httprouter.Params) {
	err := Templates.ExecuteTemplate(w, "config", nil)
	if err != nil {
		log.Println(err)
	}
}

func UpdateConfig(w http.ResponseWriter, r *http.Request, ps httprouter.Params) {
	if harvestStatus == StatusWaiting {
		harvestControl <- HNow
	}

	ConfigPage(w, r, ps)
}

func ObjectShow(w http.ResponseWriter, r *http.Request, ps httprouter.Params) {
	pid := ps.ByName("id")
	if !strings.HasPrefix(pid, "und:") {
		pid = "und:" + pid
	}
	item, err := Datasource.FindItem(pid)
	if err != nil {
		w.WriteHeader(500)
		fmt.Fprintln(w, err)
		return
	}
	err = Templates.ExecuteTemplate(w, "show", item)
	if err != nil {
		log.Println(err)
	}
}

func ObjectDownload(w http.ResponseWriter, r *http.Request, ps httprouter.Params) {
	pid := ps.ByName("id")
	pid = strings.TrimPrefix(pid, "und:")

	// is file cached?
	path := FindFileInCache(pid)
	if path == "" {
		err := DownloadFileToCache(pid, fmt.Sprintf("%s/downloads/%s", CurateURL, pid))
		if err != nil {
			w.WriteHeader(500)
			fmt.Fprintln(w, err)
			return
		}
		path = FindFileInCache(pid)
	}
	http.ServeFile(w, r, path)
}

func ObjectDownloadThumbnail(w http.ResponseWriter, r *http.Request, ps httprouter.Params) {
	pid := ps.ByName("id")
	pid = strings.TrimPrefix(pid, "und:")

	// is file cached?
	basename := pid + "-thumbnail"
	path := FindFileInCache(basename)
	if path == "" {
		err := DownloadFileToCache(basename, fmt.Sprintf("%s/downloads/%s/thumbnail", CurateURL, pid))
		if err != nil {
			w.WriteHeader(500)
			fmt.Fprintln(w, err)
			return
		}
		path = FindFileInCache(basename)
	}
	http.ServeFile(w, r, path)

	// get it
}

//
// Harvester
//

var (
	harvestControl chan int
	// should have a mutex protecting it
	harvestStatus int
)

const (
	HNow = iota
	HExit

	StatusWaiting = iota
	StatusHarvesting
)

func BackgroundHarvester() {
	var lastHarvest time.Time
	var harvestInterval time.Duration
	s, err := Datasource.ReadConfig("last-harvest")
	if err == nil {
		lastHarvest, _ = time.Parse(time.RFC3339, s)
	}
	s, err = Datasource.ReadConfig("harvest-interval")
	if err == nil {
		harvestInterval, _ = time.ParseDuration(s)
	}

	harvestControl = make(chan int, 100)

	for {
		harvestStatus = StatusWaiting
		var timer <-chan time.Time
		if harvestInterval > 0 {
			timer = time.After(harvestInterval)
		}
		select {
		case msg := <-harvestControl:
			if msg == HExit {
				return
			}
		case <-timer:
		}
		log.Println("Start Harvest since", lastHarvest)
		harvestStatus = StatusHarvesting
		t := time.Now()
		c := make(chan CurateItem, 10)
		go func() {
			for item := range c {
				err := Datasource.IndexItem(item)
				if err != nil {
					log.Println(err)
				}
			}
		}()
		err := HarvestCurateObjects(TargetFedora, lastHarvest, func(item CurateItem) error {
			c <- item
			return nil
		})

		if err != nil {
			log.Println(err)
		} else {
			lastHarvest = t
			Datasource.SetConfig("last-harvest", t.Format(time.RFC3339))
		}
		log.Println("Finish Harvest")
	}
}
