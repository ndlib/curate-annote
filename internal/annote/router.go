package annote

import (
	"fmt"
	"log"
	"net/http"

	"github.com/julienschmidt/httprouter"
)

func AddRoutes() http.Handler {
	var routes = []struct {
		method      string
		route       string
		handler     httprouter.Handle
		defaultAuth bool
	}{
		{"GET", "/", IndexHandler, false},
		{"GET", "/obj/:id", GetObject, true},  // legacy
		{"GET", "/obj", NotImplemented, true}, // legacy
		{"GET", "/show/:id", ObjectShow, true},
		{"GET", "/show/:id/annotate", ObjectAnnotate, true},
		{"POST", "/show/:id/index", ObjectIndex, true},
		{"GET", "/new", ObjectNew, true},
		{"POST", "/new", ObjectNewPost, true},
		{"GET", "/show/:id/edit", ObjectEdit, true},
		{"GET", "/downloads/:id", ObjectDownload, false},
		{"GET", "/downloads/:id/thumbnail", ObjectDownloadThumbnail, true},
		{"GET", "/search", SearchPage, true},
		{"GET", "/anno", ShowAnnotateStatus, true},
		{"POST", "/index", IndexEverything, true},
		{"GET", "/reset", ResetShow, false},
		{"POST", "/reset", ResetUpdate, false},
		{"GET", "/new-user", ProfileNewShow, false},
		{"POST", "/new-user", ProfileNewPost, false},
		{"GET", "/profile", ProfileShow, true},
		{"POST", "/profile", ProfileUpdate, true},
		{"GET", "/profile/edit", ProfileEditShow, true},
		{"POST", "/profile/edit", ProfileEditUpdate, true},
		{"GET", "/about", AboutShow, false},
		{"GET", "/config", ConfigPage, true},
		{"POST", "/config", UpdateConfig, true},
		// Annotot endpoints
		{"GET", "/annotot/pages", AnnototPages, false},
		{"GET", "/annotot/lists", NotImplemented, false},
		{"GET", "/annotot", NotImplemented, false},
		{"POST", "/annotot", AnnototCreate, true},
		{"PATCH", "/annotot/:uuid", AnnototUpdate, true},
	}

	r := httprouter.New()
	for _, route := range routes {
		h := route.handler
		if route.defaultAuth {
			h = authWrapper(h)
		}
		r.Handle(route.method,
			route.route,
			h)
	}
	if StaticFilePath != "" {
		r.ServeFiles("/static/*filepath", http.Dir(StaticFilePath))
	}
	return logWrapper(r)
}

// logWrapper takes a handler and returns a handler which does the same thing,
// after first logging the request URL.
func logWrapper(handler http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		log.Println(r.Method, r.URL)
		handler.ServeHTTP(w, r)
	})
}

func ShowForbidden(w http.ResponseWriter) {
	// tell web browsers to display password box
	w.Header().Set("WWW-Authenticate", "Basic")
	w.WriteHeader(401)
	fmt.Fprintln(w, "Forbidden")
}

// VerifyAuth looks at the basic auth username and password. If it is not
// valid, it returns a response asking for better ones and returns false. If it
// is valid, it returns true.
func VerifyAuth(w http.ResponseWriter, r *http.Request, ps httprouter.Params) bool {
	username, password, ok := r.BasicAuth()
	if !ok {
		ShowForbidden(w)
		return false
	}
	err := CheckPassword(username, password)
	if err != nil {
		ShowForbidden(w)
		return false
	}
	return true
}

func authWrapper(h httprouter.Handle) httprouter.Handle {
	return func(w http.ResponseWriter, r *http.Request, ps httprouter.Params) {
		if !VerifyAuth(w, r, ps) {
			return
		}

		h(w, r, ps)
	}
}
