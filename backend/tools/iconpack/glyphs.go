package main

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"path/filepath"

	"github.com/guided-traffic/dash/backend/internal/icons"
)

// iconifySet is an Iconify JSON icon set (icons.json of @iconify-json/*):
// each icon's SVG body on a grid of width × height.
type iconifySet struct {
	Width   int                     `json:"width"`
	Height  int                     `json:"height"`
	Icons   map[string]iconifyIcon  `json:"icons"`
	Aliases map[string]iconifyAlias `json:"aliases"`
}

// iconifyIcon is one icon. The size, offset and transformation fields are
// read only to refuse them: none of the bundled sets uses them (checked
// 2026-10-26), and a set that starts to would need the transforms drawn.
type iconifyIcon struct {
	Body   string `json:"body"`
	Width  *int   `json:"width"`
	Height *int   `json:"height"`
	Left   *int   `json:"left"`
	Top    *int   `json:"top"`
	Rotate *int   `json:"rotate"`
	HFlip  *bool  `json:"hFlip"`
	VFlip  *bool  `json:"vFlip"`
}

type iconifyAlias struct {
	Parent string `json:"parent"`
	Width  *int   `json:"width"`
	Height *int   `json:"height"`
	Rotate *int   `json:"rotate"`
	HFlip  *bool  `json:"hFlip"`
	VFlip  *bool  `json:"vFlip"`
}

func (i iconifyIcon) transformed() bool {
	return i.Width != nil || i.Height != nil || i.Left != nil || i.Top != nil || i.Rotate != nil || i.HFlip != nil || i.VFlip != nil
}

func (a iconifyAlias) transformed() bool {
	return a.Width != nil || a.Height != nil || a.Rotate != nil || a.HFlip != nil || a.VFlip != nil
}

// iconifyInfo is the set's info.json: its name and licence.
type iconifyInfo struct {
	Name    string `json:"name"`
	License struct {
		SPDX string `json:"spdx"`
	} `json:"license"`
}

// iconifyDefault is the grid of a set that names none (the Iconify format's
// default).
const iconifyDefault = 16

var errTransformed = errors.New("the set uses sizes, offsets or transformations per icon, which the builder does not draw")

// buildGlyphs builds the pack of an Iconify set.
func (b builder) buildGlyphs(ctx context.Context, g glyphSource) error {
	tgz, err := b.tarball(ctx, g)
	if err != nil {
		return err
	}
	var set iconifySet
	var info iconifyInfo
	for name, into := range map[string]any{"package/icons.json": &set, "package/info.json": &info} {
		data, err := tarFile(tgz, name)
		if err != nil {
			return err
		}
		if err := json.Unmarshal(data, into); err != nil {
			return fmt.Errorf("%s does not parse: %w", name, err)
		}
	}
	files, aliases, skipped, err := glyphFiles(set)
	if err != nil {
		return err
	}
	m := icons.Manifest{
		Catalog: g.catalog, Name: info.Name, Version: g.version, License: info.License.SPDX,
		Source: g.pkg + "@" + g.version, Skipped: skipped, Aliases: aliases,
	}
	if err := icons.WritePack(filepath.Join(b.out, g.catalog+".zip"), m, files); err != nil {
		return err
	}
	fmt.Fprintf(b.log, "%s %s: %d icons, %d aliases, %d left out by the check\n", g.catalog, g.version, len(files), len(aliases), skipped)
	return nil
}

// glyphFiles draws every icon of a set as an SVG document and keeps those
// the content check passes; the aliases lead to a kept icon.
func glyphFiles(set iconifySet) (map[string][]byte, map[string]string, int, error) {
	w, h := set.Width, set.Height
	if w == 0 {
		w = iconifyDefault
	}
	if h == 0 {
		h = iconifyDefault
	}
	files := map[string][]byte{}
	skipped := 0
	for name, icon := range set.Icons {
		if icon.transformed() {
			return nil, nil, 0, fmt.Errorf("%s: %w", name, errTransformed)
		}
		svg := fmt.Appendf(nil, `<svg xmlns="http://www.w3.org/2000/svg" width="%d" height="%d" viewBox="0 0 %d %d">%s</svg>`, w, h, w, h, icon.Body)
		if !icons.ValidIconName(name) || icons.CheckSVG(svg) != nil {
			skipped++
			continue
		}
		files[name] = svg
	}
	aliases := map[string]string{}
	for name, a := range set.Aliases {
		if a.transformed() {
			return nil, nil, 0, fmt.Errorf("alias %s: %w", name, errTransformed)
		}
		if _, ok := files[a.Parent]; ok && icons.ValidIconName(name) {
			aliases[name] = a.Parent
		}
	}
	return files, aliases, skipped, nil
}
