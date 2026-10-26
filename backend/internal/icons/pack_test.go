package icons

import (
	"os"
	"path/filepath"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

const glyph = `<svg xmlns="http://www.w3.org/2000/svg" width="24" height="24" viewBox="0 0 24 24"><path fill="currentColor" d="M0 0h24v24H0z"/></svg>`

func TestAPackRoundTrips(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, "mdi.zip")
	require.NoError(t, WritePack(path, Manifest{
		Catalog: CatalogMDI, Name: "Material Design Icons", Version: "1.2.3", License: "Apache-2.0",
		Source: "@iconify-json/mdi@1.2.3", Skipped: 1, Aliases: map[string]string{"www": "web"},
	}, map[string][]byte{"web": []byte(glyph), "home": []byte(glyph)}))

	p, err := OpenPack(path)
	require.NoError(t, err)
	defer func() { require.NoError(t, p.Close()) }()
	assert.Equal(t, 2, p.Count)
	assert.Equal(t, "1.2.3", p.Version)
	assert.Equal(t, []string{"home", "web"}, p.Names())
	name, ok := p.Lookup("www")
	assert.True(t, ok)
	assert.Equal(t, "web", name, "an alias leads to its icon")
	_, ok = p.Lookup("nothing")
	assert.False(t, ok)
	data, err := p.Read("web")
	require.NoError(t, err)
	assert.Equal(t, glyph, string(data))
	_, err = p.Read("nothing")
	assert.Error(t, err)

	again := filepath.Join(dir, "again.zip")
	require.NoError(t, WritePack(again, p.Manifest, map[string][]byte{"web": []byte(glyph), "home": []byte(glyph)}))
	a, err := os.ReadFile(path)
	require.NoError(t, err)
	b, err := os.ReadFile(again)
	require.NoError(t, err)
	assert.Equal(t, a, b, "a pack built twice is the same file")
}

func TestOpenPacks(t *testing.T) {
	dir := t.TempDir()
	require.NoError(t, WritePack(filepath.Join(dir, "a.zip"), Manifest{Catalog: CatalogMDI, Version: "1"}, map[string][]byte{"web": []byte(glyph)}))
	require.NoError(t, WritePack(filepath.Join(dir, "b.zip"), Manifest{Catalog: CatalogSelfhst, Version: "abc"}, map[string][]byte{"grafana": []byte(glyph)}))
	require.NoError(t, os.WriteFile(filepath.Join(dir, "README"), []byte("not a pack"), 0o600))
	packs, err := OpenPacks(dir)
	require.NoError(t, err)
	defer packs.Close()
	assert.Len(t, packs, 2)
	assert.Equal(t, "abc", packs[CatalogSelfhst].Version)

	require.NoError(t, WritePack(filepath.Join(dir, "c.zip"), Manifest{Catalog: CatalogMDI, Version: "2"}, nil))
	_, err = OpenPacks(dir)
	assert.ErrorContains(t, err, "two icon packs of mdi")

	empty, err := OpenPacks(t.TempDir())
	require.NoError(t, err)
	assert.Empty(t, empty)
}

func TestABrokenPackIsRefused(t *testing.T) {
	dir := t.TempDir()
	for name, m := range map[string]Manifest{
		"unknown.zip":   {Catalog: "fa", Version: "1"},
		"alias.zip":     {Catalog: "simple-icons", Version: "1"},
		"noversion.zip": {Catalog: CatalogMDI},
		"slash.zip":     {Catalog: CatalogMDI, Version: "1/../x"},
	} {
		path := filepath.Join(dir, name)
		require.NoError(t, WritePack(path, m, nil))
		_, err := OpenPack(path)
		assert.Error(t, err, name)
	}
	notZip := filepath.Join(dir, "x.zip")
	require.NoError(t, os.WriteFile(notZip, []byte("nope"), 0o600))
	_, err := OpenPack(notZip)
	assert.Error(t, err)
	_, err = OpenPacks(dir)
	assert.Error(t, err, "one broken pack refuses the start")
}

func TestNormalizeName(t *testing.T) {
	for in, want := range map[string]string{
		"Home Assistant": "home-assistant",
		"home_assistant": "home-assistant",
		"Wiki.js":        "wiki-js",
		"pgAdmin":        "pgadmin",
		"Stirling-PDF":   "stirling-pdf",
		"  --SearXNG-- ": "searxng",
		"Grafana 8":      "grafana-8",
		"":               "",
	} {
		assert.Equal(t, want, NormalizeName(in), in)
	}
	assert.Equal(t, "wikijs", CompactName("Wiki.js"))
	assert.Equal(t, "homeassistant", CompactName("Home Assistant"))
}
