package annote

import (
	"encoding/json"
	"log"
	"net/http"
	"net/url"
	"strconv"
	"strings"
	"time"
)

type SolrQuery struct {
	Query       string
	QueryFields []string
	FieldList   []string
	Start       int
	Rows        int
	Sort        []string
	FilterQuery []string
	Facets      []string
}

type SolrInfo struct {
	Host string
}

var (
	Solr *SolrInfo
)

type SolrResponse struct {
	Response struct {
		NumFound int                       `json:"numFound"`
		Start    int                       `json:"start"`
		MaxScore float32                   `json:"maxScore"`
		Docs     []map[string]StringOrList `json:"docs"`
	} `json:"response"`
	//Facets struct {
	//	FacetFields map[string][]StringCount `json:"facet_fields"`
	//} `json:"facet_counts"`
}

// StringOrList is used to decode json documents having values
// that may either be a string or a list of strings. It makes
// everything a list of strings, so that we have a consistent
// data type.
type StringOrList []string

func (x *StringOrList) UnmarshalJSON(b []byte) error {
	// is it a list?
	if b[0] == '[' {
		return json.Unmarshal(b, (*[]string)(x))
	}
	// perhaps a string?
	if b[0] == '"' {
		var s string
		err := json.Unmarshal(b, &s)
		if err != nil {
			return err
		}
		*x = []string{s}
		return nil
	}
	// maybe it is a number??
	var v float64
	err := json.Unmarshal(b, &v)
	if err != nil {
		return err
	}
	*x = []string{strconv.FormatFloat(v, 'f', 7, 64)}
	return nil
}

func (si *SolrInfo) SendQuery(q SolrQuery) (SolrResponse, error) {
	var result SolrResponse
	params := url.Values{}

	params.Add("q", q.Query)
	if len(q.QueryFields) > 0 {
		params.Add("qf", strings.Join(q.QueryFields, " "))
	}
	if len(q.FieldList) > 0 {
		params.Add("fl", strings.Join(q.FieldList, ","))
	}
	params.Add("wt", "json")
	params.Add("start", strconv.Itoa(q.Start))
	params.Add("rows", strconv.Itoa(q.Rows))
	params.Add("sort", strings.Join(q.Sort, ", "))
	for _, fq := range q.FilterQuery {
		params.Add("fq", fq)
	}
	for _, facet := range q.Facets {
		params.Add("facet.field", facet)
	}

	u := si.Host + "?" + params.Encode()
	req, err := http.NewRequest("GET", u, nil)
	if err != nil {
		return result, err
	}

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
		log.Println("GET", u, "returned", resp.Status)
	}
	// read the response...?
	decoder := json.NewDecoder(resp.Body)
	err = decoder.Decode(&result)

	return result, err
}
