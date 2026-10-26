package icons

import (
	"archive/zip"
	"encoding/json"
	"fmt"
	"os"
	"slices"
	"time"
)

// WritePack writes a pack: the manifest — its Count set from files — and
// every file under icons/<name>.svg. The zip is reproducible: entries in
// name order, every time stamp the same, so a pack built twice from the
// same pins is the same file.
func WritePack(path string, m Manifest, files map[string][]byte) error {
	m.Count = len(files)
	manifest, err := json.MarshalIndent(m, "", "  ")
	if err != nil {
		return err
	}
	f, err := os.Create(path) // #nosec G304 -- the path is the builder's output, named by its caller
	if err != nil {
		return err
	}
	z := zip.NewWriter(f)
	stamp := time.Date(1980, 1, 1, 0, 0, 0, 0, time.UTC)
	write := func(name string, data []byte) error {
		w, err := z.CreateHeader(&zip.FileHeader{Name: name, Method: zip.Deflate, Modified: stamp})
		if err != nil {
			return err
		}
		_, err = w.Write(data)
		return err
	}
	if err := write(ManifestFile, manifest); err != nil {
		_ = f.Close()
		return err
	}
	names := make([]string, 0, len(files))
	for name := range files {
		names = append(names, name)
	}
	slices.Sort(names)
	for _, name := range names {
		if err := write(IconDir+name+".svg", files[name]); err != nil {
			_ = f.Close()
			return fmt.Errorf("write %s: %w", name, err)
		}
	}
	if err := z.Close(); err != nil {
		_ = f.Close()
		return err
	}
	return f.Close()
}
