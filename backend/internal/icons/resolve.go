package icons

import (
	"context"
	"errors"
	"fmt"
	"log/slog"
	"regexp"
	"strings"
)

// PathPrefix is where the backend serves icons; every Icon's Src begins
// with it, so the browser never leaves the dashboard's origin for one.
const PathPrefix = "/api/v1/icons/"

// Icon is a resolved icon, as the API hands it out (docs/adr/0008 D3): the
// frontend never parses a reference.
type Icon struct {
	// Src is the icon's address on the dashboard's origin.
	Src string
	// SrcDark is the address of the variant the dark scheme shows, empty
	// when the icon is the same in both.
	SrcDark string
	// Tint marks a monochrome glyph, which the page draws as a mask in the
	// token colour; a logo is drawn as it is.
	Tint bool
}

// AutoMatchCatalogs maps the configuration's names of the logo catalogues
// (icons.autoMatch.catalogs) to their prefixes.
var AutoMatchCatalogs = map[string]string{"selfhst": CatalogSelfhst, "dashboard-icons": CatalogDashboardIcons}

// Options configure NewResolver.
type Options struct {
	Packs Packs
	// AutoMatch are the logo catalogues an app without an icon is looked up
	// in, in order, by their prefixes; empty switches auto-match off.
	AutoMatch []string
	// DefaultApp and DefaultBookmark are the references every item without
	// an icon of its own falls back to (icons.defaultAppIcon, …).
	DefaultApp, DefaultBookmark string
	// ConfigMaps is the folder the icon ConfigMaps are mounted in, one
	// folder each; a missing folder holds none.
	ConfigMaps string
	// Remote are the rules of the icons named by a URL.
	Remote RemoteOptions
	// Context bounds the downloads: the backend's lifetime. nil is none.
	Context context.Context
	// Logger receives what is left out and what failed; nil discards it.
	Logger *slog.Logger
	// fetcher replaces the remote fetch in the tests.
	fetcher fetcher
}

// Resolver turns references into icons, deciding every fallback: every app
// and every bookmark has an icon.
type Resolver struct {
	packs           Packs
	autoMatch       []string
	defaultApp      Icon
	defaultBookmark Icon
	blobs           *blobs
	configMaps      map[string]configMapIcon
	remote          *remote
}

// NewResolver reads the icon ConfigMaps and checks that both defaults
// resolve, which the start depends on: an item without an icon of its own
// needs them.
func NewResolver(o Options) (*Resolver, error) {
	if o.Logger == nil {
		o.Logger = slog.New(slog.DiscardHandler)
	}
	if o.Context == nil {
		o.Context = context.Background()
	}
	if o.fetcher == nil {
		o.fetcher = NewFetcher(o.Remote.Fetch)
	}
	r := &Resolver{packs: o.Packs, autoMatch: o.AutoMatch, blobs: newBlobs()}
	r.remote = newRemote(o.Context, o.Remote, o.fetcher, r.blobs, o.Logger)
	configMaps, err := loadConfigMaps(o.ConfigMaps, r.blobs, o.Logger)
	if err != nil {
		return nil, err
	}
	r.configMaps = configMaps
	var problems []string
	for _, d := range []struct {
		key, ref string
		into     *Icon
	}{
		{"icons.defaultAppIcon", o.DefaultApp, &r.defaultApp},
		{"icons.defaultBookmarkIcon", o.DefaultBookmark, &r.defaultBookmark},
	} {
		icon, err := r.resolve(d.ref)
		if err != nil {
			problems = append(problems, d.key+": "+err.Error())
			continue
		}
		*d.into = icon
	}
	if len(problems) > 0 {
		return nil, errors.New(strings.Join(problems, "; "))
	}
	return r, nil
}

// Register makes a reference known as it is read — from the configuration
// at start, from an Ingress's annotation when the discovery reads it — and
// tells why it will not resolve. A URL starts its download and is no
// problem while it runs; its failure is logged when it happens.
func (r *Resolver) Register(raw string, fromAnnotation bool) error {
	ref, err := ParseRef(raw)
	if err != nil {
		return err
	}
	if ref.Kind != RefRemote {
		_, err := r.resolve(raw)
		return err
	}
	if fromAnnotation && r.remote.opts.Enabled && !r.remote.opts.FromAnnotations {
		return ErrRemoteAnnotations
	}
	_, err = r.remote.icon(ref.URL)
	if errors.Is(err, errRemoteNotYet) || errors.Is(err, errRemoteFailed) {
		return nil
	}
	return err
}

// Settled is nil while no download runs. The backend turns ready once the
// icons known at start are downloaded or failed.
func (r *Resolver) Settled() error {
	return r.remote.settled()
}

// The errors of a reference that does not resolve.
var (
	ErrNotBundled  = errors.New("names no icon of the bundled catalogues")
	ErrNoConfigMap = errors.New("names a key no icon ConfigMap holds")
)

func (r *Resolver) resolve(raw string) (Icon, error) {
	ref, err := ParseRef(raw)
	if err != nil {
		return Icon{}, err
	}
	switch ref.Kind {
	case RefCatalog:
		icon, ok := r.catalog(ref.Catalog, ref.Name)
		if !ok {
			return Icon{}, ErrNotBundled
		}
		return icon, nil
	case RefConfigMap:
		c, ok := r.configMaps[ref.Key]
		if !ok {
			return Icon{}, ErrNoConfigMap
		}
		return Icon{Src: c.src}, nil
	default:
		return r.remote.icon(ref.URL)
	}
}

// catalog resolves an icon of a pack.
func (r *Resolver) catalog(catalog, name string) (Icon, bool) {
	p, ok := r.packs[catalog]
	if !ok {
		return Icon{}, false
	}
	file, ok := p.Lookup(name)
	if !ok {
		return Icon{}, false
	}
	icon := Icon{Src: catalogPath(p, file), Tint: IsGlyphCatalog(catalog)}
	if dark, ok := p.Dark[file]; ok {
		icon.SrcDark = catalogPath(p, dark)
	}
	return icon, true
}

func catalogPath(p *Pack, file string) string {
	return PathPrefix + p.Catalog + "@" + p.Version + "/" + file + ".svg"
}

// App is an app's icon (docs/adr/0008 D3): its own reference, else the logo
// of the first of its names that a logo catalogue knows, else the default.
func (r *Resolver) App(raw string, names []string) Icon {
	if raw != "" {
		if icon, err := r.resolve(raw); err == nil {
			return icon
		}
	}
	if icon, ok := r.Match(names); ok {
		return icon
	}
	return r.defaultApp
}

// Bookmark is a bookmark's icon: its own reference, else the default.
func (r *Resolver) Bookmark(raw string) Icon {
	if raw != "" {
		if icon, err := r.resolve(raw); err == nil {
			return icon
		}
	}
	return r.defaultBookmark
}

// Match looks the names up in the auto-match catalogues: each name in turn,
// the more specific first, in each catalogue in the configured order; a
// name's normalised form before its compact one. Exact matches only.
func (r *Resolver) Match(names []string) (Icon, bool) {
	for _, form := range []func(string) string{NormalizeName, CompactName} {
		for _, n := range names {
			key := form(n)
			if key == "" {
				continue
			}
			for _, catalog := range r.autoMatch {
				p, ok := r.packs[catalog]
				if !ok {
					continue
				}
				if file, ok := p.Match[key]; ok {
					return r.catalog(catalog, file)
				}
			}
		}
	}
	return Icon{}, false
}

// iconFile is the form of a served catalogue icon's file name.
var iconFile = regexp.MustCompile(`^([a-z0-9]+(?:[._-][a-z0-9]+)*)\.svg$`)

// ErrNoIcon answers a request for an icon that is not served: an unknown
// pack, another version, an unknown name.
var ErrNoIcon = errors.New("no such icon")

// CatalogIcon is the SVG of a served catalogue icon: pack is
// <catalogue>@<version>, file <name>.svg, as Src names them. Another
// version than the bundled one is no icon: the addresses are immutable.
func (r *Resolver) CatalogIcon(pack, file string) ([]byte, error) {
	catalog, version, ok := strings.Cut(pack, "@")
	m := iconFile.FindStringSubmatch(file)
	if !ok || m == nil {
		return nil, ErrNoIcon
	}
	p, ok := r.packs[catalog]
	if !ok || p.Version != version {
		return nil, ErrNoIcon
	}
	if _, ok := p.files[m[1]]; !ok {
		return nil, ErrNoIcon
	}
	data, err := p.Read(m[1])
	if err != nil {
		return nil, fmt.Errorf("read %s from %s: %w", m[1], pack, err)
	}
	return data, nil
}
