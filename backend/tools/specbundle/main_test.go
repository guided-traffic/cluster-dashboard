package main

import (
	"encoding/json"
	"os"
	"path/filepath"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// The bundle of the repository's own document is what make generate writes;
// the drift check compares the committed file with it, and this test proves
// the bundling itself: every reference internalised, the document valid.
func TestTheRepositoryDocumentBundles(t *testing.T) {
	b, err := bundle(filepath.Join("..", "..", rootFile))
	require.NoError(t, err)
	var doc map[string]any
	require.NoError(t, json.Unmarshal(b, &doc))
	assert.Equal(t, "3.1.0", doc["openapi"])
	assert.NotContains(t, string(b), ".yaml#", "no external reference is left")
	committed, err := os.ReadFile(filepath.Join("..", "..", bundledFile))
	require.NoError(t, err)
	assert.JSONEq(t, string(committed), string(b), "api/openapi.gen.json is current: run make generate")
}

func TestAnInvalidDocumentIsRefused(t *testing.T) {
	dir := t.TempDir()
	root := filepath.Join(dir, "openapi.yaml")
	require.NoError(t, os.WriteFile(root, []byte("openapi: 3.1.0\ninfo: {title: x}\npaths: {}\n"), 0o600))
	_, err := bundle(root)
	assert.Error(t, err, "info.version is required")
}

func TestAMissingDocumentIsRefused(t *testing.T) {
	_, err := bundle(filepath.Join(t.TempDir(), "absent.yaml"))
	assert.Error(t, err)
}

func TestRunWritesTheBundle(t *testing.T) {
	dir := t.TempDir()
	root := filepath.Join(dir, "openapi.yaml")
	require.NoError(t, os.WriteFile(root, []byte("openapi: 3.1.0\ninfo: {title: x, version: '1'}\npaths: {}\n"), 0o600))
	out := filepath.Join(dir, "out.json")
	require.NoError(t, run(root, out))
	b, err := os.ReadFile(out)
	require.NoError(t, err)
	assert.Contains(t, string(b), `"title": "x"`)
	assert.Error(t, run(root, filepath.Join(dir, "missing-dir", "out.json")))
}
