package api

import (
	"context"

	"github.com/guided-traffic/dash/backend/internal/api/apigen"
	"github.com/guided-traffic/dash/backend/internal/badges"
	"github.com/guided-traffic/dash/backend/internal/dashboard"
	"github.com/guided-traffic/dash/backend/internal/problem"
)

// GetDashboard answers the page's content.
func (s *server) GetDashboard(ctx context.Context, _ apigen.GetDashboardRequestObject) (apigen.GetDashboardResponseObject, error) {
	if s.h.opts.Dashboard == nil {
		return nil, problem.New(problem.NotReady, "the dashboard is not set up")
	}
	d, err := s.h.opts.Dashboard(ctx)
	if err != nil {
		return nil, err
	}
	return apigen.GetDashboard200JSONResponse(toDashboard(d)), nil
}

// toDashboard is the page's content in the API's shape: every list a list,
// never null.
func toDashboard(d dashboard.Dashboard) apigen.Dashboard {
	out := apigen.Dashboard{
		Ui: apigen.DashboardUI{
			Title:              d.UI.Title,
			Greeting:           apigen.Greeting{Enabled: d.UI.Greeting.Enabled, Name: d.UI.Greeting.Name},
			DefaultTheme:       apigen.DashboardUIDefaultTheme(d.UI.DefaultTheme),
			RefreshSeconds:     d.UI.RefreshSeconds,
			ShowSummary:        d.UI.ShowSummary,
			ShowAppGroups:      d.UI.ShowAppGroups,
			ShowAppUrls:        d.UI.ShowAppURLs,
			ShowBookmarkGroups: d.UI.ShowBookmarkGroups,
		},
		Discovery: apigen.DiscoveryScope{
			Enabled:      d.Scope.Enabled,
			InstanceName: d.Scope.InstanceName,
			Namespaces: apigen.NamespaceScope{
				Any:           d.Scope.Any,
				Names:         orEmpty(d.Scope.Names),
				LabelSelector: d.Scope.LabelSelector,
			},
		},
		Summary: apigen.Summary{
			Total: d.Summary.Total, Healthy: d.Summary.Healthy, Degraded: d.Summary.Degraded,
			Down: d.Summary.Down, Unknown: d.Summary.Unknown,
		},
		Groups:    make([]apigen.AppGroup, 0, len(d.Apps)),
		Bookmarks: make([]apigen.BookmarkGroup, 0, len(d.Bookmarks)),
	}
	for _, g := range d.Apps {
		group := apigen.AppGroup{Name: g.Name, Apps: make([]apigen.App, 0, len(g.Apps))}
		for _, a := range g.Apps {
			group.Apps = append(group.Apps, toApp(a))
		}
		out.Groups = append(out.Groups, group)
	}
	for _, g := range d.Bookmarks {
		group := apigen.BookmarkGroup{Name: g.Name, Bookmarks: make([]apigen.Bookmark, 0, len(g.Bookmarks))}
		for _, b := range g.Bookmarks {
			group.Bookmarks = append(group.Bookmarks, apigen.Bookmark{
				Id: b.ID, Name: b.Name, Url: b.URL, TargetBlank: b.TargetBlank, Icon: toIcon(b.Icon), Badges: toBadges(b.Badges),
			})
		}
		out.Bookmarks = append(out.Bookmarks, group)
	}
	return out
}

func toApp(a dashboard.App) apigen.App {
	app := apigen.App{
		Id:          a.ID,
		Kind:        apigen.AppKind(a.Kind),
		Name:        a.Name,
		Url:         a.URL,
		TargetBlank: a.TargetBlank,
		Icon:        toIcon(a.Icon),
		Badges:      toBadges(a.Badges),
		Status: apigen.AppStatus{
			State: apigen.AppStatusState(a.Status.State),
			Ready: a.Status.Ready,
			Total: a.Status.Total,
		},
	}
	if a.Info != "" {
		app.Info = &a.Info
	}
	return app
}

// toBadges are an item's badges in the API's shape, a list even when empty.
func toBadges(in []badges.Badge) []apigen.Badge {
	out := make([]apigen.Badge, 0, len(in))
	for _, b := range in {
		badge := apigen.Badge{Corner: apigen.BadgeCorner(b.Corner), Label: b.Label, Tone: apigen.BadgeTone(b.Tone)}
		if b.Icon != nil {
			icon := toIcon(*b.Icon)
			badge.Icon = &icon
		}
		if b.Text != "" {
			text := b.Text
			badge.Text = &text
		}
		out = append(out, badge)
	}
	return out
}

func orEmpty(s []string) []string {
	if s == nil {
		return []string{}
	}
	return s
}
