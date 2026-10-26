// Package icons resolves and serves the icon of every app, bookmark and badge
// (docs/adr/0008). This file is the grammar of an icon reference: the one
// string an annotation or the configuration names an icon with.
package icons

import (
	"errors"
	"net/url"
	"regexp"
	"slices"
	"strings"
)

// The bundled catalogues, by the prefix a reference names them with.
const (
	CatalogMDI             = "mdi"
	CatalogMaterialSymbols = "material-symbols"
	CatalogSimpleIcons     = "si"
	CatalogSelfhst         = "sh"
	CatalogDashboardIcons  = "di"
)

// catalogPrefixes maps every accepted prefix to its catalogue: the short
// prefix and the catalogue's long name.
var catalogPrefixes = map[string]string{
	CatalogMDI:             CatalogMDI,
	CatalogMaterialSymbols: CatalogMaterialSymbols,
	CatalogSimpleIcons:     CatalogSimpleIcons,
	"simple-icons":         CatalogSimpleIcons,
	CatalogSelfhst:         CatalogSelfhst,
	"selfhst":              CatalogSelfhst,
	CatalogDashboardIcons:  CatalogDashboardIcons,
	"dashboard-icons":      CatalogDashboardIcons,
}

// GlyphCatalogs are the catalogues of monochrome glyphs, which take the
// colour they are drawn in; the others are full-colour logos.
var GlyphCatalogs = []string{CatalogMDI, CatalogMaterialSymbols, CatalogSimpleIcons}

// IsGlyphCatalog reports whether catalog draws monochrome glyphs.
func IsGlyphCatalog(catalog string) bool {
	return slices.Contains(GlyphCatalogs, catalog)
}

// RefKind is what a reference names.
type RefKind int

// The kinds of reference, in the order the grammar tries them.
const (
	// RefRemote is a URL the backend fetches (docs/adr/0009).
	RefRemote RefKind = iota + 1
	// RefConfigMap is a key of a mounted icon ConfigMap.
	RefConfigMap
	// RefCatalog is an icon of a bundled catalogue.
	RefCatalog
)

// Ref is a parsed icon reference.
type Ref struct {
	Kind RefKind
	// URL is the remote icon's address, for RefRemote.
	URL string
	// Key is the ConfigMap key, for RefConfigMap.
	Key string
	// Catalog and Name name a catalogue's icon, for RefCatalog: the
	// catalogue's short prefix and the icon's name, both lower case.
	Catalog string
	Name    string
}

// The errors of ParseRef. None of them echoes the reference: it can be a
// URL with a token in its query.
var (
	ErrRefEmpty         = errors.New("is empty")
	ErrRefURL           = errors.New("must be an absolute http or https URL without a user or password")
	ErrRefConfigMapKey  = errors.New("must name a ConfigMap key after configmap: — letters, digits, '-', '_' and '.'")
	ErrRefUnknownPrefix = errors.New("names no catalogue: the prefixes are mdi, material-symbols, si (simple-icons), sh (selfhst) and di (dashboard-icons)")
	ErrRefName          = errors.New("must name an icon in lower case letters, digits and single '-', '_' or '.' between them")
)

// iconName is the form of a catalogue icon's name: every catalogue names its
// files in lower case, words joined by a dash.
var iconName = regexp.MustCompile(`^[a-z0-9]+([._-][a-z0-9]+)*$`)

// configMapKey is the form Kubernetes allows for a ConfigMap's key.
var configMapKey = regexp.MustCompile(`^[-._a-zA-Z0-9]+$`)

// maxNameLength bounds a catalogue icon's name and a ConfigMap key.
const maxNameLength = 253

// ParseRef parses an icon reference (docs/adr/0008 D2), in this order: a
// string containing // is a URL; configmap:<key> a ConfigMap's key;
// <prefix>:<name> a catalogue's icon; a bare name an icon of mdi, Hajimari's
// rule, so that a manifest migrates by replacing the annotation prefix.
// Surrounding space is dropped, a prefix and a name are read in lower case.
func ParseRef(raw string) (Ref, error) {
	s := strings.TrimSpace(raw)
	switch {
	case s == "":
		return Ref{}, ErrRefEmpty
	case strings.Contains(s, "//"):
		return parseURL(s)
	}
	prefix, name, found := strings.Cut(s, ":")
	if !found {
		return catalogRef(CatalogMDI, s)
	}
	if prefix == "configmap" {
		if len(name) > maxNameLength || !configMapKey.MatchString(name) || name == "." || name == ".." {
			return Ref{}, ErrRefConfigMapKey
		}
		return Ref{Kind: RefConfigMap, Key: name}, nil
	}
	catalog, known := catalogPrefixes[strings.ToLower(prefix)]
	if !known {
		return Ref{}, ErrRefUnknownPrefix
	}
	return catalogRef(catalog, name)
}

func catalogRef(catalog, name string) (Ref, error) {
	name = strings.ToLower(name)
	if len(name) > maxNameLength || !iconName.MatchString(name) {
		return Ref{}, ErrRefName
	}
	return Ref{Kind: RefCatalog, Catalog: catalog, Name: name}, nil
}

func parseURL(s string) (Ref, error) {
	u, err := url.Parse(s)
	if err != nil || (u.Scheme != "https" && u.Scheme != "http") || u.Hostname() == "" || u.User != nil {
		return Ref{}, ErrRefURL
	}
	return Ref{Kind: RefRemote, URL: u.String()}, nil
}

// String is the reference in its canonical form: the URL, configmap:<key>,
// or <catalogue>:<name> with the short prefix.
func (r Ref) String() string {
	switch r.Kind {
	case RefRemote:
		return r.URL
	case RefConfigMap:
		return "configmap:" + r.Key
	case RefCatalog:
		return r.Catalog + ":" + r.Name
	default:
		return ""
	}
}
