package icons

import (
	"bytes"
	pngpkg "image/png"
	"log/slog"
	"os"
	"path/filepath"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// mountConfigMap lays a ConfigMap out as the kubelet mounts it: the keys
// in a timestamped folder, ..data pointing at it, each key a link through
// ..data.
func mountConfigMap(t *testing.T, root, name string, keys map[string][]byte) {
	t.Helper()
	dir := filepath.Join(root, name)
	stamp := filepath.Join(dir, "..2026_10_26_12_00_00.123")
	require.NoError(t, os.MkdirAll(stamp, 0o750))
	for k, v := range keys {
		require.NoError(t, os.WriteFile(filepath.Join(stamp, k), v, 0o600))
	}
	require.NoError(t, os.Symlink("..2026_10_26_12_00_00.123", filepath.Join(dir, "..data")))
	for k := range keys {
		require.NoError(t, os.Symlink(filepath.Join("..data", k), filepath.Join(dir, k)))
	}
}

func TestConfigMapIcons(t *testing.T) {
	root := t.TempDir()
	png := pngOf(t, 300, 300)
	mountConfigMap(t, root, "dash-icons", map[string][]byte{"billing.jpg": png, "logo.svg": []byte(logo)})
	mountConfigMap(t, root, "team-icons", map[string][]byte{"team.png": png, "broken.svg": []byte("<svg><script/></svg>")})
	require.NoError(t, os.WriteFile(filepath.Join(root, "stray-file"), nil, 0o600))

	var log bytes.Buffer
	r, err := NewResolver(Options{
		Packs: testPacks(t), DefaultApp: "configmap:logo.svg", DefaultBookmark: "material-symbols:bookmark",
		ConfigMaps: root, Logger: slog.New(slog.NewTextHandler(&log, nil)),
	})
	require.NoError(t, err)

	billing := r.App("configmap:billing.jpg", nil)
	assert.Regexp(t, `^/api/v1/icons/blob/[0-9a-f]{64}\.png$`, billing.Src)
	assert.False(t, billing.Tint, "an image is drawn as it is")
	data, contentType, err := r.Blob(filepath.Base(billing.Src))
	require.NoError(t, err)
	assert.Equal(t, "image/png", contentType)
	cfg, err := pngpkg.DecodeConfig(bytes.NewReader(data))
	require.NoError(t, err)
	assert.Equal(t, [2]int{256, 256}, [2]int{cfg.Width, cfg.Height}, "re-encoded at 256 pixels a side")

	svg := r.App("", nil)
	assert.Regexp(t, `/api/v1/icons/blob/[0-9a-f]{64}\.svg$`, svg.Src, "a ConfigMap key as the default")
	data, contentType, err = r.Blob(filepath.Base(svg.Src))
	require.NoError(t, err)
	assert.Equal(t, "image/svg+xml", contentType)
	assert.Equal(t, logo, string(data))

	assert.ErrorIs(t, r.Register("configmap:broken.svg", true), ErrNoConfigMap, "left out by the check")
	assert.Contains(t, log.String(), `msg="a ConfigMap icon is left out" configmap=team-icons key=broken.svg`)
	assert.ErrorIs(t, r.Register("configmap:absent.png", true), ErrNoConfigMap)
	assert.Equal(t, r.App("", nil), r.App("configmap:absent.png", nil), "a missing key falls back")

	for _, file := range []string{"nothing.png", "../x.png", filepath.Base(billing.Src)[:64] + ".gif", "0000000000000000000000000000000000000000000000000000000000000000.png"} {
		_, _, err := r.Blob(file)
		assert.ErrorIs(t, err, ErrNoIcon, file)
	}
}

func TestAKeyInTwoConfigMapsRefusesTheStart(t *testing.T) {
	root := t.TempDir()
	mountConfigMap(t, root, "a", map[string][]byte{"logo.svg": []byte(logo)})
	mountConfigMap(t, root, "b", map[string][]byte{"logo.svg": []byte(logo)})
	_, err := NewResolver(Options{Packs: testPacks(t), DefaultApp: "mdi:web", DefaultBookmark: "mdi:web", ConfigMaps: root})
	assert.EqualError(t, err, "the icon key logo.svg is in the ConfigMaps a and b; keys must be unique across them")
}

func TestConfigMapsThatCannotBeRead(t *testing.T) {
	root := t.TempDir()
	mountConfigMap(t, root, "big", map[string][]byte{"huge.png": make([]byte, maxConfigMapIcon+1)})
	_, err := NewResolver(Options{Packs: testPacks(t), DefaultApp: "mdi:web", DefaultBookmark: "mdi:web", ConfigMaps: root})
	assert.ErrorContains(t, err, "read the icon huge.png of the ConfigMap big: larger than")

	notADir := filepath.Join(t.TempDir(), "file")
	require.NoError(t, os.WriteFile(notADir, nil, 0o600))
	_, err = NewResolver(Options{Packs: testPacks(t), DefaultApp: "mdi:web", DefaultBookmark: "mdi:web", ConfigMaps: notADir})
	assert.ErrorContains(t, err, "read the icon ConfigMaps")

	r, err := NewResolver(Options{Packs: testPacks(t), DefaultApp: "mdi:web", DefaultBookmark: "mdi:web", ConfigMaps: filepath.Join(t.TempDir(), "absent")})
	require.NoError(t, err, "no folder, no ConfigMap icons")
	assert.ErrorIs(t, r.Register("configmap:x.png", false), ErrNoConfigMap)
}
