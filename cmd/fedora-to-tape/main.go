//
// scans fedora and assigns items with no bendo id a bendo id.
//
// will also make a list of items that then need to be uploaded to fedora.
package main

import (
	"log"
	"os"
	"path/filepath"
	"strings"
	"time"

	"github.com/ndlib/curate-annote/internal/annote"
)

type target struct {
	Fedora           *annote.RemoteFedora
	ContentPath      string
	Since            time.Time
	BendoIDs         map[string]string // pid -> bendoID
	DefaultBendoItem string
	DefaultCount     int
	AssignItem       func(*annote.CurateItem, string) error
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
	// LAST-iARVEST file, if any.
	since := os.Getenv("SINCE")
	if since != "" {
		t.Since, _ = time.Parse(time.RFC3339, since)
	}

	if !t.Since.IsZero() {
		log.Println("Harvesting since", t.Since)
	}

	start := time.Now()
	err = t.Scan()
	if err != nil {
		log.Println(err)
	}

	// save new last-harvest file
	fname = filepath.Join(t.ContentPath, "LAST-HARVEST")
	os.WriteFile(fname, []byte(start.Format(time.RFC3339)), 0666)
}

func (t *target) Scan() error {
	return annote.HarvestCurateObjects(t.Fedora, t.Since, func(item annote.CurateItem) error {
		log.Println("----")
		bendoID, isnew, err := t.FindOrAssignByItem(item)
		if isnew {
			afmodel := item.FirstField("af-model")
			log.Println(item.PID, afmodel, bendoID)
		}
		return err
	})
}

// FindOrAssignByPID is a convinence function that loads an item from fedora
// and then calls FindOrAssignByItem().
func (t *target) FindOrAssignByPID(pid string) (string, bool, error) {
	if id, ok := t.BendoIDs[pid]; ok {
		return id, false, nil
	}

	// lazy initalize
	if t.BendoIDs == nil {
		t.BendoIDs = make(map[string]string)
	}

	// get the item
	item, err := annote.FetchOneCurateObject(t.Fedora, pid)
	if err != nil {
		return "", false, err
	}
	bendoID, isnew, err := t.FindOrAssignByItem(item)
	return bendoID, isnew, err
}

// FindOrAssignByItem will return the bendo item for this item, or if there
// is not one it will assign one by trying to group things together.
func (t *target) FindOrAssignByItem(item annote.CurateItem) (string, bool, error) {
	// lazy initalize
	if t.BendoIDs == nil {
		t.BendoIDs = make(map[string]string)
	}

	bendoItem := item.FirstField("bendo-item")
	if bendoItem != "" {
		t.BendoIDs[item.PID] = bendoItem
		return bendoItem, false, nil
	}
	bendoItem, err := t.Assign(item)
	if bendoItem == "" || err != nil {
		return "", false, err
	}
	// this is where to hook into new bendo items being assigned
	log.Println("Assign", item.PID, bendoItem)
	t.BendoIDs[item.PID] = bendoItem
	if t.AssignItem != nil {
		err = t.AssignItem(item, bendoItem)
	}
	return bendoItem, true, err
}

// Assign will return a suitable bendo identifier for a given item.
func (t *target) Assign(item annote.CurateItem) (string, error) {
	pid := item.PID
	shortpid := strings.TrimPrefix(pid, "und:")

	afmodel := item.FirstField("af-model")
	switch afmodel {
	case "Hydramata_Group", "Person", "Profile", "ProfileSection":
		// these kinds of items don't get saved to bendo
		return "", nil

	case "GenericFile", "LinkedResource":
		// group with the parent resource, if any
		parent := item.FirstField("isPartOf")
		if parent != "" {
			bendoitem, _, err := t.FindOrAssignByPID(parent)
			return bendoitem, err
		}
		fallthrough

	default:
	}

	// if item belongs to a collection, use that
	collection := item.FirstField("isMemberOfCollection")
	if collection != "" {
		bendoitem, _, err := t.FindOrAssignByPID(collection)
		return bendoitem, err
	}

	if afmodel == "LibraryCollection" {
		// collections without parents always go into their own bendo item
		return shortpid, nil
	}

	// Group items into a larger bendo item.
	//
	// After 100 objects we will start a new item.
	// This limit is completely arbitrary. Feel free to adjust.
	if t.DefaultBendoItem == "" || t.DefaultCount > 100 {
		t.DefaultBendoItem = shortpid
		t.DefaultCount = 0
	}
	t.DefaultCount++
	return t.DefaultBendoItem, nil
}
