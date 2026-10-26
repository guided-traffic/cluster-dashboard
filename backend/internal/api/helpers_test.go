package api

import (
	"log/slog"
	"testing"

	apispec "github.com/guided-traffic/dash/backend/api"
)

func newDiscardLogger() *slog.Logger {
	return slog.New(slog.DiscardHandler)
}

func mustDocument(t *testing.T) []byte {
	t.Helper()
	if len(apispec.Document) == 0 {
		t.Fatal("the embedded API document is empty: run make generate")
	}
	return apispec.Document
}
