package annote

// This file implements the handlers needed for an annotot annotation server interface.
// The interface is defined by the implementation given in
//
// https://github.com/mejackreed/annotot.git

import (
	"encoding/json"
	"log"
	"net/http"
	"net/url"
	"time"

	"github.com/julienschmidt/httprouter"
)

type annototpages struct {
	ID    string            `json:"id"`
	Items []json.RawMessage `json:"items"`
	Type  string            `json:"type"`
}

type tots struct {
	ID         int       `json:"id,omitempty"`
	UUID       string    `json:"uuid"`
	Canvas     string    `json:"canvas"`
	Data       string    `json:"data"`
	Creator    string    `json:"creator,omitempty"`
	CreateDate time.Time `json:"create_date,omitempty"`
	ModifiedBy string    `json:"modified_by,omitempty"`
	ModifyDate time.Time `json:"modify_date,omitempty"`
}

// AnnototPages returns a list of annotations for a given object.
//
// GET	/annotot/pages
//
// The object is passed in with the `uri=` parameter. The result list
// contains the contents for all the annotations on that object.
func AnnototPages(w http.ResponseWriter, r *http.Request, ps httprouter.Params) {
	canvas := r.FormValue("uri")
	if canvas == "" {
		w.WriteHeader(404)
		return
	}

	// sanity checking on canvas?

	// search database
	tots, err := Datasource.TotsByCanvas(canvas)
	if err != nil {
		log.Println(err)
	}

	result := annototpages{
		ID:   canvas,
		Type: "AnnotationPage",
	}
	log.Println("tots found", len(tots))
	for _, t := range tots {
		// Annotot stores the annotation payload as a json string,
		// but wants it back as the json object that it is. SOOOO,
		// we can't return it as a string but instead need to
		// "promote" and stick it in as-is.
		result.Items = append(result.Items, json.RawMessage(t.Data))
	}

	w.Header().Add("Content-Type", "application/json")
	encoder := json.NewEncoder(w)
	err = encoder.Encode(result)
	if err != nil {
		log.Println(err)
	}
}

type annototcreate struct {
	Annotation tots `json:"annotation"`
}

// AnnototCreate creates a new annotation.
//
// POST /annotot
// body of request is the annotation in JSON
func AnnototCreate(w http.ResponseWriter, r *http.Request, ps httprouter.Params) {
	username, _, _ := r.BasicAuth()

	var input annototcreate
	decoder := json.NewDecoder(r.Body)
	err := decoder.Decode(&input)
	if err != nil {
		log.Println(err)
		w.WriteHeader(400)
		return
	}

	t := time.Now()
	input.Annotation.Creator = username
	input.Annotation.CreateDate = t
	input.Annotation.ModifiedBy = username
	input.Annotation.ModifyDate = t

	// URI unencode input.Canvas
	input.Annotation.Canvas, err = url.PathUnescape(input.Annotation.Canvas)
	if err != nil {
		log.Println(err)
	}

	Datasource.TotCreate(input.Annotation)
}

// AnnototUpdate updates a given annotation
//
// PATCH /annotot/:uuid
// the body is an annotation as JSON, just as with AnnototCreate
func AnnototUpdate(w http.ResponseWriter, r *http.Request, ps httprouter.Params) {
	username, _, _ := r.BasicAuth()

	uuid := ps.ByName("uuid")
	var input annototcreate
	decoder := json.NewDecoder(r.Body)
	err := decoder.Decode(&input)
	if err != nil {
		log.Println(err)
		w.WriteHeader(400)
		return
	}
	if uuid != input.Annotation.UUID {
		log.Println("mismatch uuid", uuid, input.Annotation.UUID)
		w.WriteHeader(400)
		return
	}

	t := time.Now()
	input.Annotation.ModifiedBy = username
	input.Annotation.ModifyDate = t

	Datasource.TotUpdateData(input.Annotation)
}

// AnnototList displays all the tots in the database
//
// GET /annotot
// This returns HTML
func AnnototListAll(w http.ResponseWriter, r *http.Request, ps httprouter.Params) {
	// search database
	tots, err := Datasource.TotsByCanvas("")
	if err != nil {
		log.Println(err)
	}

	log.Println("tots found", len(tots))
	DoTemplate(w, "tots-listall", tots)
}
