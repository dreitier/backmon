package web

import (
	"github.com/dreitier/cloudmon/metrics"
	"github.com/gorilla/mux"
	"net/http"
	"net/url"
)

var Router *mux.Router

const DiskInfoRoute = "disk_info_route"
const LatestFileRoute = "latest_file_route"

func init () {
	Router = mux.NewRouter().UseEncodedPath()
	Router.StrictSlash(true)
	Router.HandleFunc("/", BaseHandler)
	Router.Handle("/metrics", metrics.Handler())

	Router.HandleFunc("/api", EnvHandler)
	Router.HandleFunc("/api/{disk}", DiskInfoHandler).Methods("GET")
	Router.HandleFunc("/api/{disk}/{dir}", DirectoryInfoHandler).Methods("GET")
	Router.HandleFunc("/api/{disk}/{dir}/{file}", FileInfoHandler).Methods("GET")
	Router.HandleFunc("/api/{disk}/{dir}/{file}/{variant}", LatestFileHandler).Methods("GET")
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
