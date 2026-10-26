package api

import (
	"context"
	"errors"
	"net/http"
	"strings"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/guided-traffic/dash/backend/internal/badges"
	"github.com/guided-traffic/dash/backend/internal/config"
	"github.com/guided-traffic/dash/backend/internal/dashboard"
	"github.com/guided-traffic/dash/backend/internal/discovery"
	"github.com/guided-traffic/dash/backend/internal/icons"
	"github.com/guided-traffic/dash/backend/internal/problem"
)

// stubIcons gives every item with a reference a tinted glyph, every other
// a logo with a dark variant, and serves one SVG.
type stubIcons struct{}

func (stubIcons) App(ref string, _ []string) icons.Icon {
	if ref != "" {
		return icons.Icon{Src: "/api/v1/icons/mdi@1.2.3/web.svg", Tint: true}
	}
	return icons.Icon{Src: "/api/v1/icons/sh@abc/grafana.svg", SrcDark: "/api/v1/icons/sh@abc/grafana-light.svg"}
}

func (stubIcons) Bookmark(string) icons.Icon {
	return icons.Icon{Src: "/api/v1/icons/material-symbols@1.2.96/bookmark.svg", Tint: true}
}

// The kept icons the stub serves.
const (
	blobPNG = "0000000000000000000000000000000000000000000000000000000000000001.png"
	blobSVG = "0000000000000000000000000000000000000000000000000000000000000002.svg"
)

func (stubIcons) Blob(file string) ([]byte, string, error) {
	switch file {
	case blobPNG:
		return []byte("\x89PNG\r\n\x1a\n"), "image/png", nil
	case blobSVG:
		return []byte(`<svg xmlns="http://www.w3.org/2000/svg"/>`), "image/svg+xml", nil
	}
	return nil, "", icons.ErrNoIcon
}

func (stubIcons) CatalogIcon(pack, file string) ([]byte, error) {
	switch {
	case pack == "mdi@1.2.3" && file == "web.svg":
		return []byte(`<svg xmlns="http://www.w3.org/2000/svg"/>`), nil
	case pack == "mdi@1.2.3" && file == "broken.svg":
		return nil, errors.New("read web from mdi@1.2.3: zip: checksum error")
	}
	return nil, icons.ErrNoIcon
}

// Glyph resolves every badge glyph to the material-symbols home.
func (stubIcons) Glyph(string) (icons.Icon, error) {
	return icons.Icon{Src: "/api/v1/icons/material-symbols@1.2.96/home.svg", Tint: true}, nil
}

// noBadges is an engine without definitions or rules.
var noBadges = func() *badges.Engine {
	e, err := badges.New(nil, nil, stubIcons{})
	if err != nil {
		panic(err)
	}
	return e
}()

// withDashboard is the API, every response held to the document, answering
// the dashboard source gives.
func withDashboard(t *testing.T, source func(context.Context) (dashboard.Dashboard, error)) http.Handler {
	t.Helper()
	h, err := New(Options{Version: "1.2.3", ValidateResponses: true, Dashboard: source})
	require.NoError(t, err)
	return h
}

func TestGetDashboardAnswersThePage(t *testing.T) {
	f := config.DefaultFile()
	f.UI.ShowAppInfo = true
	f.UI.GroupOrder = []string{"tools"}
	f.Apps = []config.AppGroup{{Group: "tools", Apps: []config.App{{Name: "Router", URL: "https://192.168.1.1", Icon: "mdi:web", TargetBlank: true}}}}
	f.Bookmarks = []config.BookmarkGroup{{Group: "docs", Bookmarks: []config.Bookmark{{Name: "Kubernetes", URL: "https://kubernetes.io/docs"}}}}
	engine, err := badges.New(map[string]badges.Definition{
		"home": {Icon: "material-symbols:home", Label: "local network only"},
		"beta": {Text: "beta", Label: "beta release", Tone: "amber"},
	}, []badges.Rule{{Match: badges.Match{IngressClassNames: []string{"internal"}}, Badge: "home"}}, stubIcons{})
	require.NoError(t, err)
	f.Apps[0].Apps[0].Badges = []string{"beta@bottom-left"}
	d := dashboard.Build([]discovery.App{{
		Namespace: "monitoring", Ingress: "grafana", Name: "Grafana", Group: "monitoring",
		URL: "https://grafana.example.com/", Info: "Dashboards", IngressClass: "internal",
		Status: discovery.Status{State: discovery.Degraded, Ready: 1, Total: 2},
	}}, f, stubIcons{}, engine)
	h := withDashboard(t, func(context.Context) (dashboard.Dashboard, error) { return d, nil })

	rec := get(t, h, http.MethodGet, "/api/v1/dashboard")
	require.Equal(t, http.StatusOK, rec.Code, rec.Body.String())
	assert.Equal(t, "no-store", rec.Header().Get("Cache-Control"))
	assert.JSONEq(t, `{
		"ui": {
			"title": "Dash", "greeting": {"enabled": true, "name": "You"}, "default_theme": "system",
			"refresh_seconds": 60, "show_summary": true, "show_app_groups": true, "show_app_urls": true,
			"show_bookmark_groups": true
		},
		"discovery": {"enabled": true, "instance_name": "", "namespaces": {"any": true, "names": [], "label_selector": ""}},
		"summary": {"total": 2, "healthy": 0, "degraded": 1, "down": 0, "unknown": 1},
		"groups": [
			{"name": "tools", "apps": [
				{"id": "app/0/0", "kind": "app", "name": "Router", "url": "https://192.168.1.1", "target_blank": true,
				 "status": {"state": "unknown", "ready": 0, "total": 0},
				 "icon": {"src": "/api/v1/icons/mdi@1.2.3/web.svg", "tint": true},
				 "badges": [{"corner": "bottom-left", "text": "beta", "label": "beta release", "tone": "amber"}]}
			]},
			{"name": "monitoring", "apps": [
				{"id": "ingress/monitoring/grafana", "kind": "ingress", "name": "Grafana", "url": "https://grafana.example.com/",
				 "info": "Dashboards", "target_blank": false, "status": {"state": "degraded", "ready": 1, "total": 2},
				 "icon": {"src": "/api/v1/icons/sh@abc/grafana.svg", "src_dark": "/api/v1/icons/sh@abc/grafana-light.svg", "tint": false},
				 "badges": [{"corner": "top-right", "icon": {"src": "/api/v1/icons/material-symbols@1.2.96/home.svg", "tint": true},
				   "label": "local network only", "tone": "neutral"}]}
			]}
		],
		"bookmarks": [
			{"name": "docs", "bookmarks": [{"id": "bookmark/0/0", "name": "Kubernetes", "url": "https://kubernetes.io/docs", "target_blank": false,
			  "icon": {"src": "/api/v1/icons/material-symbols@1.2.96/bookmark.svg", "tint": true}, "badges": []}]}
		]
	}`, rec.Body.String())
}

// Every list is a list: the page iterates without checking for null.
func TestAnEmptyDashboardHasEmptyLists(t *testing.T) {
	f := config.DefaultFile()
	f.Discovery.NamespaceSelector = config.NamespaceSelector{}
	f.Discovery.Enabled = false
	h := withDashboard(t, func(context.Context) (dashboard.Dashboard, error) {
		return dashboard.Build(nil, f, stubIcons{}, noBadges), nil
	})
	rec := get(t, h, http.MethodGet, "/api/v1/dashboard")
	require.Equal(t, http.StatusOK, rec.Code, rec.Body.String())
	assert.Contains(t, rec.Body.String(), `"groups":[]`)
	assert.Contains(t, rec.Body.String(), `"bookmarks":[]`)
	assert.Contains(t, rec.Body.String(), `"names":[]`)
}

func TestGetDashboardIsNotReadyUntilTheDiscoveryListed(t *testing.T) {
	h := withDashboard(t, func(context.Context) (dashboard.Dashboard, error) {
		return dashboard.Dashboard{}, problem.New(problem.NotReady, "the discovery has not listed the Ingresses yet")
	})
	rec := get(t, h, http.MethodGet, "/api/v1/dashboard")
	require.Equal(t, http.StatusServiceUnavailable, rec.Code, rec.Body.String())
	body := problemOf(t, rec)
	assert.Equal(t, "not_ready", body.Code)
	assert.Equal(t, "the discovery has not listed the Ingresses yet", body.Detail)
}

func TestGetDashboardWithoutASourceIsNotReady(t *testing.T) {
	rec := get(t, withDashboard(t, nil), http.MethodGet, "/api/v1/dashboard")
	require.Equal(t, http.StatusServiceUnavailable, rec.Code, rec.Body.String())
	assert.Equal(t, "not_ready", problemOf(t, rec).Code)
}

func TestAFailingSourceIsAnInternalError(t *testing.T) {
	h := withDashboard(t, func(context.Context) (dashboard.Dashboard, error) {
		return dashboard.Dashboard{}, errors.New("boom at 10.0.0.1")
	})
	rec := get(t, h, http.MethodGet, "/api/v1/dashboard")
	require.Equal(t, http.StatusInternalServerError, rec.Code)
	body := problemOf(t, rec)
	assert.Equal(t, "internal", body.Code)
	assert.NotContains(t, rec.Body.String(), "10.0.0.1", "the cause goes to the log only")
}

func TestGetDashboardRefusesAnUnknownParameter(t *testing.T) {
	h := withDashboard(t, func(context.Context) (dashboard.Dashboard, error) {
		return dashboard.Build(nil, config.DefaultFile(), stubIcons{}, noBadges), nil
	})
	rec := get(t, h, http.MethodGet, "/api/v1/dashboard?namespace=kube-system")
	require.Equal(t, http.StatusBadRequest, rec.Code)
	assert.Equal(t, "validation_failed", problemOf(t, rec).Code)
}

// An icon is immutable at its address, and runs nothing opened on its own.
func TestGetCatalogIcon(t *testing.T) {
	h := newAPI(t)
	rec := get(t, h, http.MethodGet, "/api/v1/icons/mdi@1.2.3/web.svg")
	require.Equal(t, http.StatusOK, rec.Code, rec.Body.String())
	assert.Equal(t, "image/svg+xml", rec.Header().Get("Content-Type"))
	assert.Equal(t, "public, max-age=31536000, immutable", rec.Header().Get("Cache-Control"))
	assert.Equal(t, "default-src 'none'; style-src 'unsafe-inline'; sandbox", rec.Header().Get("Content-Security-Policy"))
	assert.Equal(t, "nosniff", rec.Header().Get("X-Content-Type-Options"))
	assert.Equal(t, `<svg xmlns="http://www.w3.org/2000/svg"/>`, rec.Body.String())

	for _, path := range []string{
		"/api/v1/icons/mdi@1.2.2/web.svg",
		"/api/v1/icons/mdi@1.2.3/nothing.svg",
		"/api/v1/icons/mdi/web.svg",
		"/api/v1/icons/mdi@1.2.3/web.png",
		"/api/v1/icons/mdi@1.2.3/..%2Fweb.svg",
		"/api/v1/icons/MDI@1/web.svg",
	} {
		rec := get(t, h, http.MethodGet, path)
		assert.Equal(t, http.StatusNotFound, rec.Code, path)
		assert.Equal(t, "not_found", problemOf(t, rec).Code, path)
		assert.Equal(t, "no-store", rec.Header().Get("Cache-Control"), "a missing icon is not cached: %s", path)
	}

	rec = get(t, h, http.MethodGet, "/api/v1/icons/mdi@1.2.3/broken.svg")
	assert.Equal(t, http.StatusInternalServerError, rec.Code)
	assert.NotContains(t, rec.Body.String(), "checksum", "the cause goes to the log only")

	h, err := New(Options{ValidateResponses: true})
	require.NoError(t, err)
	assert.Equal(t, http.StatusNotFound, get(t, h, http.MethodGet, "/api/v1/icons/mdi@1.2.3/web.svg").Code, "without a source")
}

func TestGetBlobIcon(t *testing.T) {
	h := newAPI(t)
	for file, contentType := range map[string]string{blobPNG: "image/png", blobSVG: "image/svg+xml"} {
		rec := get(t, h, http.MethodGet, "/api/v1/icons/blob/"+file)
		require.Equal(t, http.StatusOK, rec.Code, rec.Body.String())
		assert.Equal(t, contentType, rec.Header().Get("Content-Type"))
		assert.Equal(t, "public, max-age=31536000, immutable", rec.Header().Get("Cache-Control"))
		assert.Equal(t, "default-src 'none'; style-src 'unsafe-inline'; sandbox", rec.Header().Get("Content-Security-Policy"))
		assert.Equal(t, "nosniff", rec.Header().Get("X-Content-Type-Options"))
	}
	for _, path := range []string{
		"/api/v1/icons/blob/" + strings.Repeat("f", 64) + ".png",
		"/api/v1/icons/blob/" + strings.Repeat("f", 64) + ".gif",
		"/api/v1/icons/blob/x.png",
	} {
		rec := get(t, h, http.MethodGet, path)
		assert.Equal(t, http.StatusNotFound, rec.Code, path)
	}
	h, err := New(Options{ValidateResponses: true})
	require.NoError(t, err)
	assert.Equal(t, http.StatusNotFound, get(t, h, http.MethodGet, "/api/v1/icons/blob/"+blobPNG).Code, "without a source")
}
