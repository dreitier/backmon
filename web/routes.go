package web

import (
	"github.com/dreitier/cloudmon/metrics"
	"github.com/gorilla/mux"
	"net/http"
	"net/url"
	"github.com/dreitier/cloudmon/config"
	log "github.com/sirupsen/logrus"
	"github.com/goji/httpauth"
	"sync"
)

type routeConfiguration struct {
	endpointsRouter *mux.Router
}

var (
	instance *routeConfiguration
	once     sync.Once
)

const DiskInfoRoute = "disk_info_route"
const LatestFileRoute = "latest_file_route"
const HttpMethodGet = "GET"

func init () {
	instance = &routeConfiguration{
		endpointsRouter: mux.NewRouter().UseEncodedPath(),
	}
}

func GetInstance() *routeConfiguration {
	once.Do(func() {
		instance.endpointsRouter.StrictSlash(true)
		instance.endpointsRouter.HandleFunc("/", BaseHandler)
		instance.endpointsRouter.Handle("/metrics", metrics.Handler())

		// #2: for /api, we are using an HTTP Basic Auth middleware
		apiEndpoint := instance.endpointsRouter.PathPrefix("/api").Subrouter()

		apiEndpoint.Use(loggingMiddleware)

		if config.GetInstance().Http().BasicAuth != nil {
			log.Debug("Registering Basic Auth middleware")

			apiEndpoint.Use(httpauth.SimpleBasicAuth(
				config.GetInstance().Http().BasicAuth.Username,
				config.GetInstance().Http().BasicAuth.Password,
			))
		}

		apiEndpoint.HandleFunc("", EnvHandler)
		apiEndpoint.HandleFunc("/{disk}", DiskInfoHandler).Methods(HttpMethodGet)
		apiEndpoint.HandleFunc("/{disk}/{dir}", DirectoryInfoHandler).Methods(HttpMethodGet)
		apiEndpoint.HandleFunc("/{disk}/{dir}/{file}", FileInfoHandler).Methods(HttpMethodGet)
	
		if config.GetInstance().Downloads().Enabled {
			log.Debug("Registering GET handler for artifact downloads")
			apiEndpoint.HandleFunc("/{disk}/{dir}/{file}/{variant}", LatestFileHandler).Methods(HttpMethodGet)
		}
	})

	return instance
}

func loggingMiddleware(next http.Handler) http.Handler {
    return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
        // Do stuff here
        log.Debugf("GET %s", r.RequestURI)
        // Call the next handler, which can be another middleware in the chain, or the final handler.
        next.ServeHTTP(w, r)
    })
}

// Base route to access the API Documentation.
func BaseHandler(w http.ResponseWriter, r *http.Request) {
	http.Redirect(w, r, "/api", http.StatusMovedPermanently)
}

func EnvHandler(w http.ResponseWriter, r *http.Request) {
	GetDisks(w)
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

	GetDirectories(w, diskName)
}

func DirectoryInfoHandler(w http.ResponseWriter, r *http.Request) {
	vars := mux.Vars(r)
	unescape(vars)
	diskName := vars["disk"]
	dirName := vars["dir"]

	GetFiles(w, diskName, dirName)
}

func FileInfoHandler(w http.ResponseWriter, r *http.Request) {
	vars := mux.Vars(r)
	unescape(vars)
	diskName := vars["disk"]
	dirName := vars["dir"]
	fileName := vars["file"]

	GetVariations(w, diskName, dirName, fileName)
}

func diskNotFound(w http.ResponseWriter, disk string) {
	w.WriteHeader(http.StatusNotFound)
	_, _ = w.Write([]byte(`Disk '`))
	_, _ = w.Write([]byte(disk))
	_, _ = w.Write([]byte(`' does not exist.`))
}

func directoryNotFound(w http.ResponseWriter, directory string) {
	w.WriteHeader(http.StatusNotFound)
	_, _ = w.Write([]byte(`Directory '`))
	_, _ = w.Write([]byte(directory))
	_, _ = w.Write([]byte(`' does not exist.`))
}

func fileNotFound(w http.ResponseWriter, file string) {
	w.WriteHeader(http.StatusNotFound)
	_, _ = w.Write([]byte(`File '`))
	_, _ = w.Write([]byte(file))
	_, _ = w.Write([]byte(`' does not exist.`))
}

func groupNotFound(w http.ResponseWriter, group string) {
	w.WriteHeader(http.StatusNotFound)
	_, _ = w.Write([]byte(`Group '`))
	_, _ = w.Write([]byte(group))
	_, _ = w.Write([]byte(`' does not exist.`))
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
