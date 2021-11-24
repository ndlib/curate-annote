// curate-export will harvest curate records from a fedora instance write them
// as JSON structures to a given directory. Unlike the f3cp tool, this tool
// understands the Curate object layout and will deconstruct a curate object's
// complicated datastream layout into a bunch of key-value pairs.
//
// All configuration is done using envrionment variables. It is intended to be
// called as a cron job, or to run inside a docker container.
//
// The envrionment variable FEDORA_PATH gives the URL to the fedora 3.X
// instance you want to get data from. For example:
//
// 	   FEDORA_PATH="https://fedoraAdmin@xxxx@fedoraprod.lc.nd.edu:8443/fedora/"
//
// The environment variable CONTENT_PATH gives the local directory to write all
// the metadata files (as "$PID") as well as any content files in fedora as
// "PID-content". Only the most recent version of a content file is written.
// Since fedora keeps ALL versions of a file there could be others, but based
// on how Curate uses fedora, there are additional versions of content files
// only rarely.
//
// The harvest can be either "everything" or all items changed since a given
// date. A harvest date can be given in a few ways:
//   * use the envrionment variable SINCE in the form "2022-10-11"
//   * If CONTENT_PATH is set, a file named "LAST-HARVEST" is read, if it exists
//     and that date contained in the file is used.
//
// USAGE
//
// To dump records and content files into the directory "stuff"
//
//	   env FEDORA_PATH="..." CONTENT_PATH="./stuff" ./curate-export
//
// To only harvest records and files changed since a given date
//
//		env FEDORA_PATH="..." CONTENT_PATH="./stuff" SINCE="2021-01-01T00:00:00Z" ./curate-export
//
package main

import (
	"encoding/json"
	"io"
	"log"
	"os"
	"path/filepath"
	"strings"
	"time"

	"github.com/ndlib/curate-annote/internal/annote"
)

type target struct {
	Fedora      *annote.RemoteFedora
	ContentPath string
	Since       time.Time
}

func main() {
	var err error
	t := &target{}
	fedoraPath := os.Getenv("FEDORA_PATH")
	if fedoraPath == "" {
		log.Println("The envrionment variable FEDORA_PATH is not set")
		// exit 1?
		return
	}
	t.Fedora = annote.NewRemote(fedoraPath)

	t.ContentPath = os.Getenv("CONTENT_PATH")
	if t.ContentPath == "" {
		log.Println("The envrionment variable CONTENT_PATH is not set")
		return
	}

	// is there a LAST-HARVEST file?
	fname := filepath.Join(t.ContentPath, "LAST-HARVEST")
	content, err := os.ReadFile(fname)
	if err == nil {
		t.Since, _ = time.Parse(time.RFC3339, string(content))
	}

	// do this after CONTENT_PATH so setting SINCE will override the
	// LAST-HARVEST file, if any.
	since := os.Getenv("SINCE")
	if since != "" {
		t.Since, _ = time.Parse(time.RFC3339, since)
	}

	if !t.Since.IsZero() {
		log.Println("Harvesting since", t.Since)
	}

	start := time.Now()
	err = t.Harvest()
	if err != nil {
		log.Println(err)
	}

	// save new last-harvest file
	fname = filepath.Join(t.ContentPath, "LAST-HARVEST")
	os.WriteFile(fname, []byte(start.Format(time.RFC3339)), 0666)
}

func (t *target) Harvest() error {
	return annote.HarvestCurateObjects(t.Fedora, t.Since, func(item annote.CurateItem) error {
		pid := item.PID
		log.Println(pid)
		shortpid := strings.TrimPrefix(pid, "und:")
		alt := item.AsAlternate()
		data, err := json.MarshalIndent(alt, "", "    ")
		if err != nil {
			return err
		}
		name := filepath.Join(t.ContentPath, shortpid)
		err = os.WriteFile(name, data, 0666)
		if err != nil {
			return err
		}

		location := item.FirstField("file-location")
		if !strings.HasPrefix(location, "und:") {
			// either the location is empty and there is no content datastream,
			// or it is stored on bendo so lets skip it
			return nil
		}

		name = filepath.Join(t.ContentPath, shortpid+"-content")
		f, err := os.Create(name)
		if err != nil {
			return err
		}
		defer f.Close()
		// get content from fedora...

		source, err := t.Fedora.GetDatastream(pid, "content")
		if err != nil {
			return err
		}
		defer source.Close()

		_, err = io.Copy(f, source)
		return err
	})
}
