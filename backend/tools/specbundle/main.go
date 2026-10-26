// Command specbundle bundles the API document, split per path family
// (docs/adr/0001 D5), into the one JSON document the backend embeds and
// serves and the generators read: every external reference is internalised
// under the last segment of its JSON pointer, and the result is validated.
//
// It reads api/openapi.yaml and writes api/openapi.gen.json, relative to
// backend/, where `make generate` runs it.
package main

import (
	"context"
	"encoding/json"
	"fmt"
	"os"
	"strings"

	"github.com/getkin/kin-openapi/openapi3"
)

const (
	rootFile    = "api/openapi.yaml"
	bundledFile = "api/openapi.gen.json"
)

func main() {
	if err := run(rootFile, bundledFile); err != nil {
		fmt.Fprintf(os.Stderr, "specbundle: %v\n", err)
		os.Exit(1)
	}
}

func run(root, out string) error {
	b, err := bundle(root)
	if err != nil {
		return err
	}
	if err := os.WriteFile(out, b, 0o600); err != nil {
		return fmt.Errorf("write %s: %w", out, err)
	}
	return nil
}

// bundle loads root with its references, internalises them, validates the
// result and returns it as indented JSON with a final newline.
func bundle(root string) ([]byte, error) {
	ctx := context.Background()
	loader := openapi3.NewLoader()
	loader.IsExternalRefsAllowed = true
	doc, err := loader.LoadFromFile(root)
	if err != nil {
		return nil, fmt.Errorf("load %s: %w", root, err)
	}
	doc.InternalizeRefs(ctx, func(_ *openapi3.T, ref openapi3.ComponentRef) string {
		s := ref.RefString()
		return s[strings.LastIndex(s, "/")+1:]
	})
	if err := doc.Validate(ctx); err != nil {
		return nil, fmt.Errorf("validate the bundled document: %w", err)
	}
	b, err := json.MarshalIndent(doc, "", "  ")
	if err != nil {
		return nil, fmt.Errorf("encode: %w", err)
	}
	return append(b, '\n'), nil
}
