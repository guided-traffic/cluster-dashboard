package main

import (
	"bytes"
	"encoding/json"
	"errors"
	"fmt"
	"os"
	"regexp"
	"slices"
	"strings"

	"go.yaml.in/yaml/v3"

	"github.com/guided-traffic/dash/backend/internal/icons"
)

// glyphSource is an Iconify set pinned in package-lock.json.
type glyphSource struct {
	catalog   string
	pkg       string
	version   string
	resolved  string
	integrity string
	license   string
}

// iconifyPackages are the Iconify sets Dash bundles, by npm package, and the
// catalogue each becomes.
var iconifyPackages = map[string]string{
	"@iconify-json/mdi":              icons.CatalogMDI,
	"@iconify-json/material-symbols": icons.CatalogMaterialSymbols,
	"@iconify-json/simple-icons":     icons.CatalogSimpleIcons,
}

// npmRegistry is the only host a tarball is downloaded from.
const npmRegistry = "https://registry.npmjs.org/"

type lockFile struct {
	Packages map[string]struct {
		Version   string `json:"version"`
		Resolved  string `json:"resolved"`
		Integrity string `json:"integrity"`
		License   string `json:"license"`
	} `json:"packages"`
}

// readLock reads the pins of the Iconify sets, every one of iconifyPackages,
// from npm's lock file.
func readLock(path string) ([]glyphSource, error) {
	data, err := os.ReadFile(path) // #nosec G304 -- the pins' path is the builder's argument
	if err != nil {
		return nil, err
	}
	var lock lockFile
	if err := json.Unmarshal(data, &lock); err != nil {
		return nil, fmt.Errorf("%s does not parse: %w", path, err)
	}
	pkgs := make([]string, 0, len(iconifyPackages))
	for pkg := range iconifyPackages {
		pkgs = append(pkgs, pkg)
	}
	slices.Sort(pkgs)
	out := make([]glyphSource, 0, len(pkgs))
	for _, pkg := range pkgs {
		p, ok := lock.Packages["node_modules/"+pkg]
		switch {
		case !ok:
			return nil, fmt.Errorf("%s pins no %s", path, pkg)
		case !strings.HasPrefix(p.Resolved, npmRegistry):
			return nil, fmt.Errorf("%s resolves %s outside %s", path, pkg, npmRegistry)
		case !strings.HasPrefix(p.Integrity, "sha512-"):
			return nil, fmt.Errorf("%s pins %s without a sha512 integrity", path, pkg)
		}
		out = append(out, glyphSource{
			catalog: iconifyPackages[pkg], pkg: pkg, version: p.Version,
			resolved: p.Resolved, integrity: p.Integrity, license: p.License,
		})
	}
	return out, nil
}

// logoSource is a logo catalogue pinned in sources.yaml: a commit of its
// repository and the SHA-256 of the files the pack is built from.
type logoSource struct {
	Catalog    string `yaml:"-"`
	Name       string `yaml:"name"`
	Repository string `yaml:"repository"`
	Commit     string `yaml:"commit"`
	SHA256     string `yaml:"sha256"`
	Index      string `yaml:"index"`
	License    string `yaml:"license"`
}

// logoCatalogs are the keys of sources.yaml and the catalogue each becomes.
var logoCatalogs = map[string]string{"selfhst": icons.CatalogSelfhst, "dashboard-icons": icons.CatalogDashboardIcons}

var (
	commitPattern     = regexp.MustCompile(`^[0-9a-f]{40}$`)
	sumPattern        = regexp.MustCompile(`^[0-9a-f]{64}$`)
	repositoryPattern = regexp.MustCompile(`^https://github\.com/[A-Za-z0-9_.-]+/[A-Za-z0-9_.-]+$`)
	indexPattern      = regexp.MustCompile(`^[a-z-]+\.json$`)
)

// readSources reads the logo catalogues' pins, every one of logoCatalogs.
// Without withSums a pin may lack its SHA-256: make icon-pins computes it.
func readSources(path string, withSums bool) ([]logoSource, error) {
	data, err := os.ReadFile(path) // #nosec G304 -- the pins' path is the builder's argument
	if err != nil {
		return nil, err
	}
	var pins map[string]logoSource
	dec := yaml.NewDecoder(bytes.NewReader(data))
	dec.KnownFields(true)
	if err := dec.Decode(&pins); err != nil {
		return nil, fmt.Errorf("%s does not parse: %w", path, err)
	}
	var out []logoSource
	for _, key := range []string{"selfhst", "dashboard-icons"} {
		s, ok := pins[key]
		if !ok {
			return nil, fmt.Errorf("%s pins no %s", path, key)
		}
		s.Catalog = logoCatalogs[key]
		if err := s.check(withSums); err != nil {
			return nil, fmt.Errorf("%s: %s: %w", path, key, err)
		}
		out = append(out, s)
	}
	if len(pins) != len(out) {
		return nil, fmt.Errorf("%s pins a catalogue Dash does not bundle", path)
	}
	return out, nil
}

// check holds a pin to the forms that reach git and the file system.
func (s logoSource) check(withSum bool) error {
	switch {
	case !repositoryPattern.MatchString(s.Repository):
		return errors.New("repository must be https://github.com/<owner>/<name>")
	case !commitPattern.MatchString(s.Commit):
		return errors.New("commit must be a full commit hash")
	case withSum && !sumPattern.MatchString(s.SHA256):
		return errors.New("sha256 must be 64 hex digits: run make icon-pins")
	case !indexPattern.MatchString(s.Index):
		return errors.New("index must name the catalogue's JSON index")
	case s.Name == "" || s.License == "":
		return errors.New("name and license are required")
	}
	return nil
}
