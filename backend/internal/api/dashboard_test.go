package api

import (
	"context"
	"errors"
	"net/http"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/guided-traffic/dash/backend/internal/config"
	"github.com/guided-traffic/dash/backend/internal/dashboard"
	"github.com/guided-traffic/dash/backend/internal/discovery"
	"github.com/guided-traffic/dash/backend/internal/problem"
)

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
	f.Apps = []config.AppGroup{{Group: "tools", Apps: []config.App{{Name: "Router", URL: "https://192.168.1.1", TargetBlank: true}}}}
	f.Bookmarks = []config.BookmarkGroup{{Group: "docs", Bookmarks: []config.Bookmark{{Name: "Kubernetes", URL: "https://kubernetes.io/docs"}}}}
	d := dashboard.Build([]discovery.App{{
		Namespace: "monitoring", Ingress: "grafana", Name: "Grafana", Group: "monitoring",
		URL: "https://grafana.example.com/", Info: "Dashboards",
		Status: discovery.Status{State: discovery.Degraded, Ready: 1, Total: 2},
	}}, f)
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
				 "status": {"state": "unknown", "ready": 0, "total": 0}}
			]},
			{"name": "monitoring", "apps": [
				{"id": "ingress/monitoring/grafana", "kind": "ingress", "name": "Grafana", "url": "https://grafana.example.com/",
				 "info": "Dashboards", "target_blank": false, "status": {"state": "degraded", "ready": 1, "total": 2}}
			]}
		],
		"bookmarks": [
			{"name": "docs", "bookmarks": [{"id": "bookmark/0/0", "name": "Kubernetes", "url": "https://kubernetes.io/docs", "target_blank": false}]}
		]
	}`, rec.Body.String())
}

// Every list is a list: the page iterates without checking for null.
func TestAnEmptyDashboardHasEmptyLists(t *testing.T) {
	f := config.DefaultFile()
	f.Discovery.NamespaceSelector = config.NamespaceSelector{}
	f.Discovery.Enabled = false
	h := withDashboard(t, func(context.Context) (dashboard.Dashboard, error) { return dashboard.Build(nil, f), nil })
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
		return dashboard.Build(nil, config.DefaultFile()), nil
	})
	rec := get(t, h, http.MethodGet, "/api/v1/dashboard?namespace=kube-system")
	require.Equal(t, http.StatusBadRequest, rec.Code)
	assert.Equal(t, "validation_failed", problemOf(t, rec).Code)
}
