// Command iconpack builds the icon packs the backend image carries
// (docs/adr/0008 D1): one zip of checked SVG files and a manifest per
// catalogue, from the pins in backend/icons/ — the Iconify sets by
// package-lock.json (tarball and integrity), selfh.st and Dashboard Icons by
// sources.yaml (a commit and the SHA-256 of the files used).
//
//	iconpack -pins icons -cache DIR -out DIR   build every pack into DIR
//	iconpack -pins icons -cache DIR -sums      print the SHA-256 of each pinned commit's files
//
// Every download is verified before it is used: an npm tarball against its
// integrity, a commit's files against the pinned SHA-256. Every SVG passes
// the content check of internal/icons; one that fails is left out and
// counted. The packs must then resolve every reference of required.txt. The
// image build runs it (backend/Containerfile); make icon-packs builds the
// packs into .cache for make run.
package main

import (
	"context"
	"flag"
	"fmt"
	"io"
	"os"
	"path/filepath"
	"time"
)

func main() {
	pins := flag.String("pins", "icons", "the folder of package-lock.json and sources.yaml")
	cacheDir := flag.String("cache", "", "where downloads are kept between runs (required)")
	out := flag.String("out", "", "where the packs are written")
	sums := flag.Bool("sums", false, "print the SHA-256 of each pinned commit's files and exit")
	flag.Parse()
	ctx, cancel := context.WithTimeout(context.Background(), 30*time.Minute)
	defer cancel()
	b := builder{pins: *pins, cache: *cacheDir, out: *out, log: os.Stdout, git: runGit, fetch: httpGet}
	var err error
	if *sums {
		err = b.printSums(ctx)
	} else {
		err = b.build(ctx)
	}
	if err != nil {
		fmt.Fprintf(os.Stderr, "iconpack: %v\n", err)
		os.Exit(1)
	}
}

// builder builds the packs; git and fetch reach outside, so a test replaces
// them.
type builder struct {
	pins, cache, out string
	log              io.Writer
	git              func(ctx context.Context, dir string, args ...string) error
	fetch            func(ctx context.Context, url string) ([]byte, error)
}

func (b builder) build(ctx context.Context) error {
	if b.cache == "" || b.out == "" {
		return fmt.Errorf("-cache and -out are required")
	}
	// #nosec G301 -- the image copies the folder as it is, and its nonroot user reads the packs.
	if err := os.MkdirAll(b.out, 0o755); err != nil {
		return err
	}
	glyphs, err := readLock(filepath.Join(b.pins, "package-lock.json"))
	if err != nil {
		return err
	}
	for _, g := range glyphs {
		if err := b.buildGlyphs(ctx, g); err != nil {
			return fmt.Errorf("%s: %w", g.catalog, err)
		}
	}
	logos, err := readSources(filepath.Join(b.pins, "sources.yaml"), true)
	if err != nil {
		return err
	}
	for _, s := range logos {
		if err := b.buildLogos(ctx, s); err != nil {
			return fmt.Errorf("%s: %w", s.Catalog, err)
		}
	}
	return b.checkRequired(filepath.Join(b.pins, "required.txt"))
}

func (b builder) printSums(ctx context.Context) error {
	if b.cache == "" {
		return fmt.Errorf("-cache is required")
	}
	logos, err := readSources(filepath.Join(b.pins, "sources.yaml"), false)
	if err != nil {
		return err
	}
	for _, s := range logos {
		dir, err := b.checkout(ctx, s)
		if err != nil {
			return fmt.Errorf("%s: %w", s.Catalog, err)
		}
		sum, err := treeSum(dir, s.Index)
		if err != nil {
			return err
		}
		fmt.Fprintf(b.log, "%s %s sha256: %s\n", s.Name, s.Commit, sum)
	}
	return nil
}
