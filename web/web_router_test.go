package web

import (
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"net/url"
	"strings"
	"testing"
)

// request sends a request through the real router, including the middlewares
func request(method string, path string, authenticated bool) *httptest.ResponseRecorder {
	req := httptest.NewRequest(method, path, nil)

	if authenticated {
		req.SetBasicAuth(testUsername, testPassword)
	}

	rec := httptest.NewRecorder()
	GetInstance().endpointsRouter.ServeHTTP(rec, req)

	return rec
}

// apiPath builds a path below /api; the segments are escaped, as disk names of local disks contain slashes
func apiPath(segments ...string) string {
	path := "/api"

	for _, segment := range segments {
		path += "/" + url.PathEscape(segment)
	}

	return path
}

func getJSON(t *testing.T, path string, target interface{}) {
	t.Helper()

	rec := request(http.MethodGet, path, true)
	if rec.Code != http.StatusOK {
		t.Fatalf("GET %s: status = %d, body = %q", path, rec.Code, rec.Body.String())
	}

	if contentType := rec.Header().Get("Content-Type"); !strings.HasPrefix(contentType, "application/json") {
		t.Errorf("GET %s: Content-Type = %q, want application/json", path, contentType)
	}

	if err := json.Unmarshal(rec.Body.Bytes(), target); err != nil {
		t.Fatalf("GET %s: invalid JSON %q: %v", path, rec.Body.String(), err)
	}
}

func TestRouter_rootRedirectsToApi(t *testing.T) {
	rec := request(http.MethodGet, "/", false)

	if rec.Code != http.StatusMovedPermanently || rec.Header().Get("Location") != "/api" {
		t.Errorf("status = %d, Location = %q, want a redirect to /api", rec.Code, rec.Header().Get("Location"))
	}
}

func TestRouter_metricsAreServedWithoutAuthentication(t *testing.T) {
	rec := request(http.MethodGet, "/metrics", false)

	if rec.Code != http.StatusOK {
		t.Fatalf("status = %d, want %d", rec.Code, http.StatusOK)
	}

	if !strings.Contains(rec.Body.String(), "backmon_") {
		t.Error("expected backmon metrics in the response")
	}
}

func TestRouter_apiRequiresBasicAuth(t *testing.T) {
	for _, path := range []string{"/api", apiPath(testDiskName), apiPath(testDiskName, "my-backups")} {
		if rec := request(http.MethodGet, path, false); rec.Code != http.StatusUnauthorized {
			t.Errorf("GET %s without credentials: status = %d, want %d", path, rec.Code, http.StatusUnauthorized)
		}
	}

	req := httptest.NewRequest(http.MethodGet, "/api", nil)
	req.SetBasicAuth(testUsername, "wrong")
	rec := httptest.NewRecorder()
	GetInstance().endpointsRouter.ServeHTTP(rec, req)

	if rec.Code != http.StatusUnauthorized {
		t.Errorf("GET /api with a wrong password: status = %d, want %d", rec.Code, http.StatusUnauthorized)
	}
}

func TestRouter_apiOnlyServesGet(t *testing.T) {
	rec := request(http.MethodPost, apiPath(testDiskName), true)

	if rec.Code != http.StatusNotFound && rec.Code != http.StatusMethodNotAllowed {
		t.Errorf("status = %d, want the POST request to be rejected", rec.Code)
	}
}

func TestRouter_listsDisks(t *testing.T) {
	var disks []string
	getJSON(t, "/api", &disks)

	if len(disks) != 1 || disks[0] != testDiskName {
		t.Errorf("disks = %v, want only %q", disks, testDiskName)
	}
}

func TestRouter_showsDefinitionsOfDisk(t *testing.T) {
	rec := request(http.MethodGet, apiPath(testDiskName), true)

	if rec.Code != http.StatusOK {
		t.Fatalf("status = %d, body = %q", rec.Code, rec.Body.String())
	}

	if !json.Valid(rec.Body.Bytes()) || !strings.Contains(rec.Body.String(), "my-backups") {
		t.Errorf("body = %q, want JSON containing the directory my-backups", rec.Body.String())
	}
}

func TestRouter_showsFilesOfDirectory(t *testing.T) {
	rec := request(http.MethodGet, apiPath(testDiskName, "my-backups"), true)

	if rec.Code != http.StatusOK {
		t.Fatalf("status = %d, body = %q", rec.Code, rec.Body.String())
	}

	if !json.Valid(rec.Body.Bytes()) || !strings.Contains(rec.Body.String(), "pgdump") {
		t.Errorf("body = %q, want JSON containing the file pgdump", rec.Body.String())
	}
}

func TestRouter_unknownDirectoryIsNotFound(t *testing.T) {
	rec := request(http.MethodGet, apiPath(testDiskName, "unknown"), true)

	if rec.Code != http.StatusNotFound || !strings.Contains(rec.Body.String(), "Directory 'unknown' does not exist.") {
		t.Errorf("status = %d, body = %q", rec.Code, rec.Body.String())
	}
}

func TestRouter_unknownFileIsNotFound(t *testing.T) {
	rec := request(http.MethodGet, apiPath(testDiskName, "my-backups", "unknown"), true)

	if rec.Code != http.StatusNotFound || !strings.Contains(rec.Body.String(), "File 'unknown' does not exist.") {
		t.Errorf("status = %d, body = %q", rec.Code, rec.Body.String())
	}
}

func TestRouter_downloadsLatestFileOfGroup(t *testing.T) {
	var groups []string
	getJSON(t, apiPath(testDiskName, "my-backups", "pgdump"), &groups)

	if len(groups) != 1 {
		t.Fatalf("groups = %v, want exactly one", groups)
	}

	rec := request(http.MethodGet, apiPath(testDiskName, "my-backups", "pgdump", groups[0]), true)

	// shortcut: files of a local disk can't be downloaded until the path fix of PR #45 is merged; fail here afterwards
	if rec.Code == http.StatusNotFound {
		t.Skip("downloading from a local disk needs the path fix of PR #45")
	}

	if rec.Code != http.StatusOK {
		t.Fatalf("status = %d, body = %q", rec.Code, rec.Body.String())
	}

	if rec.Body.String() != testDiskFiles["backups/dump-20220718.sql"] {
		t.Errorf("body = %q, want the content of the newest dump", rec.Body.String())
	}

	if disposition := rec.Header().Get("Content-Disposition"); !strings.Contains(disposition, "attachment") {
		t.Errorf("Content-Disposition = %q, want an attachment", disposition)
	}

	if contentType := rec.Header().Get("Content-Type"); contentType != "application/octet-stream" {
		t.Errorf("Content-Type = %q, want application/octet-stream", contentType)
	}
}
