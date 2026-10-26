package icons

import (
	"path/filepath"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

const logo = `<svg xmlns="http://www.w3.org/2000/svg" viewBox="0 0 24 24"><path fill="#f46800" d="M0 0h1v1z"/></svg>`

// testPacks are small packs of every catalogue: what the resolution needs to
// show each rule.
func testPacks(t *testing.T) Packs {
	t.Helper()
	dir := t.TempDir()
	for _, p := range []struct {
		m     Manifest
		files []string
	}{
		{Manifest{Catalog: CatalogMDI, Version: "1.2.3", Aliases: map[string]string{"www": "web"}}, []string{"web", "home-automation", "pail"}},
		{Manifest{Catalog: CatalogMaterialSymbols, Version: "1.2.96"}, []string{"apps", "bookmark", "router"}},
		{Manifest{Catalog: CatalogSimpleIcons, Version: "1.2.101"}, []string{"grafana", "kubernetes"}},
		{Manifest{Catalog: CatalogSelfhst, Version: "1d65d45a42e6", Dark: map[string]string{"github": "github-light"},
			Match: map[string]string{"grafana": "grafana", "github": "github", "teddycloud": "teddycloud", "home-assistant": "home-assistant"}},
			[]string{"grafana", "github", "github-light", "teddycloud", "home-assistant"}},
		{Manifest{Catalog: CatalogDashboardIcons, Version: "d3b9234dd56f",
			Match: map[string]string{"grafana": "grafana", "alertmanager": "alertmanager", "wikijs": "wikijs", "prometheus": "prometheus"}},
			[]string{"grafana", "alertmanager", "wikijs", "prometheus"}},
	} {
		files := map[string][]byte{}
		for _, f := range p.files {
			files[f] = []byte(logo)
		}
		require.NoError(t, WritePack(filepath.Join(dir, p.m.Catalog+".zip"), p.m, files))
	}
	packs, err := OpenPacks(dir)
	require.NoError(t, err)
	t.Cleanup(packs.Close)
	return packs
}

func resolver(t *testing.T) *Resolver {
	t.Helper()
	r, err := NewResolver(Options{
		Packs: testPacks(t), AutoMatch: []string{CatalogSelfhst, CatalogDashboardIcons},
		DefaultApp: "material-symbols:apps", DefaultBookmark: "material-symbols:bookmark",
	})
	require.NoError(t, err)
	return r
}

var (
	defaultApp      = Icon{Src: "/api/v1/icons/material-symbols@1.2.96/apps.svg", Tint: true}
	defaultBookmark = Icon{Src: "/api/v1/icons/material-symbols@1.2.96/bookmark.svg", Tint: true}
)

func TestAnAppsOwnIcon(t *testing.T) {
	r := resolver(t)
	for ref, want := range map[string]Icon{
		"web":                     {Src: "/api/v1/icons/mdi@1.2.3/web.svg", Tint: true},
		"mdi:www":                 {Src: "/api/v1/icons/mdi@1.2.3/web.svg", Tint: true},
		"material-symbols:router": {Src: "/api/v1/icons/material-symbols@1.2.96/router.svg", Tint: true},
		"si:grafana":              {Src: "/api/v1/icons/si@1.2.101/grafana.svg", Tint: true},
		"sh:grafana":              {Src: "/api/v1/icons/sh@1d65d45a42e6/grafana.svg"},
		"selfhst:github": {
			Src: "/api/v1/icons/sh@1d65d45a42e6/github.svg", SrcDark: "/api/v1/icons/sh@1d65d45a42e6/github-light.svg",
		},
		"dashboard-icons:alertmanager": {Src: "/api/v1/icons/di@d3b9234dd56f/alertmanager.svg"},
	} {
		assert.Equal(t, want, r.App(ref, []string{"jellyfin"}), ref)
		assert.NoError(t, r.Register(ref, true), ref)
	}
}

// Every app has an icon: an own icon that does not resolve continues with
// auto-match, then the default.
func TestEveryFallback(t *testing.T) {
	r := resolver(t)
	for ref, why := range map[string]error{
		"mdi:nothing": ErrNotBundled,
		"fa:home":     ErrRefUnknownPrefix,
		"mdi:":        ErrRefName,
	} {
		assert.Equal(t, Icon{Src: "/api/v1/icons/sh@1d65d45a42e6/grafana.svg"}, r.App(ref, []string{"Grafana"}), ref)
		assert.Equal(t, defaultApp, r.App(ref, []string{"jellyfin"}), ref)
		assert.Equal(t, defaultBookmark, r.Bookmark(ref), ref)
		assert.ErrorIs(t, r.Register(ref, true), why, ref)
	}
	assert.Equal(t, defaultApp, r.App("", nil))
	assert.Equal(t, defaultBookmark, r.Bookmark(""))
	assert.Equal(t, Icon{Src: "/api/v1/icons/si@1.2.101/kubernetes.svg", Tint: true}, r.Bookmark("si:kubernetes"))
}

func TestAutoMatch(t *testing.T) {
	r := resolver(t)
	for name, tc := range map[string]struct {
		names []string
		want  string
	}{
		"the first catalogue first":         {[]string{"Grafana"}, "/api/v1/icons/sh@1d65d45a42e6/grafana.svg"},
		"the second where the first lacks":  {[]string{"Alertmanager"}, "/api/v1/icons/di@d3b9234dd56f/alertmanager.svg"},
		"the more specific name first":      {[]string{"Alertmanager", "grafana"}, "/api/v1/icons/di@d3b9234dd56f/alertmanager.svg"},
		"a later name when the first fails": {[]string{"my-monitoring", "prometheus"}, "/api/v1/icons/di@d3b9234dd56f/prometheus.svg"},
		"punctuation as a dash":             {[]string{"Home_Assistant"}, "/api/v1/icons/sh@1d65d45a42e6/home-assistant.svg"},
		"the compact form last":             {[]string{"Wiki.js"}, "/api/v1/icons/di@d3b9234dd56f/wikijs.svg"},
		"case aside":                        {[]string{"TeddyCloud"}, "/api/v1/icons/sh@1d65d45a42e6/teddycloud.svg"},
	} {
		got := r.App("", tc.names)
		assert.Equal(t, tc.want, got.Src, name)
		assert.False(t, got.Tint, name)
	}
	assert.Equal(t, defaultApp, r.App("", []string{"graf", "grafana-dashboards", ""}), "exact matches only")

	off, err := NewResolver(Options{Packs: r.packs, DefaultApp: "mdi:web", DefaultBookmark: "mdi:web"})
	require.NoError(t, err)
	assert.Equal(t, "/api/v1/icons/mdi@1.2.3/web.svg", off.App("", []string{"grafana"}).Src, "auto-match off")

	order, err := NewResolver(Options{Packs: r.packs, AutoMatch: []string{CatalogDashboardIcons, CatalogSelfhst}, DefaultApp: "mdi:web", DefaultBookmark: "mdi:web"})
	require.NoError(t, err)
	assert.Equal(t, "/api/v1/icons/di@d3b9234dd56f/grafana.svg", order.App("", []string{"grafana"}).Src, "the configured order")
}

// The owner's manifests name 27 MDI glyphs, all bare (docs/planning §2);
// three of them stand for the rest here, the full list is the pack test of
// the image's packs.
func TestBareNamesAreMDI(t *testing.T) {
	r := resolver(t)
	for _, name := range []string{"web", "home-automation", "pail"} {
		assert.Equal(t, "/api/v1/icons/mdi@1.2.3/"+name+".svg", r.App(name, nil).Src, name)
	}
}

func TestTheDefaultsMustResolve(t *testing.T) {
	packs := testPacks(t)
	_, err := NewResolver(Options{Packs: packs, DefaultApp: "mdi:nothing", DefaultBookmark: "fa:x"})
	require.Error(t, err)
	assert.Equal(t, "icons.defaultAppIcon: names no icon of the bundled catalogues; icons.defaultBookmarkIcon: "+ErrRefUnknownPrefix.Error(), err.Error())
	_, err = NewResolver(Options{Packs: Packs{}, DefaultApp: "material-symbols:apps", DefaultBookmark: "material-symbols:bookmark"})
	assert.Error(t, err, "without packs nothing resolves")
}

func TestCatalogIcon(t *testing.T) {
	r := resolver(t)
	data, err := r.CatalogIcon("mdi@1.2.3", "web.svg")
	require.NoError(t, err)
	assert.Equal(t, logo, string(data))
	for _, tc := range [][2]string{
		{"mdi@1.2.2", "web.svg"},
		{"mdi", "web.svg"},
		{"fa@1", "web.svg"},
		{"mdi@1.2.3", "www.svg"},
		{"mdi@1.2.3", "web.png"},
		{"mdi@1.2.3", "../web.svg"},
		{"mdi@1.2.3", "nothing.svg"},
	} {
		_, err := r.CatalogIcon(tc[0], tc[1])
		assert.ErrorIs(t, err, ErrNoIcon, "%v", tc)
	}
}
