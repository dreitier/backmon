package web

import (
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"
)

func TestApiHref(t *testing.T) {
	cases := []struct {
		segments []string
		want     string
	}{
		{nil, "/api"},
		{[]string{"disk"}, "/api/disk"},
		{[]string{"/var/backups", "my dir", "100%"}, "/api/%2Fvar%2Fbackups/my%20dir/100%25"},
	}

	for _, tc := range cases {
		if got := apiHref(tc.segments...); got != tc.want {
			t.Errorf("apiHref(%q) = %q, want %q", tc.segments, got, tc.want)
		}
	}
}

func TestEmbed(t *testing.T) {
	parent := []string{"disk", "dir"}
	children := linkBuilder{}.embed([]string{"a", "b"}, parent...)

	if len(children) != 2 {
		t.Fatalf("children = %v, want two", children)
	}

	for i, want := range []string{"/api/disk/dir/a", "/api/disk/dir/b"} {
		if got := children[i].Links[relSelf].Href; got != want {
			t.Errorf("self of child %d = %q, want %q", i, got, want)
		}
	}
}

// Clients iterate over the embedded resources, so an empty list must be [] and not null.
func TestEmbedWithoutChildrenIsAnEmptyList(t *testing.T) {
	hrefs := linkBuilder{}

	b, err := json.Marshal(rootResource{Links: links{relSelf: hrefs.to()}, Embedded: embedded{relDisks: hrefs.embed(nil)}})
	if err != nil {
		t.Fatal(err)
	}

	if want := `{"_links":{"self":{"href":"/api"}},"_embedded":{"disks":[]}}`; string(b) != want {
		t.Errorf("JSON = %s, want %s", b, want)
	}
}

// Without a route for downloads a variant has no links at all.
func TestEmbeddedResourceWithoutLinks(t *testing.T) {
	b, err := json.Marshal(embeddedResource{Name: "latest"})
	if err != nil {
		t.Fatal(err)
	}

	if want := `{"name":"latest"}`; string(b) != want {
		t.Errorf("JSON = %s, want %s", b, want)
	}
}

func TestForwardedPrefix(t *testing.T) {
	cases := []struct {
		name   string
		header string
		want   string
	}{
		{"no header", "", ""},
		{"prefix", "/backmon", "/backmon"},
		{"nested prefix", "/tools/backmon", "/tools/backmon"},
		{"trailing slash", "/backmon/", "/backmon"},
		{"only a slash", "/", ""},
		{"surrounding spaces", " /backmon ", "/backmon"},
		{"several proxies", "/outer, /inner", "/outer"},
		{"relative path", "backmon", ""},
		{"other host", "//evil.example/backmon", ""},
		{"absolute URL", "https://evil.example/backmon", ""},
		{"backslash", "/\\evil.example", ""},
		{"query", "/backmon?x=1", ""},
		{"fragment", "/backmon#x", ""},
		{"whitespace", "/back mon", ""},
		{"control character", "/back\tmon", ""},
	}

	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			req := httptest.NewRequest(http.MethodGet, "/api", nil)
			if tc.header != "" {
				req.Header.Set(headerForwardedPrefix, tc.header)
			}

			if got := forwardedPrefix(req); got != tc.want {
				t.Errorf("forwardedPrefix(%q) = %q, want %q", tc.header, got, tc.want)
			}
		})
	}
}

func TestLinkBuilderPrependsThePrefix(t *testing.T) {
	hrefs := linkBuilder{prefix: "/backmon"}

	if got := hrefs.to("disk").Href; got != "/backmon/api/disk" {
		t.Errorf("link = %q, want /backmon/api/disk", got)
	}

	children := hrefs.embed([]string{"dir"}, "disk")
	if got := children[0].Links[relSelf].Href; got != "/backmon/api/disk/dir" {
		t.Errorf("self of the child = %q, want /backmon/api/disk/dir", got)
	}
}
