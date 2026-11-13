package web

import (
	"encoding/json"
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
	children := embed([]string{"a", "b"}, parent...)

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
	b, err := json.Marshal(rootResource{Links: links{relSelf: linkTo()}, Embedded: embedded{relDisks: embed(nil)}})
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
