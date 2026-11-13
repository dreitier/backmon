package web

import (
	"net/url"
)

// The API responds with HAL documents (https://datatracker.ietf.org/doc/html/draft-kelly-json-hal): each resource
// links to itself, to its parent and embeds its children, so that clients can navigate from /api without building URLs.
const (
	halContentType = "application/hal+json"

	apiRoot = "/api"

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

func linkTo(segments ...string) link {
	return link{Href: apiHref(segments...)}
}

// embed returns the children with the given names; each one links to itself below the path of its parent
func embed(names []string, parent ...string) []embeddedResource {
	children := make([]embeddedResource, 0, len(names))

	for _, name := range names {
		// the full slice expression keeps append from writing into the backing array of parent
		segments := append(parent[:len(parent):len(parent)], name)
		children = append(children, embeddedResource{Name: name, Links: links{relSelf: linkTo(segments...)}})
	}

	return children
}
