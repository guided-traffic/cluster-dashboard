// Package dashboard puts the page together: the apps discovery found and the
// custom apps of the configuration in groups, in order; the bookmark groups;
// the summary of their states. What the configuration hides is left out
// here, not only in the page: the API serves what the page shows.
package dashboard

import (
	"fmt"
	"slices"
	"strings"

	"github.com/guided-traffic/dash/backend/internal/config"
	"github.com/guided-traffic/dash/backend/internal/discovery"
)

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
}

// Summary counts the apps served, by state.
type Summary struct {
	Total, Healthy, Degraded, Down, Unknown int
}

// Build assembles the page from the discovered apps and the configuration.
func Build(discovered []discovery.App, f config.File) Dashboard {
	var d Dashboard
	if f.UI.ShowApps {
		d.Apps = appGroups(discovered, f)
	}
	if f.UI.ShowBookmarks {
		d.Bookmarks = bookmarkGroups(f)
	}
	for _, g := range d.Apps {
		for _, a := range g.Apps {
			d.Summary.add(a.Status.State)
		}
	}
	return d
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

func appGroups(discovered []discovery.App, f config.File) []AppGroup {
	byName := map[string][]App{}
	for _, a := range discovered {
		app := App{
			ID: "ingress/" + a.Key(), Kind: KindIngress, Name: a.Name, URL: a.URL, Info: a.Info,
			TargetBlank: a.TargetBlank, Status: a.Status, Namespace: a.Namespace,
		}
		byName[a.Group] = append(byName[a.Group], app)
	}
	for gi, g := range f.Apps {
		for ai, a := range g.Apps {
			app := App{
				ID: fmt.Sprintf("app/%d/%d", gi, ai), Kind: KindApp, Name: a.Name, URL: a.URL, Info: a.Info,
				TargetBlank: a.TargetBlank, Status: discovery.Status{State: discovery.Unknown},
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

func bookmarkGroups(f config.File) []BookmarkGroup {
	groups := make([]BookmarkGroup, 0, len(f.Bookmarks))
	for gi, g := range f.Bookmarks {
		group := BookmarkGroup{Name: g.Group, Bookmarks: make([]Bookmark, 0, len(g.Bookmarks))}
		for bi, b := range g.Bookmarks {
			group.Bookmarks = append(group.Bookmarks, Bookmark{
				ID:          fmt.Sprintf("bookmark/%d/%d", gi, bi),
				Name:        b.Name,
				URL:         b.URL,
				TargetBlank: b.TargetBlank || f.UI.AlwaysTargetBlank,
			})
		}
		groups = append(groups, group)
	}
	return groups
}
