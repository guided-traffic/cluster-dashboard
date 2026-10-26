package main

import (
	"bytes"
	"compress/gzip"
	"crypto/sha256"
	"encoding/hex"
	"errors"
	"fmt"
	"io/fs"
	"os"
	"path"
	"regexp"
	"strings"
)

// The Cache-Control values (docs/adr/0001 D4).
const (
	// cacheShell is index.html's: it names the current bundle's hashes, so a
	// browser must never keep an old one.
	cacheShell = "no-store"
	// cacheImmutable is a hashed file's: its name changes with its content.
	cacheImmutable = "public, max-age=31536000, immutable"
	// cacheRevalidate is a file that keeps its name across builds — the
	// favicon, theme-init.js: kept, but revalidated by its ETag every time.
	cacheRevalidate = "no-cache"
)

// hashedName matches the content hash Angular's build writes into a file
// name, eight upper-case letters or digits before the extension
// (main-WRY5R6RT.js, inter-latin-wght-normal-NRMW37G5.woff2). A file the
// pattern misses is revalidated rather than kept: a wrong guess costs a
// request, never a stale file.
var hashedName = regexp.MustCompile(`-[A-Z0-9]{8}\.[a-z0-9]+$`)

// contentTypes are the types of the files a build holds, fixed here so the
// answer does not depend on the mime tables of the machine (the image has
// none).
var contentTypes = map[string]string{
	".html":        "text/html; charset=utf-8",
	".js":          "text/javascript; charset=utf-8",
	".mjs":         "text/javascript; charset=utf-8",
	".css":         "text/css; charset=utf-8",
	".json":        "application/json",
	".map":         "application/json",
	".webmanifest": "application/manifest+json",
	".txt":         "text/plain; charset=utf-8",
	".xml":         "application/xml",
	".svg":         "image/svg+xml",
	".png":         "image/png",
	".jpg":         "image/jpeg",
	".webp":        "image/webp",
	".ico":         "image/x-icon",
	".woff":        "font/woff",
	".woff2":       "font/woff2",
	".ttf":         "font/ttf",
}

// compressible are the types worth a gzip copy; fonts and images are
// compressed already.
var compressible = map[string]bool{
	".html": true, ".js": true, ".mjs": true, ".css": true, ".json": true, ".map": true,
	".webmanifest": true, ".txt": true, ".xml": true, ".svg": true, ".ico": true,
}

// gzipMinBytes is the smallest file that gets a gzip copy, as nginx's
// gzip_min_length in cowork's configuration.
const gzipMinBytes = 1024

// asset is one file of the bundle, held in memory with everything its
// answer needs.
type asset struct {
	content     []byte
	gzipped     []byte // nil when the file is not worth compressing
	contentType string
	etag        string // strong, of the identity bytes
	gzipETag    string // strong, of the gzip bytes
	cache       string
}

// bundle is the Angular build, read once at start.
type bundle struct {
	assets map[string]*asset // by URL path: "/main-WRY5R6RT.js"
	index  *asset
}

// loadBundle reads every regular file under dir into memory. Only regular
// files count: a directory is never listed, and a symbolic link is skipped —
// os.Root refuses one that leaves dir, and a link inside is no file a build
// writes. It fails when dir holds no index.html.
func loadBundle(dir string) (*bundle, error) {
	root, err := os.OpenRoot(dir)
	if err != nil {
		return nil, fmt.Errorf("open the bundle: %w", err)
	}
	// Only read through: a failed close loses nothing.
	defer func() { _ = root.Close() }()
	return loadFS(root.FS())
}

// loadFS reads every regular file of files into memory.
func loadFS(files fs.FS) (*bundle, error) {
	b := &bundle{assets: map[string]*asset{}}
	err := fs.WalkDir(files, ".", func(name string, d fs.DirEntry, err error) error {
		if err != nil {
			return err
		}
		if !d.Type().IsRegular() {
			return nil
		}
		content, err := fs.ReadFile(files, name)
		if err != nil {
			return err
		}
		a, err := newAsset(name, content)
		if err != nil {
			return err
		}
		b.assets["/"+name] = a
		return nil
	})
	if err != nil {
		return nil, fmt.Errorf("read the bundle: %w", err)
	}
	b.index = b.assets["/index.html"]
	if b.index == nil {
		return nil, errors.New("the bundle holds no index.html")
	}
	b.index.cache = cacheShell
	return b, nil
}

func newAsset(name string, content []byte) (*asset, error) {
	ext := strings.ToLower(path.Ext(name))
	a := &asset{content: content, contentType: contentTypes[ext], etag: etagOf(content, "")}
	if a.contentType == "" {
		a.contentType = "application/octet-stream"
	}
	a.cache = cacheRevalidate
	if hashedName.MatchString(path.Base(name)) {
		a.cache = cacheImmutable
	}
	if compressible[ext] && len(content) >= gzipMinBytes {
		gz, err := gzipOf(content)
		if err != nil {
			return nil, err
		}
		if len(gz) < len(content) {
			a.gzipped = gz
			a.gzipETag = etagOf(content, "-gz")
		}
	}
	return a, nil
}

// etagOf is a strong ETag of content; each representation has its own, so a
// cache never answers a gzip request with identity bytes or the reverse.
func etagOf(content []byte, suffix string) string {
	sum := sha256.Sum256(content)
	return `"` + hex.EncodeToString(sum[:16]) + suffix + `"`
}

func gzipOf(content []byte) ([]byte, error) {
	var buf bytes.Buffer
	zw, err := gzip.NewWriterLevel(&buf, gzip.BestCompression)
	if err != nil {
		return nil, err
	}
	if _, err := zw.Write(content); err != nil {
		return nil, err
	}
	if err := zw.Close(); err != nil {
		return nil, err
	}
	return buf.Bytes(), nil
}
