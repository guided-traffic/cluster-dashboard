import { App } from '../app/api/models/app';
import { Dashboard } from '../app/api/models/dashboard';

// Test fixtures of the dashboard answer, shared by the specs; not part of the bundle.

/** A dashboard answer with refresh_seconds of 30 and one app. */
export function dashboardOf(overrides: Partial<Dashboard> = {}): Dashboard {
  return {
    ui: {
      title: 'Dash',
      greeting: { enabled: true, name: 'You' },
      default_theme: 'system',
      refresh_seconds: 30,
      show_summary: true,
      show_app_groups: true,
      show_app_urls: true,
      show_bookmark_groups: true,
    },
    discovery: {
      enabled: true,
      instance_name: '',
      namespaces: { any: true, names: [], label_selector: '' },
    },
    summary: { total: 1, healthy: 1, degraded: 0, down: 0, unknown: 0 },
    groups: [
      {
        name: 'monitoring',
        apps: [
          {
            id: 'ingress/monitoring/grafana',
            kind: 'ingress',
            name: 'Grafana',
            url: 'https://grafana.example.com/',
            target_blank: false,
            status: { state: 'healthy', ready: 1, total: 1 },
            icon: { src: '/api/v1/icons/sh@abc/grafana.svg', tint: false },
            badges: [],
          },
        ],
      },
    ],
    bookmarks: [],
    ...overrides,
  };
}

/** An app tile, changed by overrides. */
export function appOf(overrides: Partial<App> = {}): App {
  return {
    id: 'ingress/monitoring/grafana',
    kind: 'ingress',
    name: 'Grafana',
    url: 'https://grafana.example.com/',
    target_blank: false,
    status: { state: 'healthy', ready: 1, total: 1 },
    icon: { src: '/api/v1/icons/sh@abc/grafana.svg', tint: false },
    badges: [],
    ...overrides,
  };
}
