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

	items, err := Datasource.FindAllRange(offset, numperpage)
	if err != nil {
		w.WriteHeader(500)
		fmt.Fprintln(w, err)
		return
	}

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

	// don't need to do any auth here since we get files
	// from curate's public interface. So we ultimately
	// only have access to the public things.

	// is file cached?
	path := FindFileInCache(pid)
	if path == "" {
		err := DownloadFileToCache(pid, fmt.Sprintf("%s/downloads/%s", CurateURL, pid))
		if err != nil {
			w.WriteHeader(500)
			fmt.Fprintln(w, err)
			return
		}
		path = FindFileInCache(pid)
	}
	http.ServeFile(w, r, path)
}

func ObjectDownloadThumbnail(w http.ResponseWriter, r *http.Request, ps httprouter.Params) {
	pid := ps.ByName("id")
	pid = strings.TrimPrefix(pid, "und:")

	// is file cached?
	basename := pid + "-thumbnail"
	path := FindFileInCache(basename)
	if path == "" {
		err := DownloadFileToCache(basename, fmt.Sprintf("%s/downloads/%s/thumbnail", CurateURL, pid))
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

type userparams struct {
	User    *User
	Message string
	NewUser *User
}

func ProfileShow(w http.ResponseWriter, r *http.Request, ps httprouter.Params) {
	username, _, _ := r.BasicAuth()
	user := FindUser(username)

	DoTemplate(w, "profile", userparams{User: user})
}

func ProfileUpdate(w http.ResponseWriter, r *http.Request, ps httprouter.Params) {
	username, _, _ := r.BasicAuth()
	user := FindUser(username)

	v := userparams{User: user}

	// figure out what is desired...
	cp := r.FormValue("changepass")
	if cp != "" {
		t, err := CreateResetToken(username)
		if err != nil {
			v.Message = err.Error()
			DoTemplate(w, "profile", v)
			return
		}
		http.Redirect(w, r, "/reset?r="+t, 302)
		return
	}

	nu := r.FormValue("newuser")
	if nu != "" {
		newuser, err := CreateNewUser()
		if err != nil {
			v.Message = err.Error()
		} else {
			v.NewUser = newuser
		}
		DoTemplate(w, "profile", v)
		return
	}

	// no idea. display page again
	DoTemplate(w, "profile", v)
}

func ProfileEditShow(w http.ResponseWriter, r *http.Request, ps httprouter.Params) {
	username, _, _ := r.BasicAuth()
	user := FindUser(username)

	DoTemplate(w, "profile_edit", userparams{User: user})
}

var (
	orcidRE = regexp.MustCompile(`\d{4}-\d{4}-\d{4}-\d{3}[0-9X]`)
)

func ProfileEditUpdate(w http.ResponseWriter, r *http.Request, ps httprouter.Params) {
	var err error
	username, _, _ := r.BasicAuth()
	user := FindUser(username)
	v := userparams{User: user}
	var messages []string

	newusername := r.FormValue("username")
	if newusername != username {
		otheruser := FindUser(newusername)
		if otheruser != nil {
			messages = append(messages, "Username already exists")
			goto do_orcid
		}
		user.Username = newusername
	}

do_orcid:
	orcid := r.FormValue("orcid")
	if orcid != "" {
		neworcid := orcidRE.FindString(orcid)
		if neworcid == "" {
			messages = append(messages, "no orcid in "+orcid)
			goto out
		}
		user.ORCID = neworcid
	} else {
		user.ORCID = ""
	}

	err = SaveUser(user)
	if err != nil {
		messages = append(messages, err.Error())
		log.Println(err)
	}
	if newusername != username {
		ClearUserFromCache(username)
	}

out:
	if len(messages) > 0 {
		v.Message = strings.Join(messages, " // ")
		DoTemplate(w, "profile_edit", v)
		return
	}
	http.Redirect(w, r, "/profile", 302)
}
