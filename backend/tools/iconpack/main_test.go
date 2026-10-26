package main

import (
	"archive/tar"
	"bytes"
	"compress/gzip"
	"context"
	"crypto/sha256"
	"crypto/sha512"
	"encoding/base64"
	"encoding/hex"
	"encoding/json"
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"slices"
	"strings"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/guided-traffic/dash/backend/internal/icons"
)

const (
	white = `<svg xmlns="http://www.w3.org/2000/svg" viewBox="0 0 24 24"><path style="fill:#fff" d="M0 0h1v1z"/></svg>`
	black = `<svg xmlns="http://www.w3.org/2000/svg" viewBox="0 0 24 24"><path fill="#1b1f23" d="M0 0h1v1z"/></svg>`
	color = `<svg xmlns="http://www.w3.org/2000/svg" viewBox="0 0 24 24"><path fill="#f46800" d="M0 0h1v1z"/><path fill="#000" d="M1 1h1v1z"/></svg>`
	bad   = `<svg xmlns="http://www.w3.org/2000/svg"><script>alert(1)</script></svg>`
)

// tgz is a gzipped tarball of the given files.
func tgz(t *testing.T, files map[string]string) []byte {
	t.Helper()
	var buf bytes.Buffer
	zw := gzip.NewWriter(&buf)
	tw := tar.NewWriter(zw)
	for name, content := range files {
		require.NoError(t, tw.WriteHeader(&tar.Header{Name: name, Mode: 0o644, Size: int64(len(content)), Typeflag: tar.TypeReg}))
		_, err := tw.Write([]byte(content))
		require.NoError(t, err)
	}
	require.NoError(t, tw.Close())
	require.NoError(t, zw.Close())
	return buf.Bytes()
}

func integrity(data []byte) string {
	sum := sha512.Sum512(data)
	return "sha512-" + base64.StdEncoding.EncodeToString(sum[:])
}

func iconifyJSON(icons string) string {
	return `{"prefix":"x","width":24,"height":24,"icons":{` + icons + `},"aliases":{"www":{"parent":"web"},"gone":{"parent":"absent"}}}`
}

// fixture is a pins folder, the tarballs it pins and the logo checkouts the
// fake git writes.
type fixture struct {
	pins     string
	tarballs map[string][]byte
	logos    map[string]map[string]string // catalogue → path → content
	fetched  []string
	gitCalls [][]string
}

func newFixture(t *testing.T) *fixture {
	t.Helper()
	f := &fixture{pins: t.TempDir(), tarballs: map[string][]byte{}, logos: map[string]map[string]string{
		icons.CatalogSelfhst: {
			"index.json": `[{"Name":"GitHub","Reference":"github","SVG":"Yes","Light":"Yes"},
				{"Name":"Grafana","Reference":"grafana","SVG":"Yes","Light":"Yes"},
				{"Name":"Broken","Reference":"broken","SVG":"Yes","Light":"No"},
				{"Name":"Raster only","Reference":"raster","SVG":"No","Light":"No"}]`,
			"svg/github.svg": black, "svg/github-light.svg": white, "svg/grafana.svg": color, "svg/grafana-light.svg": white,
			"svg/broken.svg": bad,
		},
		icons.CatalogDashboardIcons: {
			"metadata.json": `{"wikijs":{"base":"svg","aliases":["Wiki.js wiki"]},
				"github":{"base":"svg","aliases":["GitHub"],"colors":{"dark":"github","light":"github-light"}},
				"mailu":{"base":"png","aliases":[]}}`,
			"svg/wikijs.svg": color, "svg/github.svg": black, "svg/github-light.svg": white,
		},
	}}
	var lock strings.Builder
	lock.WriteString(`{"packages":{"":{},`)
	for i, pkg := range []string{"@iconify-json/mdi", "@iconify-json/material-symbols", "@iconify-json/simple-icons"} {
		data := tgz(t, map[string]string{
			"package/icons.json": iconifyJSON(`"web":{"body":"<path fill=\"currentColor\" d=\"M0 0h24v24H0z\"/>"},` +
				`"apps":{"body":"<path d=\"M0 0h1v1z\"/>"},"bookmark":{"body":"<path d=\"M0 0h1v1z\"/>"},"bad":{"body":"<script/>"}`),
			"package/info.json": `{"name":"Set ` + pkg + `","license":{"spdx":"Apache-2.0"}}`,
		})
		url := npmRegistry + pkg + "/-/x-1.0.0.tgz"
		f.tarballs[url] = data
		if i > 0 {
			lock.WriteString(",")
		}
		fmt.Fprintf(&lock, `"node_modules/%s":{"version":"1.0.0","resolved":%q,"integrity":%q,"license":"Apache-2.0"}`, pkg, url, integrity(data))
	}
	lock.WriteString(`}}`)
	require.NoError(t, os.WriteFile(filepath.Join(f.pins, "package-lock.json"), []byte(lock.String()), 0o600))
	f.writeSources(t, f.sum(t, icons.CatalogSelfhst), f.sum(t, icons.CatalogDashboardIcons))
	f.writeRequired(t, "# what the fixture packs hold\nweb\nmdi:www\nsi:web\n\nmatch GitHub\nmatch Wiki.js wiki\n")
	return f
}

func (f *fixture) writeRequired(t *testing.T, content string) {
	t.Helper()
	require.NoError(t, os.WriteFile(filepath.Join(f.pins, "required.txt"), []byte(content), 0o600))
}

func (f *fixture) writeSources(t *testing.T, shSum, diSum string) {
	t.Helper()
	sources := fmt.Sprintf(`selfhst:
  name: selfh.st icons
  repository: https://github.com/selfhst/icons
  commit: %s
  sha256: %q
  index: index.json
  license: CC-BY-4.0
dashboard-icons:
  name: Dashboard Icons
  repository: https://github.com/homarr-labs/dashboard-icons
  commit: %s
  sha256: %q
  index: metadata.json
  license: Apache-2.0
`, strings.Repeat("a", 40), shSum, strings.Repeat("b", 40), diSum)
	require.NoError(t, os.WriteFile(filepath.Join(f.pins, "sources.yaml"), []byte(sources), 0o600))
}

// sum is the tree sum of a fixture catalogue, computed here the way the
// shell recipe of sources.yaml does: sha256sum lines of the sorted paths.
func (f *fixture) sum(t *testing.T, catalog string) string {
	t.Helper()
	files := f.logos[catalog]
	paths := make([]string, 0, len(files))
	for p := range files {
		paths = append(paths, p)
	}
	slices.Sort(paths)
	var lines strings.Builder
	for _, p := range paths {
		s := sha256.Sum256([]byte(files[p]))
		fmt.Fprintf(&lines, "%s  %s\n", hex.EncodeToString(s[:]), p)
	}
	s := sha256.Sum256([]byte(lines.String()))
	return hex.EncodeToString(s[:])
}

func (f *fixture) builder(t *testing.T) (builder, *bytes.Buffer) {
	t.Helper()
	var log bytes.Buffer
	return builder{
		pins: f.pins, cache: t.TempDir(), out: t.TempDir(), log: &log,
		fetch: func(_ context.Context, url string) ([]byte, error) {
			f.fetched = append(f.fetched, url)
			if data, ok := f.tarballs[url]; ok {
				return data, nil
			}
			return nil, errors.New("404")
		},
		git: func(_ context.Context, dir string, args ...string) error {
			f.gitCalls = append(f.gitCalls, args)
			if args[len(args)-1] != "FETCH_HEAD" {
				return nil
			}
			catalog := icons.CatalogSelfhst
			if strings.Contains(dir, icons.CatalogDashboardIcons+"-") {
				catalog = icons.CatalogDashboardIcons
			}
			for p, content := range f.logos[catalog] {
				require.NoError(t, os.MkdirAll(filepath.Join(dir, filepath.Dir(p)), 0o750))
				require.NoError(t, os.WriteFile(filepath.Join(dir, p), []byte(content), 0o600))
			}
			return nil
		},
	}, &log
}

func TestTheBuilderBuildsEveryPack(t *testing.T) {
	f := newFixture(t)
	b, log := f.builder(t)
	require.NoError(t, b.build(t.Context()))

	packs, err := icons.OpenPacks(b.out)
	require.NoError(t, err)
	defer packs.Close()
	assert.Len(t, packs, 5)

	mdi := packs[icons.CatalogMDI]
	assert.Equal(t, "1.0.0", mdi.Version)
	assert.Equal(t, "Apache-2.0", mdi.License)
	assert.Equal(t, "@iconify-json/mdi@1.0.0", mdi.Source)
	assert.Equal(t, []string{"apps", "bookmark", "web"}, mdi.Names(), "the icon the check refuses is left out")
	assert.Equal(t, 1, mdi.Skipped)
	assert.Equal(t, map[string]string{"www": "web"}, mdi.Aliases, "an alias of a missing icon is dropped")
	web, err := mdi.Read("web")
	require.NoError(t, err)
	assert.Equal(t, `<svg xmlns="http://www.w3.org/2000/svg" width="24" height="24" viewBox="0 0 24 24"><path fill="currentColor" d="M0 0h24v24H0z"/></svg>`, string(web))

	sh := packs[icons.CatalogSelfhst]
	assert.Equal(t, "aaaaaaaaaaaa", sh.Version, "a commit's first twelve hex digits")
	assert.Equal(t, "CC-BY-4.0", sh.License)
	assert.Equal(t, []string{"github", "github-light", "grafana"}, sh.Names())
	assert.Equal(t, map[string]string{"github": "github-light"}, sh.Dark, "only the dark logo takes its light variant")
	assert.Equal(t, 1, sh.Skipped)
	assert.Equal(t, "grafana", sh.Match["grafana"])
	assert.Equal(t, "github", sh.Match["github"])

	di := packs[icons.CatalogDashboardIcons]
	assert.Equal(t, "wikijs", di.Match["wikijs"])
	assert.Equal(t, "wikijs", di.Match["wiki-js-wiki"], "an alias, normalised")
	assert.Equal(t, map[string]string{"github": "github-light"}, di.Dark)
	assert.NotContains(t, di.Names(), "mailu", "a logo without an SVG")

	assert.Contains(t, log.String(), "sh aaaaaaaaaaaa: 2 logos, 1 with a variant for dark backgrounds")
	assert.Contains(t, log.String(), "required.txt: all 5 resolve")
	assert.Contains(t, f.gitCalls, []string{"sparse-checkout", "set", "--no-cone", "/svg/", "/index.json"})
	assert.Contains(t, f.gitCalls, []string{"fetch", "-q", "--depth=1", "--filter=blob:none", "origin", strings.Repeat("a", 40)})

	// A second build reads the cache: no download, no fetch.
	f.fetched, f.gitCalls = nil, nil
	require.NoError(t, b.build(t.Context()))
	assert.Empty(t, f.fetched)
	assert.Empty(t, f.gitCalls)
}

func TestTheBuilderRefusesWhatDoesNotVerify(t *testing.T) {
	f := newFixture(t)
	for url := range f.tarballs {
		f.tarballs[url] = append(f.tarballs[url], 0)
	}
	b, _ := f.builder(t)
	assert.ErrorContains(t, b.build(t.Context()), "the tarball does not match its integrity")

	f = newFixture(t)
	f.writeSources(t, strings.Repeat("0", 64), f.sum(t, icons.CatalogDashboardIcons))
	b, _ = f.builder(t)
	err := b.build(t.Context())
	assert.ErrorContains(t, err, "have the SHA-256 "+f.sum(t, icons.CatalogSelfhst)+", the pin says 0000")
}

// A pin update that drops what required.txt names fails the build, naming
// each.
func TestTheBuildFailsOnAMissingRequiredIcon(t *testing.T) {
	f := newFixture(t)
	f.writeRequired(t, "web\nmdi:gone\nmatch Nothing At All\n")
	b, _ := f.builder(t)
	err := b.build(t.Context())
	require.Error(t, err)
	assert.Equal(t, "the packs do not hold what required.txt names:\n  mdi:gone: names no icon of the bundled catalogues\n  auto-match finds no logo for Nothing At All", err.Error())

	require.NoError(t, os.Remove(filepath.Join(f.pins, "required.txt")))
	assert.Error(t, b.build(t.Context()), "the list is required")
}

func TestTheSumsMode(t *testing.T) {
	f := newFixture(t)
	f.writeSources(t, "", "")
	b, log := f.builder(t)
	require.NoError(t, b.printSums(t.Context()))
	assert.Contains(t, log.String(), "selfh.st icons "+strings.Repeat("a", 40)+" sha256: "+f.sum(t, icons.CatalogSelfhst))
	b.cache = ""
	assert.Error(t, b.printSums(t.Context()))
	assert.Error(t, b.build(t.Context()))
	b, _ = f.builder(t)
	assert.ErrorContains(t, b.build(t.Context()), "sha256 must be 64 hex digits", "a build needs the sums")
}

func TestPins(t *testing.T) {
	dir := t.TempDir()
	write := func(name, content string) string {
		p := filepath.Join(dir, name)
		require.NoError(t, os.WriteFile(p, []byte(content), 0o600))
		return p
	}
	_, err := readLock(write("a.json", `{"packages":{}}`))
	assert.ErrorContains(t, err, "pins no @iconify-json/material-symbols")
	_, err = readLock(write("b.json", `{"packages":{"node_modules/@iconify-json/material-symbols":{"resolved":"https://evil.example.com/x.tgz","integrity":"sha512-x"}}}`))
	assert.ErrorContains(t, err, "outside https://registry.npmjs.org/")
	_, err = readLock(write("c.json", `{"packages":{"node_modules/@iconify-json/material-symbols":{"resolved":"https://registry.npmjs.org/x.tgz","integrity":"sha1-x"}}}`))
	assert.ErrorContains(t, err, "without a sha512 integrity")
	_, err = readLock(write("d.json", `{`))
	assert.Error(t, err)
	_, err = readLock(filepath.Join(dir, "absent.json"))
	assert.Error(t, err)

	good := "name: n\nrepository: https://github.com/a/b\ncommit: " + strings.Repeat("c", 40) + "\nsha256: " + strings.Repeat("d", 64) + "\nindex: index.json\nlicense: MIT\n"
	indent := func(s string) string {
		return "  " + strings.ReplaceAll(strings.TrimSuffix(s, "\n"), "\n", "\n  ") + "\n"
	}
	both := func(sh, di string) string { return "selfhst:\n" + indent(sh) + "dashboard-icons:\n" + indent(di) }
	_, err = readSources(write("ok.yaml", both(good, good)), true)
	assert.NoError(t, err)
	for name, content := range map[string]string{
		"missing":  "selfhst:\n" + indent(good),
		"extra":    both(good, good) + "fa:\n" + indent(good),
		"unknown":  both(good+"branch: main\n", good),
		"repo":     both(strings.Replace(good, "https://github.com/a/b", "https://gitlab.com/a/b", 1), good),
		"commit":   both(strings.Replace(good, strings.Repeat("c", 40), "main", 1), good),
		"index":    both(strings.Replace(good, "index.json", "../x.json", 1), good),
		"no name":  both(strings.Replace(good, "name: n\n", "", 1), good),
		"no sum":   both(strings.Replace(good, strings.Repeat("d", 64), `""`, 1), good),
		"not yaml": "selfhst: [",
	} {
		_, err := readSources(write(name+".yaml", content), true)
		assert.Error(t, err, name)
	}
	_, err = readSources(write("nosum.yaml", both(strings.Replace(good, strings.Repeat("d", 64), `""`, 1), good)), false)
	assert.NoError(t, err, "the sums mode needs no sum")
}

func TestGlyphFilesRefuseTransforms(t *testing.T) {
	for name, set := range map[string]string{
		"rotated icon":  `{"icons":{"a":{"body":"<g/>","rotate":1}}}`,
		"sized icon":    `{"icons":{"a":{"body":"<g/>","width":10}}}`,
		"flipped alias": `{"icons":{"a":{"body":"<g/>"}},"aliases":{"b":{"parent":"a","hFlip":true}}}`,
	} {
		var s iconifySet
		require.NoError(t, jsonUnmarshal(set, &s))
		_, _, _, err := glyphFiles(s)
		assert.ErrorIs(t, err, errTransformed, name)
	}
	var s iconifySet
	require.NoError(t, jsonUnmarshal(`{"icons":{"a":{"body":"<g/>"},"Bad Name":{"body":"<g/>"}}}`, &s))
	files, _, skipped, err := glyphFiles(s)
	require.NoError(t, err)
	assert.Equal(t, 1, skipped)
	assert.Contains(t, string(files["a"]), `viewBox="0 0 16 16"`, "the format's default grid")
}

func TestTarballAndFetch(t *testing.T) {
	data := tgz(t, map[string]string{"package/icons.json": "{}"})
	got, err := tarFile(data, "package/icons.json")
	require.NoError(t, err)
	assert.Equal(t, "{}", string(got))
	_, err = tarFile(data, "package/other.json")
	assert.ErrorContains(t, err, "holds no package/other.json")
	_, err = tarFile([]byte("not gzip"), "x")
	assert.Error(t, err)
	assert.Error(t, verifyIntegrity(data, "sha1-abc"))
	_, err = httpGet(t.Context(), "http://registry.npmjs.org/x")
	assert.ErrorContains(t, err, "https only")
}

func TestTheGitFailureIsReported(t *testing.T) {
	f := newFixture(t)
	b, _ := f.builder(t)
	b.git = func(context.Context, string, ...string) error { return errors.New("git fetch: exit status 128") }
	assert.ErrorContains(t, b.build(t.Context()), "exit status 128")
	assert.ErrorContains(t, runGit(t.Context(), t.TempDir(), "no-such-subcommand"), "git no-such-subcommand")
}

func TestIsDark(t *testing.T) {
	for svg, want := range map[string]bool{
		black:                         true,
		white:                         false,
		color:                         false,
		`<svg><path d="M0 0"/></svg>`: true,
		`<svg><path fill="none" stroke="currentColor"/></svg>`:          true,
		`<svg><path fill="rgb(255, 255, 255)"/></svg>`:                  false,
		`<svg><path fill="rgb(10%,10%,10%)"/></svg>`:                    true,
		`<svg><path fill="rgba(0 0 0 / 50%)"/></svg>`:                   true,
		`<svg><path fill="#777"/></svg>`:                                true,
		`<svg><path fill="#888"/></svg>`:                                false,
		`<svg><path fill="#00000080"/></svg>`:                           true,
		`<svg><stop stop-color="#ffcc00"/><path fill="url(#g)"/></svg>`: false,
		`<svg><path fill="white"/></svg>`:                               false,
		`<svg><path fill="rgb(1)"/></svg>`:                              true,
	} {
		assert.Equal(t, want, isDark([]byte(svg)), svg)
	}
	_, known := luminance("#zz")
	assert.False(t, known)
	assert.InDelta(t, 0.0, hexByte("zz"), 0)
}

func jsonUnmarshal(s string, into any) error {
	return json.Unmarshal([]byte(s), into)
}
