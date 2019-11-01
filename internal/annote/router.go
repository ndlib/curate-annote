package annote

import (
	"log"
	"net/http"

	"github.com/julienschmidt/httprouter"
)

func AddRoutes() http.Handler {
	var routes = []struct {
		method  string
		route   string
		handler httprouter.Handle
	}{
		{"GET", "/", IndexHandler},
		{"GET", "/obj/:id", GetObject},
		{"GET", "/obj", NotImplemented},
		{"GET", "/show/:id", ObjectShow},
		{"GET", "/downloads/:id", ObjectDownload},
		{"GET", "/downloads/:id/thumbnail", ObjectDownloadThumbnail},
		{"GET", "/config", ConfigPage},
		{"POST", "/config", UpdateConfig},
	}

	r := httprouter.New()
	for _, route := range routes {
		r.Handle(route.method,
			route.route,
			route.handler)
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
