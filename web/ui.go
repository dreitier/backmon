package web

import (
	"bytes"
	"embed"
	"html/template"
	"io/fs"
	"mime"
	"net/http"
	"path"
	"strings"

	log "github.com/sirupsen/logrus"
)

// The web UI is a static single page application below /ui. It reads the summary from /metrics and browses /api
// with the browser's own credentials, so it neither needs its own endpoints nor exposes anything new.
const uiRoot = "/ui/"

//go:embed ui
var uiFiles embed.FS

// uiAssets contains the files of the ui directory without the directory itself
var uiAssets = mustSub(uiFiles, "ui")

var uiIndex = template.Must(template.ParseFS(uiAssets, "index.html"))

// uiContentSecurityPolicy only allows the UI's own scripts and styles and requests to the same origin
const uiContentSecurityPolicy = "default-src 'none'; script-src 'self'; style-src 'self'; img-src 'self' data:; " +
	"connect-src 'self'; base-uri 'self'; form-action 'none'; frame-ancestors 'none'"

func mustSub(fsys fs.FS, dir string) fs.FS {
	sub, err := fs.Sub(fsys, dir)
	if err != nil {
		panic(err)
	}

	return sub
}

// acceptsHTML is true for requests of browsers. API clients like curl send no or other media types and still get
// redirected to /api.
func acceptsHTML(r *http.Request) bool {
	for _, value := range r.Header.Values("Accept") {
		for _, mediaType := range strings.Split(value, ",") {
			mediaType, _, _ = strings.Cut(mediaType, ";")

			if strings.EqualFold(strings.TrimSpace(mediaType), "text/html") {
				return true
			}
		}
	}

	return false
}

func setUIHeaders(w http.ResponseWriter) {
	w.Header().Set("Content-Security-Policy", uiContentSecurityPolicy)
	w.Header().Set("X-Content-Type-Options", "nosniff")
	w.Header().Set("Referrer-Policy", "same-origin")
	// the files are embedded into the binary; revalidating them is cheap and an upgrade is picked up immediately
	w.Header().Set("Cache-Control", "no-cache")
}

// UIHandler serves the start page of the web UI. All paths of the page are relative to the <base> element, which
// contains the path prefix of a reverse proxy, so that the UI also works below a prefix like /backmon/.
func UIHandler(w http.ResponseWriter, r *http.Request) {
	var page bytes.Buffer

	if err := uiIndex.Execute(&page, struct{ Base string }{Base: forwardedPrefix(r) + "/"}); err != nil {
		log.Errorf("Failed to render the web UI: %v", err)
		http.Error(w, "Failed to render the web UI.", http.StatusInternalServerError)

		return
	}

	setUIHeaders(w)
	w.Header().Set("Content-Type", "text/html; charset=utf-8")
	_, _ = w.Write(page.Bytes())
}

// UIAssetHandler serves the scripts and stylesheets of the web UI. Directories are not listed and the template of the
// start page is only served through UIHandler.
func UIAssetHandler(w http.ResponseWriter, r *http.Request) {
	name := strings.TrimPrefix(r.URL.Path, uiRoot)

	if name == "" || name == "index.html" || strings.HasSuffix(name, "/") || !fs.ValidPath(name) {
		http.NotFound(w, r)
		return
	}

	content, err := fs.ReadFile(uiAssets, name)
	if err != nil {
		http.NotFound(w, r)
		return
	}

	contentType := mime.TypeByExtension(path.Ext(name))
	if contentType == "" {
		contentType = "application/octet-stream"
	}

	setUIHeaders(w)
	w.Header().Set("Content-Type", contentType)
	_, _ = w.Write(content)
}
