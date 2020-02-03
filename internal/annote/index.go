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

func (e *ElasticSearcher) Search(q SearchQuery) (SearchResults, error) {
	return SearchResults{}, nil
}

func (e *ElasticSearcher) IndexRecord(item CurateItem) {
	// Todo: reformat the item since the JSON form of a CurateItem datatype
	// confuses elastic search.
	// make it more like a json object.
	b, err := json.Marshal(item.AsAlternate())
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
	start := time.Now()
	for {
		items := source.Batch()
		if len(items) == 0 {
			break
		}

		buf.Reset()
		for _, item := range items {
			b, err := json.Marshal(item.AsAlternate())
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
	log.Println("BulkIndex", count, "Items", time.Now().Sub(start))
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
