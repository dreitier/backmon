package provider

// An in-memory implementation of the parts of the Google Cloud Storage JSON API which backmon and its tests use.

import (
	"encoding/json"
	"io"
	"net/http"
	"net/http/httptest"
	"net/url"
	"sort"
	"strconv"
	"strings"
	"sync"
	"testing"
	"time"
)

type fakeGCSObject struct {
	content     []byte
	contentType string
	created     time.Time
}

type fakeGCS struct {
	*httptest.Server

	mu      sync.Mutex
	buckets map[string]map[string]*fakeGCSObject
	// maximum number of items per page of a listing
	pageSize int
	// buckets whose objects can't be accessed (403)
	denied map[string]bool
	// status codes for the next requests, one per request; 0 means that the connection is dropped and -1 that the
	// request is answered regularly
	failures []int
	// all requests, e.g. "GET /storage/v1/b"
	requests []string
	// Authorization headers of all requests
	authorizations []string
	// project parameters of all bucket listings
	projects []string
}

func newFakeGCS(t *testing.T) *fakeGCS {
	t.Helper()

	f := &fakeGCS{
		buckets:  make(map[string]map[string]*fakeGCSObject),
		pageSize: 1000,
		denied:   make(map[string]bool),
	}
	f.Server = httptest.NewServer(f)
	t.Cleanup(f.Close)

	return f
}

func (f *fakeGCS) put(bucket string, name string, content string) {
	f.mu.Lock()
	defer f.mu.Unlock()

	if f.buckets[bucket] == nil {
		f.buckets[bucket] = make(map[string]*fakeGCSObject)
	}

	f.buckets[bucket][name] = &fakeGCSObject{content: []byte(content), created: time.Now()}
}

// failAfter lets the request after the next `successful` ones fail with the given status
func (f *fakeGCS) failAfter(successful int, status int) {
	f.mu.Lock()
	defer f.mu.Unlock()

	f.failures = nil
	for range successful {
		f.failures = append(f.failures, -1)
	}
	f.failures = append(f.failures, status)
}

func (f *fakeGCS) requestCount(request string) int {
	f.mu.Lock()
	defer f.mu.Unlock()

	count := 0
	for _, r := range f.requests {
		if r == request {
			count++
		}
	}

	return count
}

func fakeGCSError(w http.ResponseWriter, status int, message string) {
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(status)
	_ = json.NewEncoder(w).Encode(map[string]interface{}{"error": map[string]interface{}{"code": status, "message": message}})
}

func fakeGCSJSON(w http.ResponseWriter, value interface{}) {
	w.Header().Set("Content-Type", "application/json")
	_ = json.NewEncoder(w).Encode(value)
}

// page returns the names of the requested page and the token of the next one
func (f *fakeGCS) page(r *http.Request, names []string) ([]string, string) {
	sort.Strings(names)

	start := 0
	if token := r.URL.Query().Get("pageToken"); token != "" {
		start, _ = strconv.Atoi(token)
	}

	size := f.pageSize
	if maxResults, err := strconv.Atoi(r.URL.Query().Get("maxResults")); err == nil && maxResults < size {
		size = maxResults
	}

	end := min(start+size, len(names))
	next := ""
	if end < len(names) {
		next = strconv.Itoa(end)
	}

	return names[start:end], next
}

func (f *fakeGCS) ServeHTTP(w http.ResponseWriter, r *http.Request) {
	f.mu.Lock()
	defer f.mu.Unlock()

	// the escaped path keeps slashes of object names (%2F) apart from path separators
	path := r.URL.EscapedPath()
	f.requests = append(f.requests, r.Method+" "+path)
	f.authorizations = append(f.authorizations, r.Header.Get("Authorization"))

	if len(f.failures) > 0 {
		status := f.failures[0]
		f.failures = f.failures[1:]

		if status == 0 {
			panic(http.ErrAbortHandler)
		}

		if status > 0 {
			fakeGCSError(w, status, "injected failure")
			return
		}
	}

	if upload, ok := strings.CutPrefix(path, "/upload/storage/v1/b/"); ok {
		f.upload(w, r, strings.TrimSuffix(upload, "/o"))
		return
	}

	rest, ok := strings.CutPrefix(path, "/storage/v1/b")
	if !ok {
		fakeGCSError(w, http.StatusNotFound, "unknown path")
		return
	}

	segments := strings.Split(strings.Trim(rest, "/"), "/")

	switch {
	case rest == "" || rest == "/":
		f.serveBuckets(w, r)
	case len(segments) == 1:
		f.serveBucket(w, r, unescape(segments[0]))
	case len(segments) == 2 && segments[1] == "o":
		f.serveObjects(w, r, unescape(segments[0]))
	case len(segments) == 3 && segments[1] == "o":
		f.serveObject(w, r, unescape(segments[0]), unescape(segments[2]))
	default:
		fakeGCSError(w, http.StatusNotFound, "unknown path")
	}
}

func unescape(segment string) string {
	unescaped, err := url.PathUnescape(segment)
	if err != nil {
		return segment
	}

	return unescaped
}

func (f *fakeGCS) serveBuckets(w http.ResponseWriter, r *http.Request) {
	project := r.URL.Query().Get("project")
	if project == "" {
		fakeGCSError(w, http.StatusBadRequest, "Required parameter: project")
		return
	}

	if r.Method == http.MethodPost {
		var body struct {
			Name string `json:"name"`
		}
		if err := json.NewDecoder(r.Body).Decode(&body); err != nil || body.Name == "" {
			fakeGCSError(w, http.StatusBadRequest, "invalid bucket")
			return
		}

		if f.buckets[body.Name] == nil {
			f.buckets[body.Name] = make(map[string]*fakeGCSObject)
		}

		fakeGCSJSON(w, map[string]string{"name": body.Name})
		return
	}

	f.projects = append(f.projects, project)

	names := make([]string, 0, len(f.buckets))
	for name := range f.buckets {
		names = append(names, name)
	}

	names, next := f.page(r, names)
	items := make([]map[string]string, 0, len(names))
	for _, name := range names {
		items = append(items, map[string]string{"name": name})
	}

	fakeGCSJSON(w, map[string]interface{}{"items": items, "nextPageToken": next})
}

func (f *fakeGCS) serveBucket(w http.ResponseWriter, r *http.Request, bucket string) {
	if f.buckets[bucket] == nil {
		fakeGCSError(w, http.StatusNotFound, "The specified bucket does not exist.")
		return
	}

	if r.Method == http.MethodDelete {
		if len(f.buckets[bucket]) > 0 {
			fakeGCSError(w, http.StatusConflict, "The bucket you tried to delete is not empty.")
			return
		}

		delete(f.buckets, bucket)
		w.WriteHeader(http.StatusNoContent)
		return
	}

	fakeGCSJSON(w, map[string]string{"name": bucket})
}

// objects returns the objects of a bucket, or writes an error
func (f *fakeGCS) objects(w http.ResponseWriter, bucket string) (map[string]*fakeGCSObject, bool) {
	if f.denied[bucket] {
		fakeGCSError(w, http.StatusForbidden, "backmon does not have storage.objects.list access to the Google Cloud Storage bucket.")
		return nil, false
	}

	objects := f.buckets[bucket]
	if objects == nil {
		fakeGCSError(w, http.StatusNotFound, "The specified bucket does not exist.")
		return nil, false
	}

	return objects, true
}

func fakeGCSMetadata(name string, obj *fakeGCSObject) map[string]string {
	created := obj.created.UTC().Format(time.RFC3339Nano)

	return map[string]string{
		"name": name,
		// 64-bit integers are strings in the JSON API
		"size":        strconv.Itoa(len(obj.content)),
		"timeCreated": created,
		"updated":     created,
	}
}

func (f *fakeGCS) serveObjects(w http.ResponseWriter, r *http.Request, bucket string) {
	objects, ok := f.objects(w, bucket)
	if !ok {
		return
	}

	names := make([]string, 0, len(objects))
	for name := range objects {
		names = append(names, name)
	}

	names, next := f.page(r, names)
	items := make([]map[string]string, 0, len(names))
	for _, name := range names {
		items = append(items, fakeGCSMetadata(name, objects[name]))
	}

	fakeGCSJSON(w, map[string]interface{}{"items": items, "nextPageToken": next})
}

func (f *fakeGCS) serveObject(w http.ResponseWriter, r *http.Request, bucket string, name string) {
	objects, ok := f.objects(w, bucket)
	if !ok {
		return
	}

	obj := objects[name]
	if obj == nil {
		fakeGCSError(w, http.StatusNotFound, "No such object: "+bucket+"/"+name)
		return
	}

	switch {
	case r.Method == http.MethodDelete:
		delete(objects, name)
		w.WriteHeader(http.StatusNoContent)
	case r.URL.Query().Get("alt") == "media":
		contentType := obj.contentType
		if contentType == "" {
			contentType = "application/octet-stream"
		}

		w.Header().Set("Content-Type", contentType)
		w.Header().Set("Content-Length", strconv.Itoa(len(obj.content)))
		_, _ = w.Write(obj.content)
	default:
		fakeGCSJSON(w, fakeGCSMetadata(name, obj))
	}
}

func (f *fakeGCS) upload(w http.ResponseWriter, r *http.Request, escapedBucket string) {
	bucket := unescape(escapedBucket)
	name := r.URL.Query().Get("name")

	if f.buckets[bucket] == nil || name == "" || r.Method != http.MethodPost {
		fakeGCSError(w, http.StatusBadRequest, "invalid upload")
		return
	}

	content, err := io.ReadAll(r.Body)
	if err != nil {
		fakeGCSError(w, http.StatusBadRequest, "invalid upload")
		return
	}

	obj := &fakeGCSObject{content: content, contentType: r.Header.Get("Content-Type"), created: time.Now()}
	f.buckets[bucket][name] = obj

	fakeGCSJSON(w, fakeGCSMetadata(name, obj))
}
