// Package dashboard puts the page together: the apps discovery found and the
// custom apps of the configuration in groups, in order; the bookmark groups;
// the summary of their states. What the configuration hides is left out
// here, not only in the page: the API serves what the page shows.
package dashboard

import (
	"fmt"
	"net/url"
	"slices"
	"strings"

	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"

	"github.com/guided-traffic/dash/backend/internal/badges"
	"github.com/guided-traffic/dash/backend/internal/config"
	"github.com/guided-traffic/dash/backend/internal/discovery"
	"github.com/guided-traffic/dash/backend/internal/icons"
)

// Badges puts the badges on each item; badges.Engine does.
type Badges interface {
	// Own parses an item's own entries of the configuration.
	Own(raw []string) ([]badges.Entry, []string)
	// For is an item's badges.
	For(item badges.Item) []badges.Badge
}

// Icons resolves the icon of each item; icons.Resolver does, deciding every
// fallback.
type Icons interface {
	// App is an app's icon: its own reference, else the logo one of its
	// names matches, else the default.
	App(ref string, names []string) icons.Icon
	// Bookmark is a bookmark's icon: its own reference, else the default.
	Bookmark(ref string) icons.Icon
}

// Kind is where an item comes from; badge rules match on it
// (docs/adr/0011).
type Kind string

// The kinds of item.
const (
	KindIngress  Kind = config.KindIngress
	KindApp      Kind = config.KindApp
	KindBookmark Kind = config.KindBookmark
)

// Dashboard is the page's content.
type Dashboard struct {
	// UI are the settings the page applies itself.
	UI config.UI
	// Scope is what the discovery reads, for the page without an app.
	Scope Scope
	// Apps are the app groups: ui.groupOrder's first, in its order, then
	// the others by name. Empty while ui.showApps is off.
	Apps []AppGroup
	// Bookmarks are the bookmark groups in the configuration's order. Empty
	// while ui.showBookmarks is off.
	Bookmarks []BookmarkGroup
	// Summary counts the apps by state.
	Summary Summary
}

// AppGroup is a group of apps: a discovered app's dash.gtrfc.com/group or
// namespace, or a custom app group of that name — one group either way.
type AppGroup struct {
	Name string
	// Apps are ordered by name.
	Apps []App
}

// App is a tile.
type App struct {
	// ID is unique on the page and stable while the cluster and the
	// configuration do not change: ingress/<namespace>/<name> for a
	// discovered app, app/<group>/<index> for a custom one.
	ID   string
	Kind Kind
	Name string
	URL  string
	// Info is empty while ui.showAppInfo is off.
	Info string
	// TargetBlank is the app's own, or ui.alwaysTargetBlank.
	TargetBlank bool
	// Status is unknown for a custom app, and for every app while
	// ui.showAppStatus is off.
	Status discovery.Status
	// Icon is resolved: every app has one.
	Icon icons.Icon
	// Badges are its pills, one per corner, in the order of badges.Corners.
	Badges []badges.Badge
	// Namespace is a discovered app's, empty for a custom one.
	Namespace string
}

// BookmarkGroup is a group of bookmarks, as the configuration lists it.
type BookmarkGroup struct {
	Name      string
	Bookmarks []Bookmark
}

// Bookmark is a link.
type Bookmark struct {
	// ID is bookmark/<group>/<index>.
	ID          string
	Name        string
	URL         string
	TargetBlank bool
	Icon        icons.Icon
	Badges      []badges.Badge
}

// Scope is what the discovery reads: the empty page names it, so that a
// person can tell a selector that matches nothing from a cluster without
// apps.
type Scope struct {
	Enabled      bool
	InstanceName string
	Any          bool
	Names        []string
	// LabelSelector is the selector in its text form, such as dash=listed;
	// empty for none.
	LabelSelector string
}

// Summary counts the apps served, by state.
type Summary struct {
	Total, Healthy, Degraded, Down, Unknown int
}

// Build assembles the page from the discovered apps and the configuration,
// each item with its icon and its badges.
func Build(discovered []discovery.App, f config.File, ic Icons, bg Badges) Dashboard {
	d := Dashboard{UI: f.UI, Scope: scopeOf(f.Discovery)}
	if f.UI.ShowApps {
		d.Apps = appGroups(discovered, f, ic, bg)
	}
	if f.UI.ShowBookmarks {
		d.Bookmarks = bookmarkGroups(f, ic, bg)
	}
	for _, g := range d.Apps {
		for _, a := range g.Apps {
			d.Summary.add(a.Status.State)
		}
	}
	return d
}

// configured are the badges of a custom app or a bookmark: its own entries,
// which config.Load checked, then the rules.
func configured(bg Badges, kind, link string, entries []string) []badges.Badge {
	own, _ := bg.Own(entries)
	return bg.For(badges.Item{Kind: kind, Host: hostOf(link), Own: own})
}

// hostOf is a URL's host, without its port.
func hostOf(link string) string {
	u, err := url.Parse(link)
	if err != nil {
		return ""
	}
	return u.Hostname()
}

func scopeOf(c config.Discovery) Scope {
	sel := c.NamespaceSelector
	scope := Scope{Enabled: c.Enabled, InstanceName: c.InstanceName, Any: sel.Any, Names: slices.Clone(sel.MatchNames)}
	if !sel.Any && !sel.LabelSelector.IsEmpty() {
		// config.Load refused a selector that does not convert.
		if ls, err := metav1.LabelSelectorAsSelector(discovery.LabelSelector(sel.LabelSelector)); err == nil {
			scope.LabelSelector = ls.String()
		}
	}
	return scope
}

func (s *Summary) add(state discovery.State) {
	s.Total++
	switch state {
	case discovery.Healthy:
		s.Healthy++
	case discovery.Degraded:
		s.Degraded++
	case discovery.Down:
		s.Down++
	default:
		s.Unknown++
	}
}

func appGroups(discovered []discovery.App, f config.File, ic Icons, bg Badges) []AppGroup {
	byName := map[string][]App{}
	for _, a := range discovered {
		app := App{
			ID: "ingress/" + a.Key(), Kind: KindIngress, Name: a.Name, URL: a.URL, Info: a.Info,
			TargetBlank: a.TargetBlank, Status: a.Status, Namespace: a.Namespace, Icon: ic.App(a.Icon, a.Names),
			Badges: bg.For(badges.Item{
				Kind: config.KindIngress, Host: hostOf(a.URL), Namespace: a.Namespace, Labels: a.Labels,
				Annotations: a.Annotations, IngressClass: a.IngressClass, IngressController: a.IngressController,
				Own: a.OwnBadges,
			}),
		}
		byName[a.Group] = append(byName[a.Group], app)
	}
	for gi, g := range f.Apps {
		for ai, a := range g.Apps {
			app := App{
				ID: fmt.Sprintf("app/%d/%d", gi, ai), Kind: KindApp, Name: a.Name, URL: a.URL, Info: a.Info,
				TargetBlank: a.TargetBlank, Status: discovery.Status{State: discovery.Unknown},
				Icon:   ic.App(a.Icon, []string{a.Name}),
				Badges: configured(bg, config.KindApp, a.URL, a.Badges),
			}
			byName[g.Group] = append(byName[g.Group], app)
		}
	}
	groups := make([]AppGroup, 0, len(byName))
	for name, apps := range byName {
		for i := range apps {
			apps[i].TargetBlank = apps[i].TargetBlank || f.UI.AlwaysTargetBlank
			if !f.UI.ShowAppInfo {
				apps[i].Info = ""
			}
			if !f.UI.ShowAppStatus {
				apps[i].Status = discovery.Status{State: discovery.Unknown}
			}
		}
		slices.SortFunc(apps, func(a, b App) int { return byNameThen(a.Name, b.Name, a.ID, b.ID) })
		groups = append(groups, AppGroup{Name: name, Apps: apps})
	}
	slices.SortFunc(groups, groupOrder(f.UI.GroupOrder))
	return groups
}

// groupOrder orders groups: those ui.groupOrder names first, in its order,
// then the others by name.
func groupOrder(order []string) func(a, b AppGroup) int {
	rank := func(name string) int {
		if i := slices.Index(order, name); i >= 0 {
			return i
		}
		return len(order)
	}
	return func(a, b AppGroup) int {
		if ra, rb := rank(a.Name), rank(b.Name); ra != rb {
			return ra - rb
		}
		return byNameThen(a.Name, b.Name, "", "")
	}
}

// byNameThen compares names as a person reads them — case aside — then
// exactly, then by the tie-breakers.
func byNameThen(a, b, tieA, tieB string) int {
	if c := strings.Compare(strings.ToLower(a), strings.ToLower(b)); c != 0 {
		return c
	}
	if c := strings.Compare(a, b); c != 0 {
		return c
	}
	return strings.Compare(tieA, tieB)
}

func bookmarkGroups(f config.File, ic Icons, bg Badges) []BookmarkGroup {
	groups := make([]BookmarkGroup, 0, len(f.Bookmarks))
	for gi, g := range f.Bookmarks {
		group := BookmarkGroup{Name: g.Group, Bookmarks: make([]Bookmark, 0, len(g.Bookmarks))}
		for bi, b := range g.Bookmarks {
			group.Bookmarks = append(group.Bookmarks, Bookmark{
				ID:          fmt.Sprintf("bookmark/%d/%d", gi, bi),
				Name:        b.Name,
				URL:         b.URL,
				TargetBlank: b.TargetBlank || f.UI.AlwaysTargetBlank,
				Icon:        ic.Bookmark(b.Icon),
				Badges:      configured(bg, config.KindBookmark, b.URL, b.Badges),
			})
		}
		groups = append(groups, group)
	}
	return groups
}
