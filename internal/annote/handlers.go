package annote

import (
	"fmt"
	"html/template"
	"io"
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
	return c.FirstField(targets...)
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

func add0(values ...int) int {
	result := 0
	for _, v := range values {
		result += v
	}
	return result
}

func sub0(limit int, values ...int) int {
	var result int
	if len(values) > 0 {
		result = values[0]
		for _, v := range values[1:] {
			result -= v
		}
	}
	if result < limit {
		result = limit
	}
	return result
}

func join(ss []string, c string) string {
	return strings.Join(ss, c)
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
		"add":               add0,
		"sub":               sub0,
		"Join":              join,
	})
	t, err := t.ParseGlob(filepath.Join(path, "*"))
	Templates = t
	return err
}

func DoTemplate(w io.Writer, name string, data interface{}) {
	err := Templates.ExecuteTemplate(w, name, data)
	if err != nil {
		log.Println(err)
	}
}

func notFound(w http.ResponseWriter) {
	w.WriteHeader(http.StatusNotFound)
	DoTemplate(w, "404", nil)
}

func serverError(w http.ResponseWriter) {
	w.WriteHeader(http.StatusInternalServerError)
	DoTemplate(w, "500", nil)
}

func NotImplemented(w http.ResponseWriter, r *http.Request, ps httprouter.Params) {
	w.WriteHeader(http.StatusNotImplemented)
	DoTemplate(w, "500", nil)
}

// IndexHandler responds to the root route.
func IndexHandler(w http.ResponseWriter, r *http.Request, ps httprouter.Params) {
	DoTemplate(w, "homepage", nil)
}

func GetObject(w http.ResponseWriter, r *http.Request, ps httprouter.Params) {
	pid := ps.ByName("id")
	item, err := Datasource.FindItem(pid)
	if err != nil {
		w.WriteHeader(500)
		fmt.Fprintln(w, err)
		return
	}
	DoTemplate(w, "item", item)
}

func ConfigPage(w http.ResponseWriter, r *http.Request, ps httprouter.Params) {
	DoTemplate(w, "config", nil)
}

func UpdateConfig(w http.ResponseWriter, r *http.Request, ps httprouter.Params) {
	if harvestStatus == StatusWaiting {
		harvestControl <- HNow
	}

	ConfigPage(w, r, ps)
}

type showTemplate struct {
	Messages       []string
	Item           CurateItem
	User           *User
	Title          string
	AnnotationInfo ItemAnnotationInfo
}

func ObjectShow(w http.ResponseWriter, r *http.Request, ps httprouter.Params) {
	username, _, _ := r.BasicAuth()
	user := FindUser(username)

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
	DoTemplate(w, "show", showTemplate{
		Item:           item,
		User:           user,
		Title:          item.FirstField("dc:title", "filename"),
		AnnotationInfo: GetAnnotationInfoForItem(pid, username),
	})
}

func ObjectIndex(w http.ResponseWriter, r *http.Request, ps httprouter.Params) {
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
	IndexRecord(item)
	http.Redirect(w, r, "/items/"+pid, 302)
}

func IndexEverything(w http.ResponseWriter, r *http.Request, ps httprouter.Params) {
	IndexBatch(&AllItems{})
}

type objectnew struct {
	Title string
	User  *User
}

func ObjectNew(w http.ResponseWriter, r *http.Request, ps httprouter.Params) {
	username, _, _ := r.BasicAuth()
	user := FindUser(username)

	DoTemplate(w, "record-new", objectnew{Title: "Deposit New Item", User: user})
}

func ObjectNewPost(w http.ResponseWriter, r *http.Request, ps httprouter.Params) {
	username, _, _ := r.BasicAuth()

	err := r.ParseMultipartForm(100 * (1 << 20)) // 100 MiB
	if err != nil {
		// ???
	}

	now := time.Now()
	today := now.Format("2006-01-02")
	todayminute := now.Format(time.RFC3339)

	workid := NewIdentifier()
	newitem := CurateItem{PID: "und:" + workid}
	for key, value := range r.MultipartForm.Value {
		for _, v := range value {
			newitem.Add(key, v)
		}
	}
	newitem.Add("fedora-create", todayminute)
	newitem.Add("fedora-modify", todayminute)

	// now save all the file attachments
	isfirstfile := true
	for _, attached := range r.MultipartForm.File["files"] {
		// make a new item for each...
		fileid := NewIdentifier()

		// make first item the representative of the work
		if isfirstfile {
			newitem.Add("representative", "und:"+fileid)
			isfirstfile = false
		}

		contents, err := attached.Open()
		if err != nil {
			log.Println(err)
		}

		writer, err := FileStore.Create(fileid)
		// calculate md5 sum as we copy it?
		_, err = io.Copy(writer, contents)
		if err != nil {
			// ???
		}

		// not using defer since we are in a loop
		writer.Close()
		contents.Close()

		fileitem := CurateItem{PID: "und:" + fileid}
		fileitem.Add("af-model", "GenericFile")
		fileitem.Add("isPartOf", "und:"+workid)
		fileitem.Add("depositor", username)
		fileitem.Add("owner", username)
		fileitem.Add("read-group", "public")
		fileitem.Add("edit-person", username)
		fileitem.Add("dc:dateSubmitted", today)
		fileitem.Add("dc:title", attached.Filename)
		fileitem.Add("filename", attached.Filename)
		//fileitem.Add("checksum-md5")
		fileitem.Add("mime-type", attached.Header.Get("Content-Type"))
		fileitem.Add("file-location", fileid)
		fileitem.Add("thumbnail", fileid)
		fileitem.Add("fedora-create", todayminute)
		fileitem.Add("fedora-modify", todayminute)
		Datasource.IndexItem(fileitem)
		IndexRecord(fileitem)

		// make thumbnail in the background
		go func() {
			switch fileitem.FirstField("mime-type") {
			case "application/pdf":
				FileStore.MakeThumbnailPDF(fileid)
			default:
				FileStore.MakeThumbnailImage(fileid)
			}
		}()
	}

	// save/index the item record last in case we added any
	// more fields while copying files, e.g. the representative
	Datasource.IndexItem(newitem)
	IndexRecord(newitem)

	target := "/show/und:" + workid
	http.Redirect(w, r, target, 302)
}

func ObjectEdit(w http.ResponseWriter, r *http.Request, ps httprouter.Params) {
}

type annotateTemplate struct {
	Messages []string
	Item     CurateItem
	Title    string
	AnnoURL  string
}

func ObjectAnnotate(w http.ResponseWriter, r *http.Request, ps httprouter.Params) {
	var output annotateTemplate
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

	username, _, _ := r.BasicAuth()
	user := FindUser(username)

	var ids []ItemUUID

	if which := r.FormValue("copy"); which != "" {
		// view someone else's uploaded copy
		ids, err = Datasource.SearchItemUUID(pid, which, "")
	} else {
		// is there already a UUID for this (item, user) pair?
		ids, err = Datasource.SearchItemUUID(pid, username, "")
	}
	if err != nil {
		w.WriteHeader(500)
		fmt.Fprintln(w, err)
		return
	}
	switch {
	case len(ids) == 0:
		// upload the item
		msg, err := AnnotationStore.UploadItem(item, user)
		if msg != "" {
			output.Messages = append(output.Messages, msg)
		}
		if err != nil {
			output.Messages = append(output.Messages, err.Error())
		}
		if len(output.Messages) > 0 {
			break
		}

		// get the uuid to provide a courtesy link
		ids, err = Datasource.SearchItemUUID(pid, username, "")
		if err != nil {
			w.WriteHeader(500)
			fmt.Fprintln(w, err)
			return
		}
		if len(ids) == 0 {
			output.Messages = append(output.Messages, "There was a problem submitting the requst to the annotation service")
			break
		}
		fallthrough
	case len(ids) > 0 && ids[0].Status != "a":
		// item is uploaded and is processing
		output.Messages = append(output.Messages, "This item is being transferred to the annotation server.")
		output.AnnoURL = AnnotationStore.ViewerURL(ids[0].UUID)
		AnnoChan <- struct{}{}
	case len(ids) > 0:
		// item is uploaded and can be viewed
		target := AnnotationStore.ViewerURL(ids[0].UUID)
		http.Redirect(w, r, target, 302)
	}

	output.Item = item
	output.Title = "Annotation for " + item.FirstField("dc:title", "filename")
	DoTemplate(w, "annotate-item", output)
}

var (
	AnnoChan chan struct{}
)

func StartBackgroundProcess() {
	AnnoChan = make(chan struct{})
	go backgroundRAPchecker(AnnoChan)
}

type searchresults struct {
	Title      string
	User       *User
	Query      string
	Page       int
	NumPerPage int
	StartIndex int
	ItemList   []CurateItem
}

func SearchPage(w http.ResponseWriter, r *http.Request, ps httprouter.Params) {
	query := r.FormValue("q")
	page0 := r.FormValue("p")
	numperpage0 := r.FormValue("n")

	page := parseIntDefault(page0, 0)
	numperpage := parseIntDefault(numperpage0, 10)

	username, _, _ := r.BasicAuth()
	user := FindUser(username)

	offset := numperpage * page

	results, err := Solr.SendQuery(SolrQuery{
		Query: query,
		Start: offset,
		Rows:  numperpage,
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
	log.Printf("solr %q", results)
	if err != nil {
		w.WriteHeader(500)
		fmt.Fprintln(w, err)
		return
	}
	items := solrToCurateItem(results.Response.Docs)

	output := searchresults{
		Title:      "Search",
		User:       user,
		Query:      query,
		Page:       page,
		NumPerPage: numperpage,
		StartIndex: offset + 1,
		ItemList:   items,
	}

	DoTemplate(w, "search", output)
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

func parseIntDefault(s string, v int) int {
	n, err := strconv.ParseInt(s, 10, 32)
	if err != nil || n < 0 {
		return v
	}
	return int(n)
}

type annotatePage struct {
	Title    string
	User     *User
	Query    string
	Messages []string
	Records  []ItemUUID
}

func ShowAnnotateStatus(w http.ResponseWriter, r *http.Request, ps httprouter.Params) {
	var output annotatePage
	username, _, _ := r.BasicAuth()
	user := FindUser(username)

	userq := r.FormValue("user")
	switch userq {
	case "all":
		userq = ""
	case "":
		userq = username
	default:
	}
	itemq := r.FormValue("item")
	statusq := r.FormValue("status")

	ids, err := Datasource.SearchItemUUID(itemq, userq, statusq)
	if err != nil {
		output.Messages = append(output.Messages, err.Error())
	}
	output.Title = "Annotation Upload Status Page"
	output.User = user
	output.Records = ids
	output.Query = userq

	DoTemplate(w, "anno-status", output)
}

func ObjectDownload(w http.ResponseWriter, r *http.Request, ps httprouter.Params) {
	pid := ps.ByName("id")
	pid = strings.TrimPrefix(pid, "und:")

	download(w, r, pid, false)
}

func ObjectDownloadThumbnail(w http.ResponseWriter, r *http.Request, ps httprouter.Params) {
	pid := ps.ByName("id")
	pid = strings.TrimPrefix(pid, "und:")

	download(w, r, pid, true)
}

// download will look for content files and thumbnail files in our local storage, our local
// cache of curate content, and finally try to copy the files from a backing curate server.
// When it finds content, it will return it via the ResponseWriter.
func download(w http.ResponseWriter, r *http.Request, pid string, isthumbnail bool) {
	basename := pid
	if isthumbnail {
		basename = pid + "-thumbnail"
	}

	// don't need to do any auth here since we get files
	// from curate's public interface. So we ultimately
	// only have access to the public things.

	// is file uploaded locally?
	path := FileStore.Find(basename)
	if path == "" {
		// is file cached?
		path = FindFileInCache(basename)
	}
	if path == "" {
		// not cached and not uploaded, see if on remote curate
		var remotepath string
		if isthumbnail {
			remotepath = fmt.Sprintf("%s/downloads/%s/thumbnail", CurateURL, pid)
		} else {
			remotepath = fmt.Sprintf("%s/downloads/%s", CurateURL, pid)
		}
		err := DownloadFileToCache(basename, remotepath)
		if err != nil {
			w.WriteHeader(500)
			fmt.Fprintln(w, err)
			return
		}
		path = FindFileInCache(basename)
	}
	http.ServeFile(w, r, path)
}

type resetparams struct {
	ResetToken string
	User       *User
	Message    string
}

func ResetShow(w http.ResponseWriter, r *http.Request, ps httprouter.Params) {
	// are there valid reset tokens?
	var user *User
	reset := r.FormValue("r")
	if reset != "" {
		user = FindUserByToken(reset)
	}

	v := resetparams{
		ResetToken: reset,
		User:       user,
	}

	DoTemplate(w, "reset", v)
}

func ResetUpdate(w http.ResponseWriter, r *http.Request, ps httprouter.Params) {
	var user *User
	reset := r.FormValue("r")
	if reset != "" {
		user = FindUserByToken(reset)
	}
	v := resetparams{
		ResetToken: reset,
		User:       user,
	}
	if user == nil {
		v.Message = "The password was not updated."
		DoTemplate(w, "reset", v)
		return
	}
	pw1 := r.FormValue("pwd")
	pw2 := r.FormValue("pwd2")
	if pw1 != pw2 {
		v.Message = "The password was not entered the same both times."
		DoTemplate(w, "reset", v)
		return
	}

	err := ResetPassword(user.Username, pw1)
	if err != nil {
		v.Message = err.Error()
		DoTemplate(w, "reset", v)
		return
	}

	http.Redirect(w, r, "/", 302)
}
