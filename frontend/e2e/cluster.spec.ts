import { execFileSync } from 'node:child_process';
import { readFileSync } from 'node:fs';
import { join } from 'node:path';
import { expect, test, type APIRequestContext, type Page } from '@playwright/test';

// The suite's own project "cluster" runs this file once, after the browser projects: it changes the
// cluster. The apps are hack/kind/seed.yaml's; golden.json is what the dashboard must show for them.

interface Icon {
  src: string;
  src_dark?: string;
  tint: boolean;
}

interface App {
  id: string;
  name: string;
  url: string;
  icon: Icon;
  badges: { corner: string; label: string }[];
  status: { state: string };
}

interface DashboardBody {
  groups: { name: string; apps: App[] }[];
}

/** kubectl inside the kind node, against its own admin configuration (hack/e2e.sh). */
function kubectl(args: string[], input?: string): string {
  const node = process.env['E2E_NODE'];
  expect(node, 'E2E_NODE: run the suite through hack/e2e.sh').toBeTruthy();
  return execFileSync(
    'docker',
    ['exec', '-i', node!, 'kubectl', '--kubeconfig', '/etc/kubernetes/admin.conf', ...args],
    {
      input,
      encoding: 'utf8',
    },
  );
}

async function seededApps(request: APIRequestContext): Promise<App[]> {
  const response = await request.get('/api/v1/dashboard');
  expect(response.status()).toBe(200);
  const body = (await response.json()) as DashboardBody;
  return body.groups
    .flatMap((g) => g.apps.map((a) => ({ ...a, group: g.name })))
    .filter((a) => a.id.startsWith('ingress/e2e-apps/'));
}

/** An icon as golden.json writes it: the kind, the address without its pack version or hash. */
function iconOf(icon: Icon): string {
  const src = icon.src.replace(/@[^/]+\//, '@*/').replace(/blob\/[0-9a-f]{64}/, 'blob/*');
  return `${icon.tint ? 'glyph' : 'logo'} ${src}${icon.src_dark ? ' +dark' : ''}`;
}

test.describe.configure({ mode: 'serial' });

test('the seeded apps are the golden list', async ({ request }) => {
  const golden = JSON.parse(readFileSync(join(__dirname, 'golden.json'), 'utf8')) as unknown[];
  // The remote icon arrives when the discovery has read its Ingress and the download ended.
  await expect
    .poll(
      async () => (await seededApps(request)).find((a) => a.name === 'Remote')?.icon.src ?? '',
      { timeout: 60_000 },
    )
    .toContain('/blob/');
  const apps = await seededApps(request);
  expect(
    apps.map((a) => ({
      group: (a as App & { group: string }).group,
      name: a.name,
      url: a.url,
      icon: iconOf(a.icon),
      badges: a.badges.map((b) => `${b.corner} ${b.label}`),
      status: a.status.state,
    })),
  ).toEqual(golden);
});

/** The README's pictures (docs/images, make screenshots copies them): the dashboard at desktop and
 *  phone width in each scheme, while one app is down and one degraded. */
async function screenshots(page: Page, outputDir: string): Promise<void> {
  for (const scheme of ['dark', 'light'] as const) {
    // Without motion: a switch of scheme draws at once, no tile caught in its transition.
    await page.emulateMedia({ colorScheme: scheme, reducedMotion: 'reduce' });
    for (const [scene, viewport] of [
      ['dashboard', { width: 1280, height: 900 }],
      ['phone', { width: 390, height: 844 }],
    ] as const) {
      await page.setViewportSize(viewport);
      await expect
        .poll(() =>
          page
            .locator('app-item-icon img')
            .evaluateAll((imgs) => imgs.every((i) => (i as HTMLImageElement).complete)),
        )
        .toBe(true);
      await page.screenshot({
        path: join(outputDir, 'screenshots', `${scene}_${scheme}.png`),
        fullPage: true,
      });
    }
  }
}

test('a Deployment scaled to zero turns its tile down, and back', async ({ page }, testInfo) => {
  await page.goto('/');
  const tile = page.getByRole('link', { name: /^Jellyfin/ });
  const ready = /^\d+\/\d+$/;
  await expect(tile).toBeVisible();
  await expect(tile.locator('.status')).toHaveText(ready);
  kubectl(['-n', 'e2e-apps', 'scale', 'deploy/jellyfin', '--replicas=0']);
  try {
    await expect(tile.locator('.status')).toHaveText('down', { timeout: 60_000 });
    await expect(page.locator('app-summary .legend')).toContainText('1 down');
    await expect(page.getByRole('link', { name: /^Alertmanager/ }).locator('.status')).toHaveText(
      '1 of 2 ready',
    );
    await screenshots(page, testInfo.project.outputDir);
  } finally {
    kubectl(['-n', 'e2e-apps', 'scale', 'deploy/jellyfin', '--replicas=1']);
  }
  await expect(tile.locator('.status')).toHaveText(ready, { timeout: 60_000 });
});

test('a new Ingress appears on the next poll, and leaves when it is deleted', async ({ page }) => {
  await page.goto('/');
  await expect(page.getByRole('link', { name: /Headlamp/ })).toBeVisible();
  kubectl(
    ['apply', '-f', '-'],
    JSON.stringify({
      apiVersion: 'networking.k8s.io/v1',
      kind: 'Ingress',
      metadata: {
        name: 'fresh',
        namespace: 'e2e-apps',
        annotations: { 'dash.gtrfc.com/appName': 'Fresh', 'dash.gtrfc.com/group': 'new' },
      },
      spec: {
        ingressClassName: 'haproxy',
        rules: [
          {
            host: 'fresh.e2e.test',
            http: {
              paths: [
                {
                  path: '/',
                  pathType: 'Prefix',
                  backend: { service: { name: 'web', port: { name: 'http' } } },
                },
              ],
            },
          },
        ],
      },
    }),
  );
  try {
    await expect(page.getByRole('link', { name: /^Fresh/ })).toBeVisible({ timeout: 30_000 });
  } finally {
    kubectl(['-n', 'e2e-apps', 'delete', 'ingress', 'fresh']);
  }
  await expect(page.getByRole('link', { name: /^Fresh/ })).toHaveCount(0, { timeout: 30_000 });
});
