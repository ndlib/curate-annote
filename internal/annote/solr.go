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

func (si *SolrInfo) Search(q SearchQuery) (SearchResults, error) {
	var out SearchResults
	results, err := si.SendQuery(SolrQuery{
		Query: q.Query,
		Start: q.Start,
		Rows:  q.NumRows,
		QueryFields: []string{
			"desc_metadata__title_tesim",
			"desc_metadata__name_tesim",
			"noid_tsi",
			"file_format_tesim",
			"desc_metadata__contributor_tesim",
			"desc_metadata__abstract_tesim",
			"desc_metadata__description_tesim",
			"desc_metadata__creator_tesim",
			"desc_metadata__author_tesim",
			"admin_unit_tesim",
			"desc_metadata__publisher_tesim",
			"desc_metadata__language_tesim",
			"desc_metadata__collection_name_tesim",
			"desc_metadata__contributor_institution_tesim",
			"desc_metadata__subject_tesim",
			"desc_metadata__identifier_sim",
			"desc_metadata__urn_tesim",
			"degree_name_tesim",
			"degree_disciplines_tesim",
			"contributors_tesim",
			"degree_department_acronyms_tesim",
			"desc_metadata__date_created_tesim",
			"desc_metadata__source_tesim",
			"desc_metadata__alephIdentifier_tesim",
			"desc_metadata__patent_number_tesim",
		},
		FieldList: []string{
			"id",
			"desc_metadata__title_tesim",
			"active_fedora_model_ssi",
			"desc_metadata__creator_tesim",
			"desc_metadata__description_tesim",
			"desc_metadata__abstract_tesim",
			"desc_metadata__date_created_tesim",
			"admin_unit_tesim",
			"representative_tesim",
		},
		Sort: []string{
			"score desc",
			"desc_metadata__date_uploaded_dtsi desc",
		},
		FilterQuery: []string{
			"read_access_group_ssim:public", // only public things in prototype
			"-active_fedora_model_ssi:Person",
			"-active_fedora_model_ssi:FileAsset",
			"-active_fedora_model_ssi:GenericFile",
			"-active_fedora_model_ssi:Profile",
			"-active_fedora_model_ssi:ProfileSection",
			"-active_fedora_model_ssi:LinkedResource",
			"-active_fedora_model_ssi:Hydramata_Group",
		},
	})
	//log.Printf("solr %q", results)
	if err != nil {
		return out, err
	}
	out.Items = solrToCurateItem(results.Response.Docs)
	return out, nil
}

var (
	solrXlat = map[string]string{
		"desc_metadata__title_tesim":        "dc:title",
		"representative_tesim":              "representative",
		"active_fedora_model_ssi":           "af-model",
		"desc_metadata__creator_tesim":      "dc:creator",
		"desc_metadata__description_tesim":  "dc:description",
		"desc_metadata__abstract_tesim":     "dc:abstract",
		"desc_metadata__date_created_tesim": "dc:created",
		"admin_unit_tesim":                  "dc:creator#administrative_unit",
	}
)

func solrToCurateItem(docs []map[string]StringOrList) []CurateItem {
	var out []CurateItem
	for _, doc := range docs {
		item := CurateItem{}
		for k, v := range doc {
			vv := []string(v)
			if k == "id" {
				item.PID = vv[0]
				continue
			}
			if kk, ok := solrXlat[k]; ok {
				k = kk
			}
			for i := range vv {
				item.Properties = append(item.Properties, Pair{
					Predicate: k,
					Object:    vv[i],
				})
			}
		}
		out = append(out, item)
	}
	return out
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

// IndexRecord and IndexBatch for Solr is not supported.
func (si *SolrInfo) IndexRecord(item CurateItem) {}
func (si *SolrInfo) IndexBatch(source Batcher)   {}
