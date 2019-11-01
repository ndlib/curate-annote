package annote

import (
	"fmt"
	"io"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
)

func TestGetObjectInfo(t *testing.T) {
	s := NewFedora()
	defer s.Close()

	rf := NewRemote(s.URL)
	obj, _ := rf.GetObjectInfo("und:123456789")
	fmt.Printf("%#v", obj)
}

func TestSearchObjects(t *testing.T) {
	s := NewFedora()
	defer s.Close()

	rf := NewRemote(s.URL)
	ids, token, err := rf.SearchObjects("pid~und:*", "")
	if err != nil {
		t.Fatal("Received", err)
	}
	if token != "db4926dfeeb9f84bffad7b749ba02fb2" {
		t.Error("Expected token db4926dfeeb9f84bffad7b749ba02fb2, received", token)
	}
	if len(ids) != 2 {
		t.Error("Expected 2 pids, received", ids)
	}
}

//
// simple server to test reading data from fedora
//

type kvserver struct {
	paths map[string]string
}

func (kv *kvserver) ServeHTTP(w http.ResponseWriter, r *http.Request) {
	path := r.URL.Path
	fmt.Println(path)
	v := kv.paths[path]
	if v == "" {
		w.WriteHeader(404)
		return
	}
	f := strings.NewReader(v)
	io.Copy(w, f)
}

var data = kvserver{
	paths: map[string]string{
		"/objects/und:123456789": `<?xml version="1.0" encoding="UTF-8"?><objectProfile xmlns="http://www.fedora.info/definitions/1/0/access/" xmlns:xsd="http://www.w3.org/2001/XMLSchema" xmlns:xsi="http://www.w3.org/2001/XMLSchema-instance" xsi:schemaLocation="http://www.fedora.info/definitions/1/0/access/ http://www.fedora.info/definitions/1/0/objectProfile.xsd" pid="und:123456789"><objLabel/><objOwnerId>fedoraAdmin</objOwnerId><objModels><model>info:fedora/fedora-system:FedoraObject-3.0</model></objModels><objCreateDate>2018-02-12T20:11:39.790Z</objCreateDate><objLastModDate>2018-02-12T20:19:01.655Z</objLastModDate><objDissIndexViewURL>http://localhost:8983/fedora/objects/und%3A123456789/methods/fedora-system%3A3/viewMethodIndex</objDissIndexViewURL><objItemIndexViewURL>http://localhost:8983/fedora/objects/und%3A123456789/methods/fedora-system%3A3/viewItemIndex</objItemIndexViewURL><objState>A</objState></objectProfile>`,
		//
		"/objects/und:123456789/datastreams": `<?xml version="1.0" encoding="UTF-8"?><objectDatastreams xmlns="http://www.fedora.info/definitions/1/0/access/" xmlns:xsd="http://www.w3.org/2001/XMLSchema" xmlns:xsi="http://www.w3.org/2001/XMLSchema-instance" xsi:schemaLocation="http://www.fedora.info/definitions/1/0/access/ http://www.fedora-commons.org/definitions/1/0/listDatastreams.xsd" pid="und:123456789" baseURL="http://localhost:8983/fedora/"><datastream dsid="DC" label="Dublin Core Record for this object" mimeType="text/xml"/><datastream dsid="content" label="" mimeType=""/></objectDatastreams>`,
		//
		"/objects/und:123456789/datastreams/DC": `<?xml version="1.0" encoding="UTF-8"?><datastreamProfile  xmlns="http://www.fedora.info/definitions/1/0/management/"  xmlns:xsd="http://www.w3.org/2001/XMLSchema" xmlns:xsi="http://www.w3.org/2001/XMLSchema-instance" xsi:schemaLocation="http://www.fedora.info/definitions/1/0/management/ http://www.fedora.info/definitions/1/0/datastreamProfile.xsd" pid="und:123456789" dsID="DC" ><dsLabel>Dublin Core Record for this object</dsLabel><dsVersionID>DC1.0</dsVersionID><dsCreateDate>2018-02-12T20:11:39.790Z</dsCreateDate><dsState>A</dsState><dsMIME>text/xml</dsMIME><dsFormatURI>http://www.openarchives.org/OAI/2.0/oai_dc/</dsFormatURI><dsControlGroup>X</dsControlGroup><dsSize>342</dsSize><dsVersionable>true</dsVersionable><dsInfoType></dsInfoType><dsLocation>und:123456789+DC+DC1.0</dsLocation><dsLocationType></dsLocationType><dsChecksumType>DISABLED</dsChecksumType><dsChecksum>none</dsChecksum></datastreamProfile>`,
		//
		"/objects/und:123456789/datastreams/content": `<?xml version="1.0" encoding="UTF-8"?><datastreamProfile  xmlns="http://www.fedora.info/definitions/1/0/management/"  xmlns:xsd="http://www.w3.org/2001/XMLSchema" xmlns:xsi="http://www.w3.org/2001/XMLSchema-instance" xsi:schemaLocation="http://www.fedora.info/definitions/1/0/management/ http://www.fedora.info/definitions/1/0/datastreamProfile.xsd" pid="und:123456789" dsID="content" ><dsLabel></dsLabel><dsVersionID>content.0</dsVersionID><dsCreateDate>2018-02-12T20:19:01.655Z</dsCreateDate><dsState>A</dsState><dsMIME></dsMIME><dsFormatURI></dsFormatURI><dsControlGroup>R</dsControlGroup><dsSize>0</dsSize><dsVersionable>true</dsVersionable><dsInfoType></dsInfoType><dsLocation>http://localhost:14000/item/another-gb/vendor/github.com/BurntSushi/migration/Makefile</dsLocation><dsLocationType>URL</dsLocationType><dsChecksumType>DISABLED</dsChecksumType><dsChecksum>none</dsChecksum></datastreamProfile>`,
		//
		"/objects": `<?xml version="1.0" encoding="UTF-8"?><result xmlns="http://www.fedora.info/definitions/1/0/types/" xmlns:types="http://www.fedora.info/definitions/1/0/types/" xmlns:xsi="http://www.w3.org/2001/XMLSchema-instance" xsi:schemaLocation="http://www.fedora.info/definitions/1/0/types/ https://10.71.1.223:8443/fedora/schema/findObjects.xsd"><listSession><token>db4926dfeeb9f84bffad7b749ba02fb2</token><cursor>0</cursor><expirationDate>2019-09-16T13:25:09.312Z</expirationDate></listSession><resultList><objectFields><pid>und:00000001q45</pid></objectFields><objectFields><pid>und:00000001q5h</pid></objectFields></resultList></result>`,
	},
}

func NewFedora() *httptest.Server {
	return httptest.NewServer(&data)
}
