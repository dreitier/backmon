package web

import (
	"net/http"
	"net/url"
	"strings"
)

// The API responds with HAL documents (https://datatracker.ietf.org/doc/html/draft-kelly-json-hal): each resource
// links to itself, to its parent and embeds its children, so that clients can navigate from /api without building URLs.
const (
	halContentType = "application/hal+json"

	apiRoot = "/api"

	// headerForwardedPrefix is set by reverse proxies which strip a path prefix before forwarding the request
	headerForwardedPrefix = "X-Forwarded-Prefix"

	relSelf     = "self"
	relUp       = "up"
	relDownload = "download"

	relDisks       = "disks"
	relDirectories = "directories"
	relFiles       = "files"
	relVariants    = "variants"
)

type link struct {
	Href string `json:"href"`
}

type links map[string]link

// embeddedResource is a child of the requested resource
type embeddedResource struct {
	Name  string `json:"name"`
	Links links  `json:"_links,omitempty"`
}

type embedded map[string][]embeddedResource

// rootResource is the entry point of the API and lists all disks
type rootResource struct {
	Links    links    `json:"_links"`
	Embedded embedded `json:"_embedded"`
}

type diskResource struct {
	Name string `json:"name"`
	// Quota is the quota of the disk in bytes; 0 if the disk has none
	Quota uint64 `json:"quota"`
	// Timezone is only present if the backup definitions set one
	Timezone string   `json:"timezone,omitempty"`
	Links    links    `json:"_links"`
	Embedded embedded `json:"_embedded"`
}

// namedResource is used for directories and files, which only have a name besides their links and children
type namedResource struct {
	Name     string   `json:"name"`
	Links    links    `json:"_links"`
	Embedded embedded `json:"_embedded"`
}

// apiHref returns the path of an API resource. Each segment is escaped, as names of local disks contain slashes.
func apiHref(segments ...string) string {
	href := apiRoot

	for _, segment := range segments {
		href += "/" + url.PathEscape(segment)
	}

	return href
}

// linkBuilder creates the links of a response. They are absolute paths as the client sees them: behind a reverse
// proxy which strips a path prefix, the prefix is put in front of each link.
type linkBuilder struct {
	prefix string
}

func newLinkBuilder(r *http.Request) linkBuilder {
	return linkBuilder{prefix: forwardedPrefix(r)}
}

func (b linkBuilder) to(segments ...string) link {
	return link{Href: b.prefix + apiHref(segments...)}
}

// embed returns the children with the given names; each one links to itself below the path of its parent
func (b linkBuilder) embed(names []string, parent ...string) []embeddedResource {
	children := make([]embeddedResource, 0, len(names))

	for _, name := range names {
		// the full slice expression keeps append from writing into the backing array of parent
		segments := append(parent[:len(parent):len(parent)], name)
		children = append(children, embeddedResource{Name: name, Links: links{relSelf: b.to(segments...)}})
	}

	return children
}

// forwardedPrefix returns the path prefix of the X-Forwarded-Prefix header without a trailing slash, or an empty
// string. The header can be set by any client, so only a plain absolute path is accepted: links and redirects must
// never leave the host of the request.
func forwardedPrefix(r *http.Request) string {
	// with several proxies in a row the first value is the prefix the client has used
	prefix, _, _ := strings.Cut(r.Header.Get(headerForwardedPrefix), ",")
	prefix = strings.TrimRight(strings.TrimSpace(prefix), "/")

	if !strings.HasPrefix(prefix, "/") || strings.HasPrefix(prefix, "//") {
		return ""
	}

	for _, c := range prefix {
		// a backslash is treated like a slash by browsers; ? and # would end the path
		if c <= ' ' || c == 0x7f || c == '\\' || c == '?' || c == '#' {
			return ""
		}
	}

	return prefix
}
