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

// The web UI is a static single page application whose assets are served below /ui. It reads the summary from /metrics and browses /api
// with the browser's own credentials, so it neither needs its own endpoints nor exposes anything new.
const uiRoot = "/ui/"

//go:embed ui
var uiFiles embed.FS

// uiAssets contains the files of the ui directory without the directory itself
var uiAssets = mustSub(uiFiles, "ui")

var uiIndex = template.Must(template.ParseFS(uiAssets, "index.html"))

// uiContentSecurityPolicy only allows the UI's own scripts and styles and requests to the same origin
const uiContentSecurityPolicy = "default-src 'none'; script-src 'self'; style-src 'self'; img-src 'self' data:; " +
	"font-src 'self'; connect-src 'self'; base-uri 'self'; form-action 'none'; frame-ancestors 'none'"

// uiContentTypes are the media types of assets which the mime package only knows from the files of the operating
// system; the container image has none of them
var uiContentTypes = map[string]string{
	".woff2": "font/woff2",
}

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

// uiAsset is a script, stylesheet or font of the web UI
type uiAsset struct {
	contentType string
	content     []byte
}

// uiAssetFiles are all files of the ui directory except the template of the start page, which is only served through
// UIHandler. They are read once, so that a request can only get one of these files and directories are never listed.
var uiAssetFiles = mustLoadUIAssets(uiAssets)

func mustLoadUIAssets(fsys fs.FS) map[string]uiAsset {
	assets := map[string]uiAsset{}

	err := fs.WalkDir(fsys, ".", func(name string, entry fs.DirEntry, err error) error {
		if err != nil || entry.IsDir() || name == "index.html" {
			return err
		}

		content, err := fs.ReadFile(fsys, name)
		if err != nil {
			return err
		}

		contentType := uiContentTypes[path.Ext(name)]
		if contentType == "" {
			contentType = mime.TypeByExtension(path.Ext(name))
		}

		if contentType == "" {
			contentType = "application/octet-stream"
		}

		assets[name] = uiAsset{contentType: contentType, content: content}

		return nil
	})
	if err != nil {
		panic(err)
	}

	return assets
}

// uiAssetHandler serves one asset of the web UI; each asset has its own route, so nothing of the request is used
func uiAssetHandler(asset uiAsset) http.HandlerFunc {
	return func(w http.ResponseWriter, _ *http.Request) {
		setUIHeaders(w)
		w.Header().Set("Content-Type", asset.contentType)
		_, _ = w.Write(asset.content)
	}
}
