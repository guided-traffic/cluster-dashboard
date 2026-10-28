import { ChangeDetectionStrategy, Component, computed, effect, inject } from '@angular/core';
import { App } from '../../api/models/app';
import { BookmarkGroup } from '../../api/models/bookmark-group';
import { Dashboard } from '../../api/models/dashboard';
import { DiscoveryScope } from '../../api/models/discovery-scope';
import { Clock, greeting, longDate, shortTime } from '../../core/clock';
import { DashboardService, retrySeconds } from '../../core/dashboard.service';
import { ThemeService } from '../../core/theme.service';
import { GroupHead } from '../../shared/group-head/group-head';
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
  /** The apps' bar: "13 apps · 3 groups". */
  appsMeta: string;
  /** The bookmarks' bar: "8 links". */
  bookmarksMeta: string;
}

/** "1 app", "13 apps". */
function counted(n: number, noun: string): string {
  return `${n} ${noun}${n === 1 ? '' : 's'}`;
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
  const links = bookmarks.reduce((n, g) => n + g.bookmarks.length, 0);
  return {
    dashboard: d,
    groups,
    all,
    tokens: all.map((p) => ({ name: p.app.name, state: p.app.status.state, anchor: p.anchor })),
    bookmarks,
    scope: scopeWords(d.discovery),
    appsMeta: d.ui.show_app_groups
      ? `${counted(all.length, 'app')} · ${counted(groups.length, 'group')}`
      : counted(all.length, 'app'),
    bookmarksMeta: counted(links, 'link'),
  };
}

/**
 * The dashboard (docs/adr/0012 D2): the overview — the greeting or the title, the summary — over
 * the split of the apps, their groups as hairline grids of tiles, beside the bookmarks; a footer
 * naming what the discovery reads. The horizon where there is nothing to show. The page asks the
 * backend through DashboardService and applies the installation's colour scheme while the person
 * has chosen none.
 */
@Component({
  selector: 'app-dashboard',
  imports: [Summary, Tile, GroupHead, Bookmarks, Horizon],
  templateUrl: './dashboard.html',
  styleUrl: './dashboard.css',
  changeDetection: ChangeDetectionStrategy.OnPush,
})
export class DashboardPage {
  private readonly service = inject(DashboardService);
  private readonly theme = inject(ThemeService);
  private readonly clock = inject(Clock);

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
  /** The time of day and the name, while ui.greeting is enabled. */
  protected readonly greeting = computed(() => {
    const s = this.state();
    if (s.kind !== 'ready' || !s.dashboard.ui.greeting.enabled) {
      return undefined;
    }
    return greeting(this.clock.now(), s.dashboard.ui.greeting.name);
  });
  /** The overview's bar: "Monday, 26 October · 19:30". */
  protected readonly when = computed(() => {
    const now = this.clock.now();
    return `${longDate(now)} · ${shortTime(now)}`;
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
