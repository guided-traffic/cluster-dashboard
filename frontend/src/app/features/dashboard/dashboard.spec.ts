import { provideHttpClient } from '@angular/common/http';
import { HttpTestingController, provideHttpClientTesting } from '@angular/common/http/testing';
import { TestBed } from '@angular/core/testing';
import { appOf, dashboardOf } from '../../../testing/dashboard';
import { provideApiConfiguration } from '../../api/api-configuration';
import { Dashboard } from '../../api/models/dashboard';
import { longDate, shortTime } from '../../core/clock';
import { DashboardService } from '../../core/dashboard.service';
import { ThemeService } from '../../core/theme.service';
import { DashboardPage, scopeWords } from './dashboard';

describe('DashboardPage', () => {
  let http: HttpTestingController;

  beforeEach(() => {
    vi.useFakeTimers();
    Object.defineProperty(document, 'visibilityState', { value: 'visible', configurable: true });
    TestBed.configureTestingModule({
      providers: [provideHttpClient(), provideHttpClientTesting(), provideApiConfiguration('')],
    });
    http = TestBed.inject(HttpTestingController);
  });

  afterEach(() => {
    TestBed.inject(DashboardService).stop();
    http.verify();
    vi.useRealTimers();
    localStorage.clear();
    document.documentElement.removeAttribute('data-theme');
  });

  function render(answer: Dashboard | 'starting' | 'unreachable') {
    const fixture = TestBed.createComponent(DashboardPage);
    fixture.detectChanges();
    const request = http.expectOne('/api/v1/dashboard');
    if (answer === 'starting') {
      request.flush({ code: 'not_ready' }, { status: 503, statusText: 'Service Unavailable' });
    } else if (answer === 'unreachable') {
      request.error(new ProgressEvent('error'));
    } else {
      request.flush(answer);
    }
    fixture.detectChanges();
    return { fixture, host: fixture.nativeElement as HTMLElement };
  }

  const text = (el: Element | null) => el?.textContent?.replace(/\s+/g, ' ').trim();

  it('shows nothing while the first answer is on its way', () => {
    const fixture = TestBed.createComponent(DashboardPage);
    fixture.detectChanges();
    const host = fixture.nativeElement as HTMLElement;
    expect(host.querySelector('app-tile')).toBeNull();
    expect(host.querySelector('app-horizon')).toBeNull();
    http.expectOne('/api/v1/dashboard').flush(dashboardOf());
  });

  it('says a backend that is starting, and one that cannot be reached, on the horizon', () => {
    let { host } = render('starting');
    expect(text(host.querySelector('app-horizon .headline'))).toBe('the backend is starting');
    expect(text(host.querySelector('app-horizon .detail'))).toBe(
      'The page asks again every 5 seconds.',
    );
    TestBed.inject(DashboardService).stop();
    TestBed.resetTestingModule();
    TestBed.configureTestingModule({
      providers: [provideHttpClient(), provideHttpClientTesting(), provideApiConfiguration('')],
    });
    http = TestBed.inject(HttpTestingController);
    ({ host } = render('unreachable'));
    expect(text(host.querySelector('app-horizon .headline'))).toBe('the backend cannot be reached');
  });

  it('shows the summary, the groups under their heads beside the bookmarks', () => {
    const { host } = render(
      dashboardOf({
        groups: [
          {
            name: 'monitoring',
            apps: [appOf(), appOf({ id: 'ingress/monitoring/prometheus', name: 'Prometheus' })],
          },
          { name: 'media', apps: [appOf({ id: 'ingress/media/jellyfin', name: 'Jellyfin' })] },
        ],
        summary: { total: 3, healthy: 3, degraded: 0, down: 0, unknown: 0 },
        bookmarks: [
          {
            name: 'docs',
            bookmarks: [
              {
                id: 'bookmark/0/0',
                name: 'K8s',
                url: 'https://kubernetes.io',
                target_blank: false,
                icon: { src: '/k.svg', tint: true },
                badges: [],
              },
            ],
          },
        ],
      }),
    );
    expect(host.querySelector('app-summary')).not.toBeNull();
    expect(text(host.querySelector('app-summary .legend'))).toBe(
      '3 apps in 2 groups. All healthy.',
    );
    expect([...host.querySelectorAll('.group > app-group-head h3')].map(text)).toEqual([
      'monitoring, 2',
      'media, 1',
    ]);
    expect([...host.querySelectorAll('app-tile a')].map((a) => a.id)).toEqual([
      'app-0',
      'app-1',
      'app-2',
    ]);
    expect(
      [...host.querySelectorAll('app-summary .token')].map((a) => a.getAttribute('href')),
    ).toEqual(['#app-0', '#app-1', '#app-2']);
    expect(text(host.querySelector('#apps .bar .meta'))).toBe('3 apps · 2 groups');
    expect(text(host.querySelector('#bookmarks app-group-head h3'))).toBe('docs, 1');
    expect(text(host.querySelector('#bookmarks .bar .meta'))).toBe('1 link');
    expect(host.querySelector('.stale')).toBeNull();
  });

  it('heads its sections with a number a screen reader skips', () => {
    const { host } = render(
      dashboardOf({
        bookmarks: [
          {
            name: 'docs',
            bookmarks: [
              {
                id: 'bookmark/0/0',
                name: 'K8s',
                url: 'https://kubernetes.io',
                target_blank: false,
                icon: { src: '/k.svg', tint: true },
                badges: [],
              },
            ],
          },
        ],
      }),
    );
    const heads = [...host.querySelectorAll('h2')];
    expect(heads.map((h) => h.lastChild!.textContent!.trim())).toEqual([
      'Overview',
      'Apps',
      'Bookmarks',
    ]);
    expect(heads.map((h) => text(h.querySelector('.number')))).toEqual(['01/', '02/', '03/']);
    heads.forEach((h) =>
      expect(h.querySelector('.number')!.getAttribute('aria-hidden')).toBe('true'),
    );
    expect(host.querySelector('section.pane.apps')!.getAttribute('aria-labelledby')).toBe(
      'apps-title',
    );
  });

  it('greets by the time of day and the name, the time set apart', () => {
    const now = new Date(2026, 9, 26, 19, 30);
    vi.setSystemTime(now);
    const d = dashboardOf();
    d.ui = { ...d.ui, greeting: { enabled: true, name: 'Hans' } };
    const { host } = render(d);
    expect(text(host.querySelector('[data-testid=greeting]'))).toBe('Good evening, Hans.');
    expect(text(host.querySelector('.display em'))).toBe('evening');
    expect(text(host.querySelector('.overview .bar .meta'))).toBe(
      `${longDate(now)} · ${shortTime(now)}`,
    );
  });

  it('greets without a name when ui.greeting has none', () => {
    vi.setSystemTime(new Date(2026, 9, 26, 9, 0));
    const d = dashboardOf();
    d.ui = { ...d.ui, greeting: { enabled: true, name: ' ' } };
    const { host } = render(d);
    expect(text(host.querySelector('[data-testid=greeting]'))).toBe('Good morning.');
  });

  it('words the summary without groups while ui.showAppGroups is off', () => {
    const d = dashboardOf();
    d.ui = { ...d.ui, show_app_groups: false };
    const { host } = render(d);
    expect(text(host.querySelector('app-summary .legend'))).toBe('1 app. All healthy.');
  });

  it('shows the title instead while ui.greeting is off', () => {
    const d = dashboardOf();
    d.ui = { ...d.ui, title: 'Home lab', greeting: { enabled: false, name: 'Hans' } };
    const { host } = render(d);
    expect(host.querySelector('[data-testid=greeting]')).toBeNull();
    expect(text(host.querySelector('.display'))).toBe('Home lab');
  });

  it('names in its footer what the discovery reads and how often the page asks', () => {
    const { host } = render(dashboardOf());
    expect([...host.querySelectorAll('.foot p')].map(text)).toEqual([
      'Watching Ingresses in every namespace.',
      'Refresh every 30 s',
    ]);
  });

  it('follows the switches the page applies itself', () => {
    const d = dashboardOf({
      bookmarks: [
        {
          name: 'a',
          bookmarks: [
            {
              id: 'b/0/0',
              name: 'A',
              url: 'https://a',
              target_blank: false,
              icon: { src: '/a', tint: true },
              badges: [],
            },
          ],
        },
        {
          name: 'b',
          bookmarks: [
            {
              id: 'b/1/0',
              name: 'B',
              url: 'https://b',
              target_blank: false,
              icon: { src: '/b', tint: true },
              badges: [],
            },
          ],
        },
      ],
    });
    d.ui = {
      ...d.ui,
      show_summary: false,
      show_app_groups: false,
      show_app_urls: false,
      show_bookmark_groups: false,
    };
    const { host } = render(d);
    expect(host.querySelector('app-summary')).toBeNull();
    expect(host.querySelector('.group')).toBeNull();
    expect(host.querySelectorAll('#apps app-group-head').length).toBe(0);
    expect(host.querySelectorAll('#apps .grid app-tile').length).toBe(1);
    expect(text(host.querySelector('#apps .bar .meta'))).toBe('1 app');
    expect(host.querySelector('app-tile .url')).toBeNull();
    expect(host.querySelectorAll('app-bookmarks app-group-head').length).toBe(0);
    expect(host.querySelectorAll('app-bookmarks li').length).toBe(2);
    expect(text(host.querySelector('#bookmarks .bar .meta'))).toBe('2 links');
  });

  it('draws the horizon and names the scope when there is nothing to show', () => {
    const { host } = render(
      dashboardOf({
        groups: [],
        summary: { total: 0, healthy: 0, degraded: 0, down: 0, unknown: 0 },
        discovery: {
          enabled: true,
          instance_name: 'home',
          namespaces: { any: false, names: ['media'], label_selector: '' },
        },
      }),
    );
    expect(text(host.querySelector('app-horizon .headline'))).toBe('no apps discovered');
    expect(text(host.querySelector('[data-testid=scope]'))).toBe(
      'Watching Ingresses in media for the instance home.',
    );
  });

  it('names the scope above the bookmarks when there are bookmarks but no app', () => {
    const { host } = render(
      dashboardOf({
        groups: [],
        bookmarks: [
          {
            name: 'a',
            bookmarks: [
              {
                id: 'b/0/0',
                name: 'A',
                url: 'https://a',
                target_blank: false,
                icon: { src: '/a', tint: true },
                badges: [],
              },
            ],
          },
        ],
      }),
    );
    expect(host.querySelector('app-horizon')).toBeNull();
    expect(text(host.querySelector('[data-testid=scope]'))).toBe(
      'No apps discovered. Watching Ingresses in every namespace.',
    );
    expect(host.querySelector('app-summary')).toBeNull();
    expect(host.querySelector('#apps .bar .meta')).toBeNull();
    expect(host.querySelector('app-bookmarks')).not.toBeNull();
  });

  it('keeps showing the last answer when a refresh fails, and says so', () => {
    const { fixture, host } = render(dashboardOf());
    vi.advanceTimersByTime(30_000);
    http.expectOne('/api/v1/dashboard').flush(null, { status: 502, statusText: 'Bad Gateway' });
    fixture.detectChanges();
    expect(host.querySelector('app-tile')).not.toBeNull();
    expect(text(host.querySelector('.stale'))).toBe(
      'The backend did not answer the last refresh: this is its last answer.',
    );
  });

  it('applies the installation colour scheme while the person has chosen none', () => {
    const d = dashboardOf();
    d.ui = { ...d.ui, default_theme: 'dark' };
    const { fixture } = render(d);
    fixture.detectChanges();
    expect(TestBed.inject(ThemeService).preference()).toBe('dark');
  });

  it('words every scope', () => {
    const scope = (
      namespaces: Partial<Dashboard['discovery']['namespaces']>,
      extra: Partial<Dashboard['discovery']> = {},
    ) =>
      scopeWords({
        enabled: true,
        instance_name: '',
        namespaces: { any: false, names: [], label_selector: '', ...namespaces },
        ...extra,
      });
    expect(scope({ any: true })).toBe('Watching Ingresses in every namespace.');
    expect(scope({ names: ['a', 'b'] })).toBe('Watching Ingresses in a, b.');
    expect(scope({ label_selector: 'dash=listed' })).toBe(
      'Watching Ingresses in namespaces matching dash=listed.',
    );
    expect(scope({ label_selector: 'dash=listed', names: ['static'] })).toBe(
      'Watching Ingresses in namespaces matching dash=listed and static.',
    );
    expect(scope({}, { enabled: false })).toBe(
      'The discovery is off: the dashboard shows its configured apps and bookmarks only.',
    );
  });
});
