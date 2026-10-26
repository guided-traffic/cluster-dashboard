import { expect, test } from '@playwright/test';

interface Icon {
  src: string;
  src_dark?: string;
  tint: boolean;
}

interface Badge {
  corner: string;
  icon?: Icon;
  text?: string;
  label: string;
  tone: string;
}

interface DashboardBody {
  discovery: { enabled: boolean };
  groups: { name: string; apps: { id: string; name: string; icon: Icon; badges: Badge[] }[] }[];
  bookmarks: { name: string; bookmarks: { name: string; icon: Icon; badges: Badge[] }[] }[];
}

async function dashboard(
  request: import('@playwright/test').APIRequestContext,
): Promise<DashboardBody> {
  const response = await request.get('/api/v1/dashboard');
  expect(response.status()).toBe(200);
  expect(response.headers()['cache-control']).toBe('no-store');
  return (await response.json()) as DashboardBody;
}

test('the dashboard answers through the backend Ingress and leaves Dash itself out', async ({
  request,
}) => {
  const body = await dashboard(request);
  expect(body.discovery.enabled).toBe(true);
  const ids = body.groups.flatMap((group) => group.apps.map((app) => app.id));
  // hack/kind/dash-values.yaml sets defaultEnable: the chart's opt-out alone keeps them off.
  expect(ids).not.toContain('ingress/dash/dash-frontend');
  expect(ids).not.toContain('ingress/dash/dash-backend');
});

test('icons come from the image packs and the icon ConfigMap, immutable and inert', async ({
  request,
}) => {
  const body = await dashboard(request);
  const headlamp = body.groups
    .flatMap((group) => group.apps)
    .find((app) => app.name === 'Headlamp');
  // Auto-match: a custom app named Headlamp gets the selfh.st logo the image carries.
  expect(headlamp?.icon.src).toMatch(/^\/api\/v1\/icons\/sh@[0-9a-f]{12}\/headlamp\.svg$/);
  expect(headlamp?.icon.tint).toBe(false);
  const bookmark = body.bookmarks
    .flatMap((group) => group.bookmarks)
    .find((b) => b.name === 'ConfigMap icon');
  expect(bookmark?.icon.src).toMatch(/^\/api\/v1\/icons\/blob\/[0-9a-f]{64}\.svg$/);

  for (const src of [headlamp!.icon.src, bookmark!.icon.src]) {
    const response = await request.get(src);
    expect(response.status(), src).toBe(200);
    expect(response.headers()['content-type']).toBe('image/svg+xml');
    expect(response.headers()['cache-control']).toBe('public, max-age=31536000, immutable');
    expect(response.headers()['content-security-policy']).toBe(
      "default-src 'none'; style-src 'unsafe-inline'; sandbox",
    );
    expect(response.headers()['x-content-type-options']).toBe('nosniff');
    expect(await response.text()).toContain('<svg');
  }
});

test('badges come from a rule and from an item of its own', async ({ request }) => {
  const body = await dashboard(request);
  const headlamp = body.groups
    .flatMap((group) => group.apps)
    .find((app) => app.name === 'Headlamp');
  expect(headlamp?.badges).toHaveLength(1);
  expect(headlamp?.badges[0]).toMatchObject({
    corner: 'top-right',
    label: 'reachable worldwide',
    tone: 'cyan',
  });
  expect(headlamp?.badges[0].icon?.tint).toBe(true);
  const glyph = await request.get(headlamp!.badges[0].icon!.src);
  expect(glyph.status()).toBe(200);

  const bookmark = body.bookmarks
    .flatMap((group) => group.bookmarks)
    .find((b) => b.name === 'ConfigMap icon');
  expect(bookmark?.badges).toEqual([
    { corner: 'bottom-left', text: 'cm', label: 'cm', tone: 'neutral' },
  ]);
});
