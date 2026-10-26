package dashboard

import (
	"testing"

	"github.com/stretchr/testify/assert"

	"github.com/guided-traffic/dash/backend/internal/config"
	"github.com/guided-traffic/dash/backend/internal/discovery"
)

func found(ns, ingress, name, group string, state discovery.State) discovery.App {
	return discovery.App{
		Namespace: ns, Ingress: ingress, Name: name, Group: group, URL: "https://" + ingress + ".example.com/",
		Info: "about " + name, Status: discovery.Status{State: state},
	}
}

// ids are the group names and their apps' IDs, in order.
func ids(d Dashboard) map[string][]string {
	out := map[string][]string{}
	for _, g := range d.Apps {
		for _, a := range g.Apps {
			out[g.Name] = append(out[g.Name], a.ID)
		}
	}
	return out
}

func groupNames(d Dashboard) []string {
	out := make([]string, 0, len(d.Apps))
	for _, g := range d.Apps {
		out = append(out, g.Name)
	}
	return out
}

func TestDiscoveredAndCustomAppsShareTheirGroup(t *testing.T) {
	f := config.DefaultFile()
	f.Apps = []config.AppGroup{
		{Group: "tools", Apps: []config.App{{Name: "router", URL: "https://192.168.1.1", Info: "edge", TargetBlank: true}}},
		{Group: "home", Apps: []config.App{{Name: "NAS", URL: "https://nas.home.arpa"}}},
	}
	d := Build([]discovery.App{
		found("tools", "argo", "Argo", "tools", discovery.Healthy),
		found("tools", "zot", "Zot", "tools", discovery.Down),
		found("media", "jellyfin", "Jellyfin", "media", discovery.Degraded),
	}, f)
	assert.Equal(t, []string{"home", "media", "tools"}, groupNames(d), "by name without a group order")
	assert.Equal(t, map[string][]string{
		"home":  {"app/1/0"},
		"media": {"ingress/media/jellyfin"},
		"tools": {"ingress/tools/argo", "app/0/0", "ingress/tools/zot"},
	}, ids(d), "apps by name, case aside")
	router := d.Apps[2].Apps[1]
	assert.Equal(t, App{
		ID: "app/0/0", Kind: KindApp, Name: "router", URL: "https://192.168.1.1", TargetBlank: true,
		Status: discovery.Status{State: discovery.Unknown},
	}, router, "a custom app has no status; its info waits for ui.showAppInfo")
	argo := d.Apps[2].Apps[0]
	assert.Equal(t, KindIngress, argo.Kind)
	assert.Equal(t, "tools", argo.Namespace)
	assert.Equal(t, Summary{Total: 5, Healthy: 1, Degraded: 1, Down: 1, Unknown: 2}, d.Summary)
}

func TestGroupOrderComesFirst(t *testing.T) {
	f := config.DefaultFile()
	f.UI.GroupOrder = []string{"monitoring", "absent", "home"}
	d := Build([]discovery.App{
		found("a", "1", "x", "zeta", discovery.Healthy),
		found("a", "2", "x", "home", discovery.Healthy),
		found("a", "3", "x", "Alpha", discovery.Healthy),
		found("a", "4", "x", "monitoring", discovery.Healthy),
		found("a", "5", "x", "alpha", discovery.Healthy),
	}, f)
	assert.Equal(t, []string{"monitoring", "home", "Alpha", "alpha", "zeta"}, groupNames(d))
}

func TestAppsOfTheSameNameKeepAStableOrder(t *testing.T) {
	d := Build([]discovery.App{
		found("b", "web", "Web", "g", discovery.Healthy),
		found("a", "web", "Web", "g", discovery.Healthy),
		found("c", "web", "web", "g", discovery.Healthy),
	}, config.DefaultFile())
	assert.Equal(t, []string{"ingress/a/web", "ingress/b/web", "ingress/c/web"}, ids(d)["g"])
}

func TestWhatTheConfigurationHidesIsNotServed(t *testing.T) {
	apps := []discovery.App{found("media", "jellyfin", "Jellyfin", "media", discovery.Down)}
	f := config.DefaultFile()
	f.Bookmarks = []config.BookmarkGroup{{Group: "docs", Bookmarks: []config.Bookmark{{Name: "K8s", URL: "https://kubernetes.io"}}}}

	d := Build(apps, f)
	assert.Empty(t, d.Apps[0].Apps[0].Info, "ui.showAppInfo is off by default")
	assert.Equal(t, discovery.Down, d.Apps[0].Apps[0].Status.State)

	f.UI.ShowAppInfo = true
	f.UI.ShowAppStatus = false
	d = Build(apps, f)
	assert.Equal(t, "about Jellyfin", d.Apps[0].Apps[0].Info)
	assert.Equal(t, discovery.Status{State: discovery.Unknown}, d.Apps[0].Apps[0].Status)
	assert.Equal(t, Summary{Total: 1, Unknown: 1}, d.Summary)

	f.UI.ShowApps = false
	f.UI.ShowBookmarks = false
	d = Build(apps, f)
	assert.Empty(t, d.Apps)
	assert.Empty(t, d.Bookmarks)
	assert.Equal(t, Summary{}, d.Summary)
}

func TestBookmarksKeepTheConfigurationsOrder(t *testing.T) {
	f := config.DefaultFile()
	f.Bookmarks = []config.BookmarkGroup{
		{Group: "zeta", Bookmarks: []config.Bookmark{{Name: "B", URL: "https://b"}, {Name: "A", URL: "https://a", TargetBlank: true}}},
		{Group: "alpha", Bookmarks: []config.Bookmark{{Name: "C", URL: "https://c"}}},
	}
	d := Build(nil, f)
	assert.Equal(t, []BookmarkGroup{
		{Name: "zeta", Bookmarks: []Bookmark{
			{ID: "bookmark/0/0", Name: "B", URL: "https://b"},
			{ID: "bookmark/0/1", Name: "A", URL: "https://a", TargetBlank: true},
		}},
		{Name: "alpha", Bookmarks: []Bookmark{{ID: "bookmark/1/0", Name: "C", URL: "https://c"}}},
	}, d.Bookmarks)
}

func TestAlwaysTargetBlankWins(t *testing.T) {
	f := config.DefaultFile()
	f.UI.AlwaysTargetBlank = true
	f.Apps = []config.AppGroup{{Group: "g", Apps: []config.App{{Name: "a", URL: "https://a"}}}}
	f.Bookmarks = []config.BookmarkGroup{{Group: "g", Bookmarks: []config.Bookmark{{Name: "b", URL: "https://b"}}}}
	d := Build([]discovery.App{found("n", "i", "x", "g", discovery.Healthy)}, f)
	for _, a := range d.Apps[0].Apps {
		assert.True(t, a.TargetBlank, a.ID)
	}
	assert.True(t, d.Bookmarks[0].Bookmarks[0].TargetBlank)
}

func TestAnEmptyDashboard(t *testing.T) {
	d := Build(nil, config.DefaultFile())
	assert.Empty(t, d.Apps)
	assert.Empty(t, d.Bookmarks)
	assert.Equal(t, Summary{}, d.Summary)
}
