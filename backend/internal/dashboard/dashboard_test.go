package dashboard

import (
	"strings"
	"testing"

	"github.com/stretchr/testify/assert"

	"github.com/guided-traffic/dash/backend/internal/badges"
	"github.com/guided-traffic/dash/backend/internal/config"
	"github.com/guided-traffic/dash/backend/internal/discovery"
	"github.com/guided-traffic/dash/backend/internal/icons"
)

// stubIcons names each item's icon after what it was asked for, so a test
// sees which reference and names reached the resolver.
type stubIcons struct{}

func (stubIcons) App(ref string, names []string) icons.Icon {
	return icons.Icon{Src: "app:" + ref + ":" + strings.Join(names, ",")}
}

func (stubIcons) Bookmark(ref string) icons.Icon { return icons.Icon{Src: "bookmark:" + ref} }

// stubBadges gives an item with own entries or an IngressClass one badge
// whose label names what it saw, and others none.
type stubBadges struct{}

func (stubBadges) Own(raw []string) ([]badges.Entry, []string) {
	out := make([]badges.Entry, 0, len(raw))
	for _, r := range raw {
		out = append(out, badges.Entry{Text: r, Corner: badges.TopLeft})
	}
	return out, nil
}

func (stubBadges) For(item badges.Item) []badges.Badge {
	if len(item.Own) == 0 && item.IngressClass == "" {
		return nil
	}
	label := item.Kind + " " + item.Host + " " + item.Namespace + " " + item.IngressClass
	for _, e := range item.Own {
		label += " own:" + e.Text
	}
	return []badges.Badge{{Corner: badges.TopRight, Label: label}}
}

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
	}, f, stubIcons{}, stubBadges{})
	assert.Equal(t, []string{"home", "media", "tools"}, groupNames(d), "by name without a group order")
	assert.Equal(t, map[string][]string{
		"home":  {"app/1/0"},
		"media": {"ingress/media/jellyfin"},
		"tools": {"ingress/tools/argo", "app/0/0", "ingress/tools/zot"},
	}, ids(d), "apps by name, case aside")
	router := d.Apps[2].Apps[1]
	assert.Equal(t, App{
		ID: "app/0/0", Kind: KindApp, Name: "router", URL: "https://192.168.1.1", TargetBlank: true,
		Status: discovery.Status{State: discovery.Unknown}, Icon: icons.Icon{Src: "app::router"},
	}, router, "a custom app has no status; its info waits for ui.showAppInfo; its name is its auto-match name")
	argo := d.Apps[2].Apps[0]
	assert.Equal(t, KindIngress, argo.Kind)
	assert.Equal(t, icons.Icon{Src: "app::"}, argo.Icon, "a discovered app hands its reference and names to the resolver")
	assert.Equal(t, "tools", argo.Namespace)
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
	}, f, stubIcons{}, stubBadges{})
	assert.Equal(t, []string{"monitoring", "home", "Alpha", "alpha", "zeta"}, groupNames(d))
}

func TestAppsOfTheSameNameKeepAStableOrder(t *testing.T) {
	d := Build([]discovery.App{
		found("b", "web", "Web", "g", discovery.Healthy),
		found("a", "web", "Web", "g", discovery.Healthy),
		found("c", "web", "web", "g", discovery.Healthy),
	}, config.DefaultFile(), stubIcons{}, stubBadges{})
	assert.Equal(t, []string{"ingress/a/web", "ingress/b/web", "ingress/c/web"}, ids(d)["g"])
}

func TestWhatTheConfigurationHidesIsNotServed(t *testing.T) {
	apps := []discovery.App{found("media", "jellyfin", "Jellyfin", "media", discovery.Down)}
	f := config.DefaultFile()
	f.Bookmarks = []config.BookmarkGroup{{Group: "docs", Bookmarks: []config.Bookmark{{Name: "K8s", URL: "https://kubernetes.io"}}}}

	d := Build(apps, f, stubIcons{}, stubBadges{})
	assert.Empty(t, d.Apps[0].Apps[0].Info, "ui.showAppInfo is off by default")
	assert.Equal(t, discovery.Down, d.Apps[0].Apps[0].Status.State)

	f.UI.ShowAppInfo = true
	f.UI.ShowAppStatus = false
	d = Build(apps, f, stubIcons{}, stubBadges{})
	assert.Equal(t, "about Jellyfin", d.Apps[0].Apps[0].Info)
	assert.Equal(t, discovery.Status{State: discovery.Unknown}, d.Apps[0].Apps[0].Status)

	f.UI.ShowApps = false
	f.UI.ShowBookmarks = false
	d = Build(apps, f, stubIcons{}, stubBadges{})
	assert.Empty(t, d.Apps)
	assert.Empty(t, d.Bookmarks)
}

func TestBookmarksKeepTheConfigurationsOrder(t *testing.T) {
	f := config.DefaultFile()
	f.Bookmarks = []config.BookmarkGroup{
		{Group: "zeta", Bookmarks: []config.Bookmark{{Name: "B", URL: "https://b"}, {Name: "A", URL: "https://a", Icon: "si:a", TargetBlank: true}}},
		{Group: "alpha", Bookmarks: []config.Bookmark{{Name: "C", URL: "https://c"}}},
	}
	d := Build(nil, f, stubIcons{}, stubBadges{})
	assert.Equal(t, []BookmarkGroup{
		{Name: "zeta", Bookmarks: []Bookmark{
			{ID: "bookmark/0/0", Name: "B", URL: "https://b", Icon: icons.Icon{Src: "bookmark:"}},
			{ID: "bookmark/0/1", Name: "A", URL: "https://a", TargetBlank: true, Icon: icons.Icon{Src: "bookmark:si:a"}},
		}},
		{Name: "alpha", Bookmarks: []Bookmark{{ID: "bookmark/1/0", Name: "C", URL: "https://c", Icon: icons.Icon{Src: "bookmark:"}}}},
	}, d.Bookmarks)
}

func TestAlwaysTargetBlankWins(t *testing.T) {
	f := config.DefaultFile()
	f.UI.AlwaysTargetBlank = true
	f.Apps = []config.AppGroup{{Group: "g", Apps: []config.App{{Name: "a", URL: "https://a"}}}}
	f.Bookmarks = []config.BookmarkGroup{{Group: "g", Bookmarks: []config.Bookmark{{Name: "b", URL: "https://b"}}}}
	d := Build([]discovery.App{found("n", "i", "x", "g", discovery.Healthy)}, f, stubIcons{}, stubBadges{})
	for _, a := range d.Apps[0].Apps {
		assert.True(t, a.TargetBlank, a.ID)
	}
	assert.True(t, d.Bookmarks[0].Bookmarks[0].TargetBlank)
}

func TestTheScopeNamesWhatTheDiscoveryReads(t *testing.T) {
	f := config.DefaultFile()
	d := Build(nil, f, stubIcons{}, stubBadges{})
	assert.Equal(t, Scope{Enabled: true, Any: true}, d.Scope)
	assert.Equal(t, f.UI, d.UI)

	f.Discovery.InstanceName = "home"
	f.Discovery.NamespaceSelector = config.NamespaceSelector{
		MatchNames:    []string{"static"},
		LabelSelector: config.LabelSelector{MatchLabels: map[string]string{"dash": "listed"}},
	}
	assert.Equal(t, Scope{Enabled: true, InstanceName: "home", Names: []string{"static"}, LabelSelector: "dash=listed"}, Build(nil, f, stubIcons{}, stubBadges{}).Scope)

	f.Discovery.Enabled = false
	assert.False(t, Build(nil, f, stubIcons{}, stubBadges{}).Scope.Enabled)
}

// Each item hands the badge engine what its rules match on: the kind, the
// host of its URL, and of an Ingress its namespace, class and own entries.
func TestEveryItemGetsItsBadges(t *testing.T) {
	f := config.DefaultFile()
	f.Apps = []config.AppGroup{{Group: "g", Apps: []config.App{{Name: "a", URL: "https://router.home.arpa:8443/x", Badges: []string{"beta"}}}}}
	f.Bookmarks = []config.BookmarkGroup{{Group: "g", Bookmarks: []config.Bookmark{{Name: "b", URL: "https://kubernetes.io", Badges: []string{"text:docs"}}}}}
	app := found("media", "grafana", "Grafana", "g", discovery.Healthy)
	app.IngressClass = "internal"
	app.OwnBadges = []badges.Entry{{Text: "new"}}
	d := Build([]discovery.App{app}, f, stubIcons{}, stubBadges{})
	labels := map[string]string{}
	for _, a := range d.Apps[0].Apps {
		labels[a.ID] = a.Badges[0].Label
	}
	assert.Equal(t, map[string]string{
		"app/0/0":               "app router.home.arpa   own:beta",
		"ingress/media/grafana": "ingress grafana.example.com media internal own:new",
	}, labels)
	assert.Equal(t, "bookmark kubernetes.io   own:text:docs", d.Bookmarks[0].Bookmarks[0].Badges[0].Label)
	assert.Empty(t, hostOf("::"), "a URL that does not parse has no host")
}

func TestAnEmptyDashboard(t *testing.T) {
	d := Build(nil, config.DefaultFile(), stubIcons{}, stubBadges{})
	assert.Empty(t, d.Apps)
	assert.Empty(t, d.Bookmarks)
}
