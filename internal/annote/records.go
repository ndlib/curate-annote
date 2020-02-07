package annote

import (
	"encoding/xml"
	"fmt"
	"io"
	"io/ioutil"
	"log"
	"strings"
	"time"

	"github.com/knakk/rdf"
)

// This file contains functions needed to harvest and decode fedora
// objects. They are then stored in the database.

type Pair struct {
	Predicate string
	Object    string
}

// A CurateItem holds the metadata record for a single collection, item, or
// file. It consists of our local identifier for the item (with the namespace
// prefix, if any), and then a sequence of field-value pairs. The field names
// can repeat. The ordering of the pairs is only important among those having
// the same field name.
type CurateItem struct {
	PID        string
	Properties []Pair
}

func (c *CurateItem) Add(predicate string, value string) {
	if value == "" {
		return
	}
	c.Properties = append(c.Properties, Pair{
		Predicate: predicate,
		Object:    value,
	})
}

func (c *CurateItem) FirstField(targets ...string) string {
	for _, target := range targets {
		for i := range c.Properties {
			if c.Properties[i].Predicate == target {
				return c.Properties[i].Object
			}
		}
	}
	return ""
}

func (c *CurateItem) RemoveAll(target string) {
	for i := 0; i < len(c.Properties); {
		if c.Properties[i].Predicate == target {
			c.Properties = append(c.Properties[:i], c.Properties[i+1:]...)
		} else {
			i++
		}
	}
}

// A CurateItemAlt is an alternative representation of a CurateItem. It
// consolidates each property field name into a map entry, and arranges the
// (possibly multiple) values as a list. The PID is stored under the map entry
// "PID".
type CurateItemAlt map[string][]string

func (c *CurateItem) AsAlternate() CurateItemAlt {
	result := make(map[string][]string)
	result["PID"] = []string{c.PID}
	for _, p := range c.Properties {
		result[p.Predicate] = append(result[p.Predicate], p.Object)
	}
	return CurateItemAlt(result)
}

type PredicatePair struct {
	XMLName xml.Name
	V       string `xml:",any,attr"`
}
type RelsExtDS struct {
	Description struct {
		P []PredicatePair `xml:",any"`
	} `xml:"Description"`
}

func ReadRelsExt(remote *RemoteFedora, id string, result *CurateItem) error {
	body, err := remote.GetDatastream(id, "RELS-EXT")
	if err != nil {
		return err
	}
	defer body.Close()
	// using the rdf decoder with rdf.RDFXML caused problems since the decoder
	// thought the `info:` scheme used by fedora was an undeclared namespace.
	// so we decode it ourself. The XML/RDF used by fedora in RELS-EXT is
	// very limited and structured, so this is not expected to be a problem.
	// (e.g. every tuple must have the given resource as a subject).
	var v RelsExtDS
	dec := xml.NewDecoder(body)
	err = dec.Decode(&v)
	body.Close()

	for _, p := range v.Description.P {
		// this isn't taking namespace into account...
		p.V = ApplyPrefixes(p.V)
		switch p.XMLName.Local {
		case "hasModel":
			result.Add("af-model", p.V)
		case "isMemberOfCollection":
			result.Add("isMemberOfCollection", p.V)
		case "isPartOf":
			result.Add("isPartOf", p.V)

		// make sure the permission labels match those used in rightsMetadata
		case "hasEditor":
			result.Add("edit-person", p.V)
		case "hasEditorGroup":
			result.Add("edit-group", p.V)

		default:
			result.Add(ApplyPrefixes(p.XMLName.Space+p.XMLName.Local), p.V)
		}
	}

	return nil
}

var Prefixes = map[string]string{
	"info:fedora/und:":                                       "und:",
	"info:fedora/afmodel:":                                   "",
	"http://purl.org/dc/terms/":                              "dc:",
	"https://library.nd.edu/ns/terms/":                       "nd:",
	"http://purl.org/ontology/bibo/":                         "bibo:",
	"http://www.ndltd.org/standards/metadata/etdms/1.1/":     "ms:",
	"http://purl.org/vra/":                                   "vracore:",
	"http://id.loc.gov/vocabulary/relators/":                 "mrel:",
	"http://www.ebu.ch/metadata/ontologies/ebucore/ebucore#": "ebucore:",
	"http://xmlns.com/foaf/0.1/":                             "foaf:",
	"http://projecthydra.org/ns/relations#":                  "hydra:",
	"http://www.w3.org/2000/01/rdf-schema#":                  "rdfs:",
	"http://purl.org/pav/":                                   "pav:",
}

func ApplyPrefixes(s string) string {
	for k, v := range Prefixes {
		if strings.HasPrefix(s, k) {
			return v + strings.TrimPrefix(s, k)
		}
	}
	return s
}

func ReadDescMetadata(remote *RemoteFedora, id string, result *CurateItem) error {
	body, err := remote.GetDatastream(id, "descMetadata")
	if err != nil {
		return err
	}
	// remember the values we see for blank nodes, to add at the end
	blanks := make(map[string][]string)
	defer body.Close()
	triples := rdf.NewTripleDecoder(body, rdf.NTriples)
	for {
		v, err := triples.Decode()
		if err == io.EOF {
			break
		}
		if err != nil {
			return err
		}

		subject := ApplyPrefixes(v.Subj.String())
		predicate := ApplyPrefixes(v.Pred.String())
		object := ApplyPrefixes(v.Obj.String())
		if v.Subj.Type() == rdf.TermBlank {
			blanks[subject] = append(blanks[subject], predicate, object)
		} else if v.Obj.Type() == rdf.TermBlank {
			blanks[object] = append(blanks[object], "@", predicate)
		} else {
			result.Add(predicate, object)
		}
	}

	// now add all the fields that used blank nodes
	for _, valueList := range blanks {
		var parentPredicate string
		var encodedValue string
		for i := 0; i < len(valueList); i += 2 {
			predicate := valueList[i]
			object := valueList[i+1]
			if predicate == "@" {
				parentPredicate = object
				continue
			}
			// very simple k-v pair encoding, and not very safe.
			// we assume "^^" is unlikely to appear in the data.
			encodedValue += "^^" + predicate + " " + object
		}
		result.Add(parentPredicate, encodedValue)
	}

	return nil
}

type Access struct {
	Type    string   `xml:"type,attr"`
	Persons []string `xml:"machine>person"`
	Groups  []string `xml:"machine>group"`
}
type rightsDS struct {
	Access  []Access `xml:"access"`
	Embargo string   `xml:"embargo>machine"`
}

func ReadRightsMetadata(remote *RemoteFedora, id string, result *CurateItem) error {
	body, err := remote.GetDatastream(id, "rightsMetadata")
	if err != nil {
		return err
	}
	var v rightsDS
	dec := xml.NewDecoder(body)
	err = dec.Decode(&v)
	body.Close()
	if err != nil {
		return err
	}

	result.Add("embargo-date", v.Embargo)
	for _, access := range v.Access {
		var grouplabel string
		var personlabel string
		switch access.Type {
		default:
			// only care about read and edit permission levels
			continue
		case "read":
			grouplabel = "read-group"
			personlabel = "read-person"
		case "edit":
			grouplabel = "edit-group"
			personlabel = "edit-person"
		}
		for _, g := range access.Groups {
			result.Add(grouplabel, g)
		}
		for _, p := range access.Persons {
			result.Add(personlabel, p)
		}
	}

	return nil
}

type propertiesDS struct {
	Depositor      string `xml:"depositor"`
	Owner          string `xml:"owner"`
	Representative string `xml:"representative"`
}

func ReadProperties(remote *RemoteFedora, id string, result *CurateItem) error {
	body, err := remote.GetDatastream(id, "properties")
	if err != nil {
		return err
	}
	var props propertiesDS
	dec := xml.NewDecoder(body)
	err = dec.Decode(&props)
	body.Close()
	if err != nil {
		return err
	}

	result.Add("depositor", props.Depositor)
	result.Add("owner", props.Owner)
	result.Add("representative", props.Representative)

	return nil
}

func ReadContent(remote *RemoteFedora, id string, result *CurateItem) error {
	info, err := remote.GetDatastreamInfo(id, "content")
	if err != nil {
		return err
	}

	result.Add("filename", info.Label)
	result.Add("checksum-md5", info.Checksum)
	result.Add("mime-type", info.MIMEType)
	//result.Add("file-size", info.Size) // convert to string
	result.Add("file-location", info.Location)

	return nil
}

func ReadCharacterization(remote *RemoteFedora, id string, result *CurateItem) error {
	body, err := remote.GetDatastream(id, "characterization")
	if err != nil {
		return err
	}
	v, err := ioutil.ReadAll(body)
	body.Close()
	if err != nil {
		return err
	}
	result.Add("characterization", string(v))

	return nil
}

func ReadThumbnail(remote *RemoteFedora, id string, result *CurateItem) error {
	info, err := remote.GetDatastreamInfo(id, "thumbnail")
	if err != nil {
		return err
	}

	result.Add("thumbnail", info.Location)

	return nil
}

func ReadBendoItem(remote *RemoteFedora, id string, result *CurateItem) error {
	body, err := remote.GetDatastream(id, "bendo-item")
	if err != nil {
		return err
	}
	v, err := ioutil.ReadAll(body)
	body.Close()
	if err != nil {
		return err
	}

	result.Add("bendo-item", string(v))

	return nil
}

// FetchOneCurateObject loads the given fedora object and interpretes it as if
// it were a curate object. This means only certain datastreams are downloaded.
func FetchOneCurateObject(remote *RemoteFedora, id string) (CurateItem, error) {
	var err error
	var rememberErr error
	result := &CurateItem{PID: id}
	// Assume the id is a curate object, which means we know exactly which
	// datastreams to look at
	err = ReadRelsExt(remote, id, result)
	if err != nil {
		rememberErr = err
	}
	err = ReadProperties(remote, id, result)
	if err != nil && err != ErrNotFound {
		// people objects don't have properties ds
		rememberErr = err
	}
	err = ReadRightsMetadata(remote, id, result)
	if err != nil && err != ErrNotFound {
		// people objects don't have rightsMetadata ds
		rememberErr = err
	}
	err = ReadDescMetadata(remote, id, result)
	if err != nil {
		rememberErr = err
	}
	// now try datastreams that may not be present
	err = ReadContent(remote, id, result)
	if err != nil && err != ErrNotFound {
		rememberErr = err
	}
	err = ReadThumbnail(remote, id, result)
	if err != nil && err != ErrNotFound {
		rememberErr = err
	}
	err = ReadCharacterization(remote, id, result)
	if err != nil && err != ErrNotFound {
		rememberErr = err
	}
	err = ReadBendoItem(remote, id, result)
	if err != nil && err != ErrNotFound {
		rememberErr = err
	}

	// finally, get the fedora create and modify dates
	info, err := remote.GetObjectInfo(id)
	if err != nil {
		rememberErr = err
	} else {
		result.Add("fedora-create", info.CreateDate.Format(time.RFC3339))
		result.Add("fedora-modify", info.LastModDate.Format(time.RFC3339))
	}

	return *result, rememberErr
}

func PrintItem(item CurateItem) error {
	for _, t := range item.Properties {
		fmt.Println(
			item.PID, "\t",
			t.Predicate, "\t",
			strings.ReplaceAll(t.Object, "\n", "\\n"))
	}
	return nil
}

func HarvestCurateObjects(remote *RemoteFedora, since time.Time, f func(CurateItem) error) error {
	var nItems, nErr int
	token := ""
	query := "pid~und:*"
	if !since.IsZero() {
		// space before mDate is important
		query += " mDate>" + since.Format("2006-01-02T03:04:05")
	}
	log.Println(query)

	var err error
	for {
		// get a page of search results
		var ids []string
		ids, token, err = remote.SearchObjects(query, token)
		if err != nil {
			log.Println(err)
			break
		}
		for _, pid := range ids {
			nItems++
			item, err := FetchOneCurateObject(remote, pid)
			if err != nil {
				log.Println(pid, err)
				// don't exit early, we want to try harvesting each item in the list at least once
				nErr++
				continue
			}
			// update item in database
			err = f(item)
			if err != nil {
				log.Println(pid, err)
				// don't exit early, we want to try harvesting each item in the list at least once
				nErr++
				continue
			}
		}
		// no token is returned on the last results page
		if token == "" {
			break
		}
	}
	log.Println("Harvest:", nItems, "items with", nErr, "errors")
	return err
}
