package web

import (
	"fmt"
	"net/http"
	"net/url"
	"sync"

	"github.com/goji/httpauth"
	"github.com/gorilla/mux"
	log "github.com/sirupsen/logrus"

	"github.com/dreitier/backmon/config"
	"github.com/dreitier/backmon/metrics"
)

type RouteConfiguration struct {
	endpointsRouter *mux.Router
}

var (
	instance *RouteConfiguration
	once     sync.Once
)

const HttpMethodGet = "GET"

func GetInstance() *RouteConfiguration {
	once.Do(func() {
		instance = &RouteConfiguration{
			endpointsRouter: mux.NewRouter().UseEncodedPath(),
		}

		instance.endpointsRouter.StrictSlash(true)
		instance.endpointsRouter.HandleFunc("/", BaseHandler)
		instance.endpointsRouter.Handle("/metrics", metrics.Handler())
		instance.endpointsRouter.PathPrefix(uiRoot).HandlerFunc(UIAssetHandler).Methods(HttpMethodGet)

		// #2: for /api, we are using an HTTP Basic Auth middleware
		apiEndpoint := instance.endpointsRouter.PathPrefix(apiRoot).Subrouter()

		apiEndpoint.Use(loggingMiddleware)

		apiEndpoint.Use(basicAuthMiddleware)

		apiEndpoint.HandleFunc("", EnvHandler)
		apiEndpoint.HandleFunc("/{disk}", DiskInfoHandler).Methods(HttpMethodGet)
		apiEndpoint.HandleFunc("/{disk}/{dir}", DirectoryInfoHandler).Methods(HttpMethodGet)
		apiEndpoint.HandleFunc("/{disk}/{dir}/{file}", FileInfoHandler).Methods(HttpMethodGet)

		// the route only matches while downloads are enabled, so a request gets the same response as for any other unknown path
		apiEndpoint.HandleFunc("/{disk}/{dir}/{file}/{variant}", LatestFileHandler).Methods(HttpMethodGet).MatcherFunc(downloadsEnabled)
	})

	return instance
}

// basicAuthMiddleware protects the API if `http.basic_auth` is configured. The configuration is read on each
// request, so that credentials changed by a configuration reload apply without a restart.
func basicAuthMiddleware(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		basicAuth := config.GetInstance().Http().BasicAuth

		if basicAuth == nil {
			next.ServeHTTP(w, r)
			return
		}

		httpauth.SimpleBasicAuth(basicAuth.Username, basicAuth.Password)(next).ServeHTTP(w, r)
	})
}

// downloadsEnabled reads `downloads.enabled` on each request, so that a configuration reload applies without a restart
func downloadsEnabled(_ *http.Request, _ *mux.RouteMatch) bool {
	return config.GetInstance().Downloads().Enabled
}

func loggingMiddleware(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		// Do stuff here
		log.Debugf("GET %s", r.RequestURI)
		// Call the next handler, which can be another middleware in the chain, or the final handler.
		next.ServeHTTP(w, r)
	})
}

// BaseHandler serves the web UI to browsers and permanently redirects all other clients to the API.
func BaseHandler(w http.ResponseWriter, r *http.Request) {
	if acceptsHTML(r) {
		UIHandler(w, r)
		return
	}

	// forwardedPrefix only returns a plain absolute path, so the redirect can't leave the host of the request
	http.Redirect(w, r, forwardedPrefix(r)+apiRoot, http.StatusMovedPermanently) //nolint:gosec // G710: see above
}

func EnvHandler(w http.ResponseWriter, r *http.Request) {
	GetDisks(w, r)
	//_, _ = w.Write([]byte(`<h1>Available Disks:</h1>`))
	//for _, disk := range response {
	//	_, _ = w.Write([]byte(`<a href="/env/`))
	//	_, _ = w.Write([]byte(disk.SafeName))
	//	_, _ = w.Write([]byte(`/">`))
	//	_, _ = w.Write([]byte(disk.Name))
	//	_, _ = w.Write([]byte(`</a><br>`))
	//}
}

func DiskInfoHandler(w http.ResponseWriter, r *http.Request) {
	vars := mux.Vars(r)
	unescape(vars)
	diskName := vars["disk"]

	GetDirectories(w, r, diskName)
}

func DirectoryInfoHandler(w http.ResponseWriter, r *http.Request) {
	vars := mux.Vars(r)
	unescape(vars)
	diskName := vars["disk"]
	dirName := vars["dir"]

	GetFiles(w, r, diskName, dirName)
}

func FileInfoHandler(w http.ResponseWriter, r *http.Request) {
	vars := mux.Vars(r)
	unescape(vars)
	diskName := vars["disk"]
	dirName := vars["dir"]
	fileName := vars["file"]

	GetVariations(w, r, diskName, dirName, fileName)
}

func diskNotFound(w http.ResponseWriter, disk string) {
	http.Error(w, fmt.Sprintf("Disk '%s' does not exist.", disk), http.StatusNotFound)
}

func definitionsNotFound(w http.ResponseWriter, disk string) {
	http.Error(w, fmt.Sprintf("Disk '%s' has no valid backup definitions.", disk), http.StatusNotFound)
}

func directoryNotFound(w http.ResponseWriter, directory string) {
	http.Error(w, fmt.Sprintf("Directory '%s' does not exist.", directory), http.StatusNotFound)
}

func fileNotFound(w http.ResponseWriter, file string) {
	http.Error(w, fmt.Sprintf("File '%s' does not exist.", file), http.StatusNotFound)
}

func groupNotFound(w http.ResponseWriter, group string) {
	http.Error(w, fmt.Sprintf("Group '%s' does not exist.", group), http.StatusNotFound)
}

func LatestFileHandler(w http.ResponseWriter, r *http.Request) {
	vars := mux.Vars(r)
	unescape(vars)
	diskName := vars["disk"]
	dirName := vars["dir"]
	fileName := vars["file"]
	variant := vars["variant"]

	Download(w, diskName, dirName, fileName, variant)
}

func unescape(vars map[string]string) {
	for key, val := range vars {
		val, err := url.PathUnescape(val)
		if err == nil {
			vars[key] = val
		}
	}
}
