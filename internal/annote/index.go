package annote

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"log"
	"net/http"
	"time"

	"github.com/elastic/go-elasticsearch"
	"github.com/elastic/go-elasticsearch/esapi"
)

type ElasticSearcher struct {
	Client *elasticsearch.Client
}

func NewElasticSearch(address string) Searcher {
	client, err := elasticsearch.NewClient(elasticsearch.Config{
		Addresses: []string{address},
		Transport: &http.Transport{
			ResponseHeaderTimeout: 10 * time.Second,
		},
	})
	if err != nil {
		log.Println(err)
	}
	return &ElasticSearcher{Client: client}
}

const (
	esMatchAllQuery = `
{
  "query": {
    "match_all": {}
  },
  "from": %d,
  "size": %d
}`

	esTermQuery = `
{
  "query": {
    "multi_match" : {
      "query" : %q,
      "fields" : [ "*" ]
    }
  },
  "from": %d,
  "size": %d
}`
)

type esSearchResults struct {
	Hits struct {
		Total struct {
			Value    int    `json:"value"`
			Relation string `json:"relation"`
		} `json:"total"`
		Hits []*Hit `json:"hits"`
	} `json:"hits"`
}

type Hit struct {
	PID    string                 `json:"_id"`
	Fields map[string]interface{} `json:"_source"`
}

func (e *ElasticSearcher) Search(q SearchQuery) (SearchResults, error) {
	buf := &bytes.Buffer{}
	if q.Query == "" {
		fmt.Fprintf(buf, esMatchAllQuery, q.Start, q.NumRows)
	} else {
		fmt.Fprintf(buf, esTermQuery, q.Query, q.Start, q.NumRows)
	}

	req := esapi.SearchRequest{
		Index: []string{"items"},
		Body:  buf,
		//StoredFields: []string{
		//	"PID",
		//	"dc:title",
		//	"af-model",
		//	"representative",
		//	"dc:creator",
		//	"dc:description",
		//	"dc:abstract",
		//	"dc:created",
		//	"dc:creator#administrative_unit",
		//},
	}
	response, err := req.Do(context.Background(), e.Client)
	if err != nil {
		log.Println("search:", err)
	}
	dec := json.NewDecoder(response.Body)
	results0 := &esSearchResults{}
	err = dec.Decode(results0)
	if err != nil {
		log.Println(err)
	}
	response.Body.Close()

	var out SearchResults
	out.Items = esToCurateItem(results0.Hits.Hits)
	out.Total = results0.Hits.Total.Value
	return out, nil
}

func esToCurateItem(hits []*Hit) []CurateItem {
	var out []CurateItem
	for _, doc := range hits {
		item := CurateItem{
			PID: doc.PID,
		}
		for k, v := range doc.Fields {
			vv := v.([]interface{})
			for i := range vv {
				vvv := vv[i].(string)
				item.Properties = append(item.Properties, Pair{
					Predicate: k,
					Object:    vvv,
				})
			}
		}
		out = append(out, item)
	}
	return out
}

func (e *ElasticSearcher) IndexRecord(item CurateItem) {
	// Todo: reformat the item since the JSON form of a CurateItem datatype
	// confuses elastic search.
	// make it more like a json object.
	alt := item.AsAlternate()
	alt = fixRecordForES(alt)
	if len(alt["PID"]) == 0 {
		// this record type is not indexed
		log.Println("Record", item.PID, "is not a type that is indexed")
		return
	}
	b, err := json.Marshal(alt)
	if err != nil {
		log.Println(err)
		return
	}

	log.Println(string(b))

	req := esapi.IndexRequest{
		Index:      "items",
		DocumentID: item.PID,
		Refresh:    "true",
		Body:       bytes.NewReader(b),
	}

	response, err := req.Do(context.Background(), e.Client)
	if err != nil {
		log.Println("index:", item.PID, err)
		return
	}
	defer response.Body.Close()
	log.Println(response)
}

var (
	esDatefields = []string{
		"dc:created",
		"dc:date",
		"dc:date#created",
		"dc:date#digitized",
		"dc:date#prior_publication", // not really a date field, but ES thinks it is
		"dc:dateSubmitted",
		"dc:issued",
		"dc:modified",
		"vracore:wasPresented",
	}

	esFieldsIndexed = []string{
		"PID",
		"af-model",
		"bendo-item",
		"bibo:eIssn",
		"bibo:isbn",
		"bibo:issue",
		"bibo:numPages",
		"bibo:pageEnd",
		"bibo:pageStart",
		"bibo:volume",
		"cd:subject#cpc",
		"cd:subject#uspc",
		"dc:abstract",
		"dc:alternate",
		"dc:alternative",
		"dc:availability",
		"dc:bibliographicCitation",
		"dc:contributor",
		"dc:contributor#advisor",
		"dc:contributor#author",
		"dc:contributor#curator",
		"dc:contributor#institution",
		"dc:contributor#repository",
		"dc:created",
		"dc:creator",
		"dc:creator#administrative_unit",
		"dc:creator#affiliation",
		"dc:creator#affilition",
		"dc:creator#author",
		"dc:creator#editor",
		"dc:creator#local",
		"dc:creator#organization",
		"dc:date",
		"dc:date#application",
		"dc:date#approved",
		"dc:date#created",
		"dc:date#digitized",
		"dc:date#prior_publication",
		"dc:dateCopyrighted",
		"dc:dateSubmitted",
		"dc:description",
		"dc:description#abstract",
		"dc:description#code_list",
		"dc:description#note",
		"dc:description#table_of_contents",
		"dc:description#technical",
		"dc:descriptions",
		"dc:extent",
		"dc:extent#claims",
		"dc:format#extent",
		"dc:identifier",
		"dc:identifier#doi",
		"dc:identifier#isbn",
		"dc:identifier#issn",
		"dc:identifier#local",
		"dc:identifier#other",
		"dc:identifier#other_application",
		"dc:identifier#patent",
		"dc:identifier#prior_publication",
		"dc:isVersionOf#edition",
		"dc:issued",
		"dc:language",
		"dc:modified",
		"dc:modifier",
		"dc:pubisher",
		"dc:publisher",
		"dc:publisher#country",
		"dc:realtion",
		"dc:related",
		"dc:relation",
		"dc:relation#ispartof",
		"dc:requires",
		"dc:rights",
		"dc:rights#permissions",
		"dc:rightsHolder",
		"dc:source",
		"dc:spatial",
		"dc:subject",
		"dc:subject#cpc",
		"dc:subject#ipc",
		"dc:subject#lcsh",
		"dc:subject#uspc",
		"dc:temporal",
		"dc:title",
		"dc:title#alternate",
		"dc:type",
		"dc:version#edition",
		"depositor",
		"ebucore:duration",
		"ebucore:hasGenre",
		"fedora-create",
		"fedora-modify",
		"isMemberOfCollection",
		"mrel:ive",
		"mrel:ivr",
		"mrel:performer",
		"mrel:prf",
		"mrel:ths",
		"ms:degree",
		"nd:alephIdentifier",
		"owner",
		"representative",
		"vracore:culturalContext",
		"vracore:material",
		"vracore:partnerInSetWith",
		"vracore:placeOfCreation",
		"vracore:placeOfDiscovery",
		"vracore:placeOfPublication",
		"vracore:wasPresented",
		"vracore:wasProduced",
	}

	esAFModelsIgnored = []string{
		"GenericFile",
		"Hydramata_Group",
		"LinkedResource",
		"Person",
		"Profile",
		"ProfileSection",
	}
)

// fixRecordForES takes an item and creates an elastic search record for it.
// The item might not be of a type that is indexed in ES, in which case the returned
// structure will be missing the "PID" key.
//
// There are too many strange fields with data that confuses elastic search since we
// did not predefine any fields and it is guessing which type each is.
// (e.g. dc:date#prior_publication on patents is a doozy)
//
// Since we are only using ES as an index, we alter records to make them fit what
// ES wants and to improve search recall.
//
// We also enrich the records to show which have annotations, which are local, etc.
func fixRecordForES(item CurateItemAlt) CurateItemAlt {
	result := make(CurateItemAlt)

	// Return early if this is an af-model we don't index
	if len(item["af-model"]) == 0 {
		log.Println("Record", item["PID"], "has no af-model")
		return result
	}
	afmodel := item["af-model"][0]
	for _, model := range esAFModelsIgnored {
		if model == afmodel {
			return result
		}
	}

	// copy the fields we care about to the new item
	for _, field := range esFieldsIndexed {
		list := item[field]
		if len(list) == 0 {
			continue
		}
		result[field] = list
	}

	// Try to normalize all the date fields.
	//
	// ElasticSearch complains about some of our date formats, e.g.
	// "2018-05-20Z" (the trailing Z), or "Sep-64".
	for _, field := range esDatefields {
		list := result[field]
		for i := 0; i < len(list); i++ {
			newtime := ParseNotWellformedTime(list[i])
			if newtime.IsZero() {
				list = append(list[:i], list[i+1:]...)
			} else {
				list[i] = newtime.Format("2006-01-02")
			}
		}
		if len(list) > 0 {
			result[field] = list
		} else {
			delete(result, field)
		}
	}

	return result
}

// A Batcher provides a way to iterate through a (potentially very large) set of
// CurateItems. Each time Batch() is called another subset of the set should be
// returned. The empty list is returned when there is nothing left and signals
// that the iteration is finished.
type Batcher interface {
	Batch() []CurateItem
}

// IndexBatch will index a lot of records. It is expected to be more
// effecient than calling IndexRecord on each record.
func (e *ElasticSearcher) IndexBatch(source Batcher) {
	// Index in batches of 1000 items at a time
	source = &Grouper{Goal: 1000, Source: source}

	buf := &bytes.Buffer{}
	count := 0
	skip := 0
	start := time.Now()
	for {
		items := source.Batch()
		if len(items) == 0 {
			break
		}

		buf.Reset()
		for _, item := range items {
			alt := item.AsAlternate()
			alt = fixRecordForES(alt)
			if len(alt["PID"]) == 0 {
				// this record type is not indexed
				skip++
				continue
			}
			b, err := json.Marshal(alt)
			if err != nil {
				log.Println(item.PID, err)
				continue
			}
			fmt.Fprintf(buf, `{"index":{"_id":"%s"}}`, item.PID)
			buf.WriteString("\n")
			buf.Write(b)
			buf.WriteString("\n")
			count++
		}
		// it would be nice to only set refresh to true on the
		// last iteration. but we don't know which iteration
		// will be the last beforehand.
		req := esapi.BulkRequest{
			Index:   "items",
			Refresh: "true",
			Body:    buf,
		}

		response, err := req.Do(context.Background(), e.Client)
		if err != nil {
			log.Println("batchindex:", err)
			continue
		}
		log.Println(response)
		response.Body.Close()
	}
	log.Println("BulkIndex", count, "Items Indexed", skip, "Skipped", time.Now().Sub(start))
}

// A Grouper wraps a source Batcher and always returns lists containing Goal
// number of items, except for the last one which may have less.
type Grouper struct {
	Goal   int
	Source Batcher
	done   bool
	extra  []CurateItem
}

func (g *Grouper) Batch() []CurateItem {
	if len(g.extra) == 0 && g.done {
		return nil
	}
	result := g.extra
	g.extra = nil
	// Add items to be returned until we either have enough or are finished.
	for len(result) < g.Goal && !g.done {
		items := g.Source.Batch()
		if len(items) == 0 {
			g.done = true
			break
		}
		result = append(result, items...)
	}
	if len(result) > g.Goal {
		g.extra = make([]CurateItem, len(result)-g.Goal)
		copy(g.extra, result[g.Goal:])
		result = result[:g.Goal]
	}
	return result
}

// AllItems is a Batcher that will return everything in the database. It
// returns Count items at a time, which if  equalt to 0 defaults to 500 items.
type AllItems struct {
	Offset int
	Count  int
}

func (a *AllItems) Batch() []CurateItem {
	if a.Count == 0 {
		a.Count = 500
	}
	result, err := Datasource.FindAllRange(a.Offset, a.Count)
	if err != nil {
		log.Println(err)
		return nil
	}
	a.Offset += a.Count
	return result
}
