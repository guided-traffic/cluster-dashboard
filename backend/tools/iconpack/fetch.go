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
	"errors"
	"fmt"
	"io"
	"io/fs"
	"net/http"
	"os"
	"os/exec"
	"path/filepath"
	"slices"
	"strings"
	"time"
)

// maxDownload bounds one tarball.
const maxDownload = 64 << 20

// httpGet downloads url over https, bounded in size and time.
func httpGet(ctx context.Context, url string) ([]byte, error) {
	if !strings.HasPrefix(url, "https://") {
		return nil, errors.New("downloads are https only")
	}
	ctx, cancel := context.WithTimeout(ctx, 5*time.Minute)
	defer cancel()
	req, err := http.NewRequestWithContext(ctx, http.MethodGet, url, nil)
	if err != nil {
		return nil, err
	}
	resp, err := http.DefaultClient.Do(req)
	if err != nil {
		return nil, err
	}
	defer func() { _ = resp.Body.Close() }()
	if resp.StatusCode != http.StatusOK {
		return nil, fmt.Errorf("GET %s: %s", url, resp.Status)
	}
	data, err := io.ReadAll(io.LimitReader(resp.Body, maxDownload+1))
	if err != nil {
		return nil, err
	}
	if len(data) > maxDownload {
		return nil, fmt.Errorf("GET %s: larger than %d bytes", url, maxDownload)
	}
	return data, nil
}

// tarball is an Iconify set's npm tarball, verified against its integrity:
// the cached copy when it still verifies, else a fresh download.
func (b builder) tarball(ctx context.Context, g glyphSource) ([]byte, error) {
	path := filepath.Join(b.cache, "npm", strings.ReplaceAll(strings.TrimPrefix(g.pkg, "@"), "/", "-")+"-"+g.version+".tgz")
	if data, err := os.ReadFile(path); err == nil && verifyIntegrity(data, g.integrity) == nil { // #nosec G304 -- a path in the builder's cache
		return data, nil
	}
	data, err := b.fetch(ctx, g.resolved)
	if err != nil {
		return nil, err
	}
	if err := verifyIntegrity(data, g.integrity); err != nil {
		return nil, fmt.Errorf("%s@%s: %w", g.pkg, g.version, err)
	}
	if err := os.MkdirAll(filepath.Dir(path), 0o750); err != nil {
		return nil, err
	}
	return data, os.WriteFile(path, data, 0o600)
}

// verifyIntegrity checks data against an npm integrity string, sha512-<base64>.
func verifyIntegrity(data []byte, integrity string) error {
	want, ok := strings.CutPrefix(integrity, "sha512-")
	if !ok {
		return errors.New("the integrity is not sha512")
	}
	sum := sha512.Sum512(data)
	if base64.StdEncoding.EncodeToString(sum[:]) != want {
		return errors.New("the tarball does not match its integrity")
	}
	return nil
}

// tarFile reads one file of a gzipped tarball.
func tarFile(tgz []byte, name string) ([]byte, error) {
	zr, err := gzip.NewReader(bytes.NewReader(tgz))
	if err != nil {
		return nil, err
	}
	tr := tar.NewReader(zr)
	for {
		h, err := tr.Next()
		if errors.Is(err, io.EOF) {
			return nil, fmt.Errorf("the tarball holds no %s", name)
		}
		if err != nil {
			return nil, err
		}
		if h.Name == name && h.Typeflag == tar.TypeReg {
			data, err := io.ReadAll(io.LimitReader(tr, maxDownload+1))
			if err == nil && len(data) > maxDownload {
				err = fmt.Errorf("%s is larger than %d bytes", name, maxDownload)
			}
			return data, err
		}
	}
}

// runGit runs git in dir; its output goes into the error only.
func runGit(ctx context.Context, dir string, args ...string) error {
	cmd := exec.CommandContext(ctx, "git", args...) // #nosec G204 -- the arguments are the pins', checked against patterns (pins.go)
	cmd.Dir = dir
	cmd.Env = append(os.Environ(), "GIT_TERMINAL_PROMPT=0")
	if out, err := cmd.CombinedOutput(); err != nil {
		return fmt.Errorf("git %s: %w: %s", args[0], err, bytes.TrimSpace(out))
	}
	return nil
}

// doneMarker marks a checkout that completed.
const doneMarker = ".dash-checkout-done"

// checkout fetches the files a logo pack is built from — svg/ and the
// index — at the pinned commit into the cache: a sparse, blob-filtered,
// one-commit fetch, so of a repository of hundreds of megabytes only those
// files arrive. A completed checkout is reused; whether it is the pinned
// one is treeSum's to say.
func (b builder) checkout(ctx context.Context, s logoSource) (string, error) {
	dir := filepath.Join(b.cache, "git", s.Catalog+"-"+s.Commit)
	if _, err := os.Stat(filepath.Join(dir, doneMarker)); err == nil {
		return dir, nil
	}
	if err := os.RemoveAll(dir); err != nil {
		return "", err
	}
	if err := os.MkdirAll(dir, 0o750); err != nil {
		return "", err
	}
	for _, args := range [][]string{
		{"init", "-q"},
		{"remote", "add", "origin", s.Repository},
		{"sparse-checkout", "set", "--no-cone", "/svg/", "/" + s.Index},
		{"fetch", "-q", "--depth=1", "--filter=blob:none", "origin", s.Commit},
		{"-c", "advice.detachedHead=false", "checkout", "-q", "FETCH_HEAD"},
	} {
		if err := b.git(ctx, dir, args...); err != nil {
			return "", err
		}
	}
	return dir, os.WriteFile(filepath.Join(dir, doneMarker), nil, 0o600)
}

// treeSum is the SHA-256 of the files a logo pack is built from — every
// file under svg/ and the index — as
//
//	find svg <index> -type f | LC_ALL=C sort | xargs sha256sum | sha256sum
//
// prints it in the checkout.
func treeSum(dir, index string) (string, error) {
	var paths []string
	err := filepath.WalkDir(filepath.Join(dir, "svg"), func(path string, d fs.DirEntry, err error) error {
		if err != nil {
			return err
		}
		if d.Type().IsRegular() {
			rel, err := filepath.Rel(dir, path)
			if err != nil {
				return err
			}
			paths = append(paths, filepath.ToSlash(rel))
		}
		return nil
	})
	if err != nil {
		return "", err
	}
	paths = append(paths, index)
	slices.Sort(paths)
	sum := sha256.New()
	for _, p := range paths {
		data, err := os.ReadFile(filepath.Join(dir, filepath.FromSlash(p))) // #nosec G304 -- a file of the checkout
		if err != nil {
			return "", err
		}
		file := sha256.Sum256(data)
		fmt.Fprintf(sum, "%s  %s\n", hex.EncodeToString(file[:]), p)
	}
	return hex.EncodeToString(sum.Sum(nil)), nil
}
