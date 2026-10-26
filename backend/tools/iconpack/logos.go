package main

import (
	"context"
	"encoding/json"
	"fmt"
	"math"
	"os"
	"path/filepath"
	"regexp"
	"slices"
	"strconv"
	"strings"

	"github.com/guided-traffic/dash/backend/internal/icons"
)

// logoEntry is one logo of a catalogue's index.
type logoEntry struct {
	// name is the logo's file name, without .svg.
	name string
	// light is the file name of its variant for dark backgrounds, "" for
	// none. Both catalogues name it <name>-light (checked 2026-10-26:
	// Dashboard Icons' README says so; selfh.st's github-light.svg is
	// white).
	light string
	// names are what auto-match finds it by.
	names []string
}

// buildLogos builds the pack of a logo catalogue from the pinned commit's
// files, which must have the pinned SHA-256.
func (b builder) buildLogos(ctx context.Context, s logoSource) error {
	dir, err := b.checkout(ctx, s)
	if err != nil {
		return err
	}
	sum, err := treeSum(dir, s.Index)
	if err != nil {
		return err
	}
	if sum != s.SHA256 {
		return fmt.Errorf("the files of %s at %s have the SHA-256 %s, the pin says %s: look at the change, then run make icon-pins",
			s.Repository, s.Commit, sum, s.SHA256)
	}
	index, err := os.ReadFile(filepath.Join(dir, s.Index)) // #nosec G304 -- the pinned index in the checkout
	if err != nil {
		return err
	}
	var entries []logoEntry
	if s.Catalog == icons.CatalogSelfhst {
		entries, err = selfhstEntries(index)
	} else {
		entries, err = dashboardIconsEntries(index)
	}
	if err != nil {
		return fmt.Errorf("%s does not parse: %w", s.Index, err)
	}
	files, dark, match, skipped := logoFiles(dir, entries)
	m := icons.Manifest{
		Catalog: s.Catalog, Name: s.Name, Version: s.Commit[:12], License: s.License,
		Source: s.Repository + "@" + s.Commit, Skipped: skipped, Dark: dark, Match: match,
	}
	if err := icons.WritePack(filepath.Join(b.out, s.Catalog+".zip"), m, files); err != nil {
		return err
	}
	fmt.Fprintf(b.log, "%s %s: %d logos, %d with a variant for dark backgrounds, %d names to match, %d left out\n",
		s.Catalog, m.Version, len(files)-len(dark), len(dark), len(match), skipped)
	return nil
}

// selfhstEntries reads selfh.st's index.json: a list of logos, each with its
// name, its file name ("Reference") and whether it has an SVG and a light
// variant.
func selfhstEntries(index []byte) ([]logoEntry, error) {
	var list []struct {
		Name      string `json:"Name"`
		Reference string `json:"Reference"`
		SVG       string `json:"SVG"`
		Light     string `json:"Light"`
	}
	if err := json.Unmarshal(index, &list); err != nil {
		return nil, err
	}
	var out []logoEntry
	for _, e := range list {
		if e.SVG != "Yes" {
			continue
		}
		entry := logoEntry{name: e.Reference, names: []string{e.Reference, e.Name}}
		if e.Light == "Yes" {
			entry.light = e.Reference + "-light"
		}
		out = append(out, entry)
	}
	return out, nil
}

// dashboardIconsEntries reads Dashboard Icons' metadata.json: a map of file
// name to the format of its base file, its aliases and its colour variants.
func dashboardIconsEntries(index []byte) ([]logoEntry, error) {
	var meta map[string]struct {
		Base    string   `json:"base"`
		Aliases []string `json:"aliases"`
		Colors  *struct {
			Light string `json:"light"`
		} `json:"colors"`
	}
	if err := json.Unmarshal(index, &meta); err != nil {
		return nil, err
	}
	var out []logoEntry
	for name, m := range meta {
		if m.Base != "svg" {
			continue
		}
		entry := logoEntry{name: name, names: append([]string{name}, m.Aliases...)}
		if m.Colors != nil && m.Colors.Light != name {
			entry.light = m.Colors.Light
		}
		out = append(out, entry)
	}
	return out, nil
}

// logoFiles reads and checks every logo of the entries. A logo's variant
// for dark backgrounds is kept only where the logo is dark itself: the
// variants are single-coloured, and a coloured logo shows on a dark stage as
// it is. The match table takes each name once, the first logo in name
// order to claim it; the compact forms come after every normalised one, so
// a compact form never takes a name another logo has exactly.
func logoFiles(dir string, entries []logoEntry) (files map[string][]byte, dark, match map[string]string, skipped int) {
	slices.SortFunc(entries, func(a, b logoEntry) int { return strings.Compare(a.name, b.name) })
	files, dark, match = map[string][]byte{}, map[string]string{}, map[string]string{}
	for _, e := range entries {
		raw, ok := readLogo(dir, e.name)
		if !ok {
			skipped++
			continue
		}
		files[e.name] = raw
		if e.light != "" && isDark(raw) {
			if light, ok := readLogo(dir, e.light); ok {
				files[e.light] = light
				dark[e.name] = e.light
			}
		}
		claim(match, e, icons.NormalizeName)
	}
	for _, e := range entries {
		if _, kept := files[e.name]; kept {
			claim(match, e, icons.CompactName)
		}
	}
	return files, dark, match, skipped
}

// claim enters each of a logo's names, in the given form, that no logo has
// yet.
func claim(match map[string]string, e logoEntry, form func(string) string) {
	for _, n := range e.names {
		if key := form(n); key != "" {
			if _, taken := match[key]; !taken {
				match[key] = e.name
			}
		}
	}
}

// readLogo reads svg/<name>.svg of the checkout and checks it.
func readLogo(dir, name string) ([]byte, bool) {
	if !icons.ValidIconName(name) {
		return nil, false
	}
	raw, err := os.ReadFile(filepath.Join(dir, "svg", name+".svg")) // #nosec G304 -- a file of the checkout, its name checked
	if err != nil || icons.CheckSVG(raw) != nil {
		return nil, false
	}
	return raw, true
}

// colourValue finds the colours an SVG paints with: in an attribute or a
// style, fill, stroke, stop-color and color.
var colourValue = regexp.MustCompile(`(?i)(?:fill|stroke|stop-color|color)\s*(?:=\s*["']|:)\s*(#[0-9a-f]{3,8}|rgba?\([^)]*\)|[a-z]+)`)

// darkLuminance is the relative luminance below which a colour is dark:
// GitHub's #1b1f23 is 0.013, a mid grey #777 is 0.18.
const darkLuminance = 0.2

// isDark reports whether every colour a logo paints with is dark — or it
// names none, and SVG paints black. "none", "transparent", "currentColor"
// (black in an image) and a gradient reference (its stops count) aside.
func isDark(svg []byte) bool {
	for _, m := range colourValue.FindAllSubmatch(svg, -1) {
		l, known := luminance(strings.ToLower(string(m[1])))
		if known && l >= darkLuminance {
			return false
		}
	}
	return true
}

// named are the colour keywords logos use; another keyword counts as
// unknown.
var named = map[string]float64{"black": 0, "white": 1, "red": 0.2126, "green": 0.1544, "blue": 0.0722, "gray": 0.2159, "grey": 0.2159}

// luminance is a colour's relative luminance (WCAG 2), and whether it is a
// colour at all.
func luminance(c string) (float64, bool) {
	if l, ok := named[c]; ok {
		return l, true
	}
	var r, g, b float64
	switch {
	case strings.HasPrefix(c, "#") && (len(c) == 4 || len(c) == 5):
		r, g, b = hexByte(c[1:2]+c[1:2]), hexByte(c[2:3]+c[2:3]), hexByte(c[3:4]+c[3:4])
	case strings.HasPrefix(c, "#") && (len(c) == 7 || len(c) == 9):
		r, g, b = hexByte(c[1:3]), hexByte(c[3:5]), hexByte(c[5:7])
	case strings.HasPrefix(c, "rgb"):
		parts := strings.FieldsFunc(strings.TrimSuffix(c[strings.IndexByte(c, '(')+1:], ")"), func(r rune) bool { return r == ',' || r == ' ' || r == '/' })
		if len(parts) < 3 {
			return 0, false
		}
		r, g, b = rgbPart(parts[0]), rgbPart(parts[1]), rgbPart(parts[2])
	default:
		return 0, false
	}
	return 0.2126*linear(r) + 0.7152*linear(g) + 0.0722*linear(b), true
}

func hexByte(s string) float64 {
	v, err := strconv.ParseUint(s, 16, 8)
	if err != nil {
		return 0
	}
	return float64(v) / 255
}

func rgbPart(s string) float64 {
	if p, ok := strings.CutSuffix(s, "%"); ok {
		v, _ := strconv.ParseFloat(p, 64)
		return v / 100
	}
	v, _ := strconv.ParseFloat(s, 64)
	return v / 255
}

func linear(c float64) float64 {
	if c <= 0.04045 {
		return c / 12.92
	}
	return math.Pow((c+0.055)/1.055, 2.4)
}
