import { ChangeDetectionStrategy, Component, computed, effect, inject } from '@angular/core';
import { App } from '../../api/models/app';
import { BookmarkGroup } from '../../api/models/bookmark-group';
import { Dashboard } from '../../api/models/dashboard';
import { DiscoveryScope } from '../../api/models/discovery-scope';
import { DashboardService, retrySeconds } from '../../core/dashboard.service';
import { ThemeService } from '../../core/theme.service';
import { Bracket } from '../../shared/bracket/bracket';
import { Bookmarks } from './bookmarks';
import { Horizon } from './horizon';
import { Summary, SummaryToken } from './summary';
import { Tile } from './tile';

/** An app on the page and the anchor the summary's token links to. */
interface Placed {
  app: App;
  anchor: string;
}

/** The page's content, arranged. */
interface View {
  dashboard: Dashboard;
  groups: { name: string; apps: Placed[] }[];
  all: Placed[];
  tokens: SummaryToken[];
  bookmarks: BookmarkGroup[];
  scope: string;
}

/** What the discovery reads, in words: the empty page names it, so a selector that matches
 *  nothing is told from a cluster without apps. */
export function scopeWords(scope: DiscoveryScope): string {
  if (!scope.enabled) {
    return 'The discovery is off: the dashboard shows its configured apps and bookmarks only.';
  }
  const ns = scope.namespaces;
  let where: string;
  if (ns.any) {
    where = 'every namespace';
  } else if (ns.label_selector) {
    where =
      `namespaces matching ${ns.label_selector}` +
      (ns.names.length > 0 ? ` and ${ns.names.join(', ')}` : '');
  } else {
    where = ns.names.join(', ');
  }
  const instance = scope.instance_name ? ` for the instance ${scope.instance_name}` : '';
  return `Watching Ingresses in ${where}${instance}.`;
}

function arrange(d: Dashboard): View {
  let n = 0;
  const groups = d.groups.map((g) => ({
    name: g.name,
    apps: g.apps.map((app) => ({ app, anchor: `app-${n++}` })),
  }));
  const all = groups.flatMap((g) => g.apps);
  const bookmarks = d.ui.show_bookmark_groups
    ? d.bookmarks
    : d.bookmarks.length > 0
      ? [{ name: '', bookmarks: d.bookmarks.flatMap((g) => g.bookmarks) }]
      : [];
  return {
    dashboard: d,
    groups,
    all,
    tokens: all.map((p) => ({ name: p.app.name, state: p.app.status.state, anchor: p.anchor })),
    bookmarks,
    scope: scopeWords(d.discovery),
  };
}

/**
 * The dashboard (docs/adr/0005): the summary, the app groups as bracketed grids of tiles, the
 * bookmarks; the horizon where there is nothing to show. The page asks the backend through
 * DashboardService and applies the installation's colour scheme while the person has chosen none.
 */
@Component({
  selector: 'app-dashboard',
  imports: [Summary, Tile, Bracket, Bookmarks, Horizon],
  templateUrl: './dashboard.html',
  styleUrl: './dashboard.css',
  changeDetection: ChangeDetectionStrategy.OnPush,
})
export class DashboardPage {
  private readonly service = inject(DashboardService);
  private readonly theme = inject(ThemeService);

  protected readonly state = this.service.state;
  protected readonly retrySeconds = retrySeconds;
  protected readonly starting = computed(() => {
    const s = this.state();
    return s.kind === 'failed' && s.reason === 'starting';
  });
  protected readonly stale = computed(() => {
    const s = this.state();
    return s.kind === 'ready' && s.stale;
  });
  protected readonly view = computed(() => {
    const s = this.state();
    return s.kind === 'ready' ? arrange(s.dashboard) : undefined;
  });

  constructor() {
    this.service.start();
    effect(() => {
      const s = this.state();
      if (s.kind === 'ready') {
        this.theme.applyDefault(s.dashboard.ui.default_theme);
      }
    });
  }
}
