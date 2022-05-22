package web

import (
	"github.com/dreitier/cloudmon/metrics"
	"github.com/gorilla/mux"
	"net/http"
	"net/url"
)

var Router *mux.Router

const BucketInfoRoute = "bucket_info_route"
const LatestFileRoute = "latest_file_route"

func init () {
	Router = mux.NewRouter().UseEncodedPath()
	Router.StrictSlash(true)
	Router.HandleFunc("/", BaseHandler)
	Router.Handle("/metrics", metrics.Handler())

	Router.HandleFunc("/api", EnvHandler)
	Router.HandleFunc("/api/{bucket}", BucketInfoHandler).Methods("GET")
	Router.HandleFunc("/api/{bucket}/{dir}", DirectoryInfoHandler).Methods("GET")
	Router.HandleFunc("/api/{bucket}/{dir}/{file}", FileInfoHandler).Methods("GET")
	Router.HandleFunc("/api/{bucket}/{dir}/{file}/{variant}", LatestFileHandler).Methods("GET")
}

// Base route to access the API Documentation.
func BaseHandler(w http.ResponseWriter, r *http.Request) {
	http.Redirect(w, r, "/api", http.StatusMovedPermanently)
}

func EnvHandler(w http.ResponseWriter, r *http.Request) {
	GetBuckets(w)
	//_, _ = w.Write([]byte(`<h1>Available Buckets:</h1>`))
	//for _, bucket := range response {
	//	_, _ = w.Write([]byte(`<a href="/env/`))
	//	_, _ = w.Write([]byte(bucket.SafeName))
	//	_, _ = w.Write([]byte(`/">`))
	//	_, _ = w.Write([]byte(bucket.Name))
	//	_, _ = w.Write([]byte(`</a><br>`))
	//}
}

func BucketInfoHandler(w http.ResponseWriter, r *http.Request) {
	vars := mux.Vars(r)
	unescape(vars)
	bucketName := vars["bucket"]

	GetDirectories(w, bucketName)
}

func DirectoryInfoHandler(w http.ResponseWriter, r *http.Request) {
	vars := mux.Vars(r)
	unescape(vars)
	bucketName := vars["bucket"]
	dirName := vars["dir"]

	GetFiles(w, bucketName, dirName)
}

func FileInfoHandler(w http.ResponseWriter, r *http.Request) {
	vars := mux.Vars(r)
	unescape(vars)
	bucketName := vars["bucket"]
	dirName := vars["dir"]
	fileName := vars["file"]

	GetVariations(w, bucketName, dirName, fileName)
}

func bucketNotFound(w http.ResponseWriter, bucket string) {
	w.WriteHeader(http.StatusNotFound)
	_, _ = w.Write([]byte(`Bucket '`))
	_, _ = w.Write([]byte(bucket))
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
	bucketName := vars["bucket"]
	dirName := vars["dir"]
	fileName := vars["file"]
	variant := vars["variant"]

	Download(w, bucketName, dirName, fileName, variant)
}

func unescape(vars map[string]string) {
	for key, val := range vars {
		val, err := url.PathUnescape(val)
		if err == nil {
			vars[key] = val
		}
	}
}
