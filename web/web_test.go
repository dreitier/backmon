package web

import (
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"github.com/gorilla/mux"
)

// unescape must URL-decode every value in the vars map in place; values that
// are already decoded (or cannot be decoded) are left untouched.
func TestUnescape(t *testing.T) {
	vars := map[string]string{
		"disk":  "my%20disk",
		"dir":   "a%2Fb",
		"file":  "plain",
		"empty": "",
	}

	unescape(vars)

	if vars["disk"] != "my disk" {
		t.Errorf("disk = %q, want %q", vars["disk"], "my disk")
	}
	if vars["dir"] != "a/b" {
		t.Errorf("dir = %q, want %q", vars["dir"], "a/b")
	}
	if vars["file"] != "plain" {
		t.Errorf("file = %q, want %q", vars["file"], "plain")
	}
	if vars["empty"] != "" {
		t.Errorf("empty = %q, want empty string", vars["empty"])
	}
}

// unescape must leave a value unchanged when it contains an invalid percent
// escape (PathUnescape returns an error for those).
func TestUnescapeInvalidEscapeLeftUnchanged(t *testing.T) {
	vars := map[string]string{"file": "100%broken"}

	unescape(vars)

	if vars["file"] != "100%broken" {
		t.Errorf("invalid escape should be left unchanged, got %q", vars["file"])
	}
}

// writeData marshals the payload to JSON, sets the JSON content type and writes
// the body with a 200 status on success.
func TestWriteDataSuccess(t *testing.T) {
	rec := httptest.NewRecorder()

	writeData(rec, map[string]int{"answer": 42})

	if rec.Code != http.StatusOK {
		t.Errorf("status = %d, want %d", rec.Code, http.StatusOK)
	}
	if ct := rec.Header().Get("Content-Type"); ct != "application/json; charset=utf-8" {
		t.Errorf("content-type = %q", ct)
	}
	if body := strings.TrimSpace(rec.Body.String()); body != `{"answer":42}` {
		t.Errorf("body = %q, want %q", body, `{"answer":42}`)
	}
}

// writeData must respond with 500 and the marshalling error when the payload
// cannot be serialised (channels are not JSON-encodable).
func TestWriteDataMarshalError(t *testing.T) {
	rec := httptest.NewRecorder()

	writeData(rec, make(chan int))

	if rec.Code != http.StatusInternalServerError {
		t.Errorf("status = %d, want %d", rec.Code, http.StatusInternalServerError)
	}
	if rec.Body.Len() == 0 {
		t.Error("expected an error message in the body")
	}
}

func TestNotFoundHelpers(t *testing.T) {
	cases := []struct {
		name string
		fn   func(http.ResponseWriter, string)
		arg  string
		want string
	}{
		{"disk", diskNotFound, "d1", "Disk 'd1' does not exist."},
		{"directory", directoryNotFound, "dir1", "Directory 'dir1' does not exist."},
		{"file", fileNotFound, "f1", "File 'f1' does not exist."},
		{"group", groupNotFound, "g1", "Group 'g1' does not exist."},
	}

	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			rec := httptest.NewRecorder()
			tc.fn(rec, tc.arg)

			if rec.Code != http.StatusNotFound {
				t.Errorf("status = %d, want %d", rec.Code, http.StatusNotFound)
			}
			if rec.Body.String() != tc.want {
				t.Errorf("body = %q, want %q", rec.Body.String(), tc.want)
			}
		})
	}
}

// BaseHandler permanently redirects the root path to /api.
func TestBaseHandler(t *testing.T) {
	rec := httptest.NewRecorder()
	req := httptest.NewRequest(http.MethodGet, "/", nil)

	BaseHandler(rec, req)

	if rec.Code != http.StatusMovedPermanently {
		t.Errorf("status = %d, want %d", rec.Code, http.StatusMovedPermanently)
	}
	if loc := rec.Header().Get("Location"); loc != "/api" {
		t.Errorf("Location = %q, want /api", loc)
	}
}

// With no disks configured, EnvHandler must emit an empty JSON array.
func TestEnvHandlerNoDisks(t *testing.T) {
	rec := httptest.NewRecorder()
	req := httptest.NewRequest(http.MethodGet, "/api", nil)

	EnvHandler(rec, req)

	if rec.Code != http.StatusOK {
		t.Errorf("status = %d, want %d", rec.Code, http.StatusOK)
	}
	if body := strings.TrimSpace(rec.Body.String()); body != "[]" {
		t.Errorf("body = %q, want []", body)
	}
}

// Requesting an unknown disk yields a 404 with the disk-not-found message.
func TestDiskInfoHandlerNotFound(t *testing.T) {
	rec := httptest.NewRecorder()
	req := httptest.NewRequest(http.MethodGet, "/api/ghost", nil)
	req = mux.SetURLVars(req, map[string]string{"disk": "ghost"})

	DiskInfoHandler(rec, req)

	if rec.Code != http.StatusNotFound {
		t.Errorf("status = %d, want %d", rec.Code, http.StatusNotFound)
	}
	if rec.Body.String() != "Disk 'ghost' does not exist." {
		t.Errorf("body = %q", rec.Body.String())
	}
}

// DirectoryInfoHandler also fails at the disk lookup when the disk is unknown.
func TestDirectoryInfoHandlerUnknownDisk(t *testing.T) {
	rec := httptest.NewRecorder()
	req := httptest.NewRequest(http.MethodGet, "/api/ghost/dir", nil)
	req = mux.SetURLVars(req, map[string]string{"disk": "ghost", "dir": "dir"})

	DirectoryInfoHandler(rec, req)

	if rec.Code != http.StatusNotFound {
		t.Errorf("status = %d, want %d", rec.Code, http.StatusNotFound)
	}
	if rec.Body.String() != "Disk 'ghost' does not exist." {
		t.Errorf("body = %q", rec.Body.String())
	}
}

// FileInfoHandler returns file-not-found when the variations lookup yields
// nothing (no disks configured).
func TestFileInfoHandlerNotFound(t *testing.T) {
	rec := httptest.NewRecorder()
	req := httptest.NewRequest(http.MethodGet, "/api/ghost/dir/file", nil)
	req = mux.SetURLVars(req, map[string]string{"disk": "ghost", "dir": "dir", "file": "file"})

	FileInfoHandler(rec, req)

	if rec.Code != http.StatusNotFound {
		t.Errorf("status = %d, want %d", rec.Code, http.StatusNotFound)
	}
	if rec.Body.String() != "File 'file' does not exist." {
		t.Errorf("body = %q", rec.Body.String())
	}
}

// LatestFileHandler returns group-not-found/404 when the download target cannot
// be resolved.
func TestLatestFileHandlerNotFound(t *testing.T) {
	rec := httptest.NewRecorder()
	req := httptest.NewRequest(http.MethodGet, "/api/ghost/dir/file/latest", nil)
	req = mux.SetURLVars(req, map[string]string{"disk": "ghost", "dir": "dir", "file": "file", "variant": "latest"})

	LatestFileHandler(rec, req)

	if rec.Code != http.StatusNotFound {
		t.Errorf("status = %d, want %d", rec.Code, http.StatusNotFound)
	}
	if !strings.Contains(rec.Body.String(), "Group 'latest' does not exist.") {
		t.Errorf("body = %q, want it to contain the group-not-found message", rec.Body.String())
	}
}
