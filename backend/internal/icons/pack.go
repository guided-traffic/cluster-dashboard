package icons

import (
	"archive/zip"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"path/filepath"
	"regexp"
	"slices"
	"strings"
)

// An icon pack is one catalogue as the backend image carries it
// (docs/adr/0008 D1): a zip of SVG files under icons/ and a manifest.json —
// a few files on disk instead of tens of thousands, random access through
// the zip's index, memory for the index only. backend/tools/iconpack builds
// the packs at image build; the backend opens them at start.

// ManifestFile and IconDir are the pack's layout.
const (
	ManifestFile = "manifest.json"
	IconDir      = "icons/"
)

// Manifest describes a pack.
type Manifest struct {
	// Catalog is the catalogue's short prefix (CatalogMDI, …).
	Catalog string `json:"catalog"`
	// Name is the catalogue's name, Version the pinned release or the
	// pinned commit's first twelve hex digits, which the icon URLs carry.
	Name    string `json:"name"`
	Version string `json:"version"`
	// License is the catalogue's SPDX identifier.
	License string `json:"license"`
	// Source names what the pack was built from: an npm package at its
	// version, or a repository at its commit.
	Source string `json:"source"`
	// Count is the number of icon files; Skipped those the content check
	// refused.
	Count   int `json:"count"`
	Skipped int `json:"skipped"`
	// Aliases map another name of an icon to its file's name.
	Aliases map[string]string `json:"aliases,omitempty"`
	// Dark maps a logo that is dark itself to its variant for dark
	// backgrounds, which the dark scheme shows instead.
	Dark map[string]string `json:"dark,omitempty"`
	// Match maps an app name, normalised (NormalizeName) and compact
	// (CompactName), to the logo of that app, for auto-match: the icons'
	// names and their catalogue's names for them.
	Match map[string]string `json:"match,omitempty"`
}

// Pack is an open icon pack.
type Pack struct {
	Manifest
	zip   *zip.ReadCloser
	files map[string]*zip.File
}

// versionPattern bounds a pack's version: it is part of every icon URL.
var versionPattern = regexp.MustCompile(`^[0-9A-Za-z][0-9A-Za-z.+-]{0,63}$`)

// OpenPack opens a pack file and reads its manifest and index.
func OpenPack(path string) (*Pack, error) {
	z, err := zip.OpenReader(path)
	if err != nil {
		return nil, fmt.Errorf("open the icon pack %s: %w", filepath.Base(path), err)
	}
	p := &Pack{zip: z, files: map[string]*zip.File{}}
	if err := p.load(); err != nil {
		_ = z.Close()
		return nil, fmt.Errorf("the icon pack %s: %w", filepath.Base(path), err)
	}
	return p, nil
}

func (p *Pack) load() error {
	var manifest *zip.File
	for _, f := range p.zip.File {
		switch {
		case f.Name == ManifestFile:
			manifest = f
		case strings.HasPrefix(f.Name, IconDir) && strings.HasSuffix(f.Name, ".svg"):
			p.files[strings.TrimSuffix(strings.TrimPrefix(f.Name, IconDir), ".svg")] = f
		}
	}
	if manifest == nil {
		return errors.New("no manifest")
	}
	data, err := readZipFile(manifest, 1<<24)
	if err != nil {
		return err
	}
	if err := json.Unmarshal(data, &p.Manifest); err != nil {
		return fmt.Errorf("the manifest does not parse: %w", err)
	}
	if _, known := catalogPrefixes[p.Catalog]; !known || catalogPrefixes[p.Catalog] != p.Catalog {
		return fmt.Errorf("the manifest names no catalogue Dash knows (%q)", p.Catalog)
	}
	if !versionPattern.MatchString(p.Version) {
		return errors.New("the manifest's version is not a version")
	}
	return nil
}

// Close releases the pack's file.
func (p *Pack) Close() error {
	return p.zip.Close()
}

// Lookup returns the file name of an icon, following an alias; false when
// the pack holds no icon of that name.
func (p *Pack) Lookup(name string) (string, bool) {
	if target, ok := p.Aliases[name]; ok {
		name = target
	}
	_, ok := p.files[name]
	return name, ok
}

// Read returns an icon's SVG by its file name.
func (p *Pack) Read(name string) ([]byte, error) {
	f, ok := p.files[name]
	if !ok {
		return nil, fmt.Errorf("no icon %s in %s", name, p.Catalog)
	}
	return readZipFile(f, MaxSVG)
}

// Names are the pack's icon file names, sorted.
func (p *Pack) Names() []string {
	names := make([]string, 0, len(p.files))
	for n := range p.files {
		names = append(names, n)
	}
	slices.Sort(names)
	return names
}

func readZipFile(f *zip.File, limit int64) ([]byte, error) {
	rc, err := f.Open()
	if err != nil {
		return nil, err
	}
	defer func() { _ = rc.Close() }()
	data, err := io.ReadAll(io.LimitReader(rc, limit+1))
	if err != nil {
		return nil, err
	}
	if int64(len(data)) > limit {
		return nil, fmt.Errorf("%s is larger than %d bytes", f.Name, limit)
	}
	return data, nil
}

// Packs are the open packs by catalogue.
type Packs map[string]*Pack

// OpenPacks opens every *.zip of dir. A catalogue in two files refuses the
// start; a catalogue without a pack has no icons, and every reference to it
// falls back.
func OpenPacks(dir string) (Packs, error) {
	paths, err := filepath.Glob(filepath.Join(dir, "*.zip"))
	if err != nil {
		return nil, err
	}
	packs := Packs{}
	for _, path := range paths {
		p, err := OpenPack(path)
		if err != nil {
			packs.Close()
			return nil, err
		}
		if _, twice := packs[p.Catalog]; twice {
			_ = p.Close()
			packs.Close()
			return nil, fmt.Errorf("two icon packs of %s in %s", p.Catalog, dir)
		}
		packs[p.Catalog] = p
	}
	return packs, nil
}

// Close closes every pack.
func (ps Packs) Close() {
	for _, p := range ps {
		_ = p.Close()
	}
}

// nonAlnum matches what NormalizeName turns into a dash.
var nonAlnum = regexp.MustCompile(`[^a-z0-9]+`)

// CompactName is the second form auto-match compares names in, where
// NormalizeName's found nothing: NormalizeName without its dashes —
// "Wiki.js" "wikijs", the name Dashboard Icons gives that logo.
func CompactName(s string) string {
	return strings.ReplaceAll(NormalizeName(s), "-", "")
}

// NormalizeName is the form auto-match compares names in: lower case, every
// run of other characters one '-', none at either end — "Home Assistant"
// and "home_assistant" both "home-assistant", "Wiki.js" "wiki-js".
func NormalizeName(s string) string {
	return strings.Trim(nonAlnum.ReplaceAllString(strings.ToLower(s), "-"), "-")
}
