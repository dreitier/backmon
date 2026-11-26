package web

import (
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
)

func requestWithHeaders(path string, headers map[string]string) *httptest.ResponseRecorder {
	req := httptest.NewRequest(http.MethodGet, path, nil)

	for key, value := range headers {
		req.Header.Set(key, value)
	}

	rec := httptest.NewRecorder()
	GetInstance().endpointsRouter.ServeHTTP(rec, req)

	return rec
}

const browserAccept = "text/html,application/xhtml+xml,application/xml;q=0.9,*/*;q=0.8"

func TestRouter_rootServesTheWebUIToBrowsers(t *testing.T) {
	rec := requestWithHeaders("/", map[string]string{"Accept": browserAccept})

	if rec.Code != http.StatusOK {
		t.Fatalf("status = %d, want %d", rec.Code, http.StatusOK)
	}

	if contentType := rec.Header().Get("Content-Type"); contentType != "text/html; charset=utf-8" {
		t.Errorf("Content-Type = %q, want text/html", contentType)
	}

	body := rec.Body.String()
	for _, want := range []string{`<base href="/">`, `src="ui/app.js"`, `href="ui/app.css"`} {
		if !strings.Contains(body, want) {
			t.Errorf("page does not contain %q", want)
		}
	}

	if csp := rec.Header().Get("Content-Security-Policy"); !strings.Contains(csp, "script-src 'self'") {
		t.Errorf("Content-Security-Policy = %q, want only the UI's own scripts", csp)
	}

	// the fonts of the UI are embedded, the stylesheet may only load them from the same origin
	if csp := rec.Header().Get("Content-Security-Policy"); !strings.Contains(csp, "font-src 'self'") {
		t.Errorf("Content-Security-Policy = %q, want the UI's own fonts", csp)
	}
}

// The page is not protected by Basic Auth: it contains no data, the browser asks for the credentials once the UI
// requests /api.
func TestRouter_webUIRequiresNoAuthentication(t *testing.T) {
	for _, path := range []string{"/", "/ui/app.js", "/ui/app.css"} {
		if rec := requestWithHeaders(path, map[string]string{"Accept": browserAccept}); rec.Code != http.StatusOK {
			t.Errorf("GET %s: status = %d, want %d", path, rec.Code, http.StatusOK)
		}
	}
}

func TestRouter_rootRedirectsApiClientsToApi(t *testing.T) {
	for _, accept := range []string{"", "*/*", "application/hal+json", "application/json, text/plain"} {
		rec := requestWithHeaders("/", map[string]string{"Accept": accept})

		if rec.Code != http.StatusMovedPermanently || rec.Header().Get("Location") != "/api" {
			t.Errorf("Accept %q: status = %d, Location = %q, want a redirect to /api", accept, rec.Code, rec.Header().Get("Location"))
		}
	}
}

func TestRouter_webUIUsesTheForwardedPrefix(t *testing.T) {
	rec := requestWithHeaders("/", map[string]string{"Accept": browserAccept, "X-Forwarded-Prefix": "/backmon/"})

	if !strings.Contains(rec.Body.String(), `<base href="/backmon/">`) {
		t.Errorf("page does not use the prefix /backmon/ as base: %s", rec.Body.String())
	}

	// a prefix of another host is ignored like for the links of the API
	rec = requestWithHeaders("/", map[string]string{"Accept": browserAccept, "X-Forwarded-Prefix": "//evil.example"})

	if !strings.Contains(rec.Body.String(), `<base href="/">`) {
		t.Errorf("page uses the prefix of another host: %s", rec.Body.String())
	}
}

func TestRouter_servesWebUIAssets(t *testing.T) {
	tests := []struct {
		path        string
		contentType string
	}{
		{"/ui/app.js", "text/javascript"},
		{"/ui/app.css", "text/css"},
		{"/ui/fonts/work-sans-latin-wght-normal.woff2", "font/woff2"},
		{"/ui/fonts/jetbrains-mono-latin-wght-normal.woff2", "font/woff2"},
	}

	for _, tc := range tests {
		rec := requestWithHeaders(tc.path, nil)

		if rec.Code != http.StatusOK {
			t.Errorf("GET %s: status = %d, want %d", tc.path, rec.Code, http.StatusOK)
			continue
		}

		if contentType := rec.Header().Get("Content-Type"); !strings.HasPrefix(contentType, tc.contentType) {
			t.Errorf("GET %s: Content-Type = %q, want %s", tc.path, contentType, tc.contentType)
		}

		if rec.Header().Get("X-Content-Type-Options") != "nosniff" {
			t.Errorf("GET %s: X-Content-Type-Options is missing", tc.path)
		}
	}
}

func TestRouter_webUIAssetsAreNotListed(t *testing.T) {
	for _, path := range []string{"/ui/", "/ui/index.html", "/ui/missing.js", "/ui/../config.yaml", "/ui/%2e%2e/config.yaml"} {
		if rec := requestWithHeaders(path, nil); rec.Code != http.StatusNotFound && rec.Code != http.StatusMovedPermanently {
			t.Errorf("GET %s: status = %d, want %d", path, rec.Code, http.StatusNotFound)
		} else if strings.Contains(rec.Body.String(), "{{") || strings.Contains(rec.Body.String(), "basic_auth") {
			t.Errorf("GET %s: leaks %q", path, rec.Body.String())
		}
	}
}

func TestAcceptsHTML(t *testing.T) {
	tests := map[string]bool{
		"":                          false,
		"*/*":                       false,
		"application/hal+json":      false,
		"text/html":                 true,
		"TEXT/HTML; q=0.9":          true,
		browserAccept:               true,
		"application/xhtml+xml":     false,
		"text/plain, text/html;q=1": true,
	}

	for accept, want := range tests {
		req := httptest.NewRequest(http.MethodGet, "/", nil)
		req.Header.Set("Accept", accept)

		if got := acceptsHTML(req); got != want {
			t.Errorf("acceptsHTML(%q) = %v, want %v", accept, got, want)
		}
	}
}

// The webserver keeps the path encoded: names of local disks contain slashes, which the API escapes in its links.
func TestServer_handlerKeepsEscapedSlashes(t *testing.T) {
	req := httptest.NewRequest(http.MethodGet, apiPath(testDiskName, "my-backups"), nil)
	req.SetBasicAuth(testUsername, testPassword)

	rec := httptest.NewRecorder()
	newHandler().ServeHTTP(rec, req)

	if rec.Code != http.StatusOK {
		t.Fatalf("GET %s: status = %d, Location = %q, want %d", req.URL.Path, rec.Code, rec.Header().Get("Location"), http.StatusOK)
	}

	if !strings.Contains(rec.Body.String(), `"name":"my-backups"`) {
		t.Errorf("body = %q, want the directory my-backups", rec.Body.String())
	}
}
