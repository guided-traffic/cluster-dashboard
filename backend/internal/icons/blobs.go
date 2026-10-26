package icons

import (
	"crypto/sha256"
	"encoding/hex"
	"errors"
	"fmt"
	"io/fs"
	"log/slog"
	"os"
	"path/filepath"
	"regexp"
	"slices"
	"strings"
	"sync"
)

// Icons that come from no catalogue — a ConfigMap's key, a downloaded URL —
// are kept in memory after the content check, addressed by the SHA-256 of
// what is served: /api/v1/icons/blob/<sha256>.<png|svg>. The address names
// one content for good, so it is cached as immutably as a catalogue's.

// blob is a checked icon.
type blob struct {
	data   []byte
	format string
}

// blobs are the kept icons by their file name, <sha256>.<format>.
type blobs struct {
	mu    sync.RWMutex
	files map[string]blob
}

func newBlobs() *blobs {
	return &blobs{files: map[string]blob{}}
}

// put keeps a checked icon and returns its address.
func (b *blobs) put(data []byte, format string) string {
	sum := sha256.Sum256(data)
	name := hex.EncodeToString(sum[:]) + "." + format
	b.mu.Lock()
	defer b.mu.Unlock()
	b.files[name] = blob{data: data, format: format}
	return PathPrefix + "blob/" + name
}

func (b *blobs) get(name string) (blob, bool) {
	b.mu.RLock()
	defer b.mu.RUnlock()
	f, ok := b.files[name]
	return f, ok
}

// blobFile is the form of a kept icon's file name.
var blobFile = regexp.MustCompile(`^[0-9a-f]{64}\.(png|svg)$`)

// Blob is a kept icon and its media type, by the file name its Src ends in.
func (r *Resolver) Blob(file string) ([]byte, string, error) {
	if !blobFile.MatchString(file) {
		return nil, "", ErrNoIcon
	}
	b, ok := r.blobs.get(file)
	if !ok {
		return nil, "", ErrNoIcon
	}
	return b.data, ContentType(b.format), nil
}

// maxConfigMapIcon bounds a key's file: a ConfigMap holds at most 1 MiB.
const maxConfigMapIcon = 1 << 20

// configMapIcon is a key of a mounted icon ConfigMap.
type configMapIcon struct {
	configMap string
	src       string
}

// loadConfigMaps reads the icon ConfigMaps mounted under dir, one folder per
// ConfigMap named after it (docs/adr/0008 D4): every key is an icon,
// referenced as configmap:<key>. A key in two ConfigMaps refuses the start
// and names both; a key whose content the check refuses is left out with a
// warning. A missing dir holds none.
func loadConfigMaps(dir string, b *blobs, logger *slog.Logger) (map[string]configMapIcon, error) {
	icons := map[string]configMapIcon{}
	maps, err := os.ReadDir(dir)
	if errors.Is(err, fs.ErrNotExist) {
		return icons, nil
	}
	if err != nil {
		return nil, fmt.Errorf("read the icon ConfigMaps in %s: %w", dir, err)
	}
	for _, m := range maps {
		if !m.IsDir() || strings.HasPrefix(m.Name(), ".") {
			continue
		}
		keys, err := configMapKeys(filepath.Join(dir, m.Name()))
		if err != nil {
			return nil, err
		}
		for _, key := range keys {
			if prev, twice := icons[key]; twice {
				return nil, fmt.Errorf("the icon key %s is in the ConfigMaps %s and %s; keys must be unique across them", key, prev.configMap, m.Name())
			}
			raw, err := readBounded(filepath.Join(dir, m.Name(), key), maxConfigMapIcon)
			if err != nil {
				return nil, fmt.Errorf("read the icon %s of the ConfigMap %s: %w", key, m.Name(), err)
			}
			data, format, err := CheckIcon(raw)
			if err != nil {
				logger.Warn("a ConfigMap icon is left out", "configmap", m.Name(), "key", key, "reason", err.Error())
				continue
			}
			icons[key] = configMapIcon{configMap: m.Name(), src: b.put(data, format)}
		}
	}
	return icons, nil
}

// configMapKeys are a mounted ConfigMap's keys: the folder's entries but the
// kubelet's own (..data and the timestamped folder it points to).
func configMapKeys(dir string) ([]string, error) {
	entries, err := os.ReadDir(dir)
	if err != nil {
		return nil, fmt.Errorf("read the icon ConfigMap %s: %w", filepath.Base(dir), err)
	}
	var keys []string
	for _, e := range entries {
		if strings.HasPrefix(e.Name(), "..") || e.IsDir() {
			continue
		}
		keys = append(keys, e.Name())
	}
	slices.Sort(keys)
	return keys, nil
}

func readBounded(path string, limit int64) ([]byte, error) {
	f, err := os.Open(path) // #nosec G304 -- a key file of a mounted ConfigMap
	if err != nil {
		return nil, err
	}
	defer func() { _ = f.Close() }()
	fi, err := f.Stat()
	if err != nil {
		return nil, err
	}
	if !fi.Mode().IsRegular() {
		return nil, errors.New("not a file")
	}
	if fi.Size() > limit {
		return nil, fmt.Errorf("larger than %d bytes", limit)
	}
	data := make([]byte, fi.Size())
	if _, err := f.ReadAt(data, 0); err != nil && fi.Size() > 0 {
		return nil, err
	}
	return data, nil
}
