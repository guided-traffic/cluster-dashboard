import { expect, test } from '@playwright/test';

interface DashboardBody {
  discovery: { enabled: boolean };
  groups: { name: string; apps: { id: string }[] }[];
}

test('the dashboard answers through the backend Ingress and leaves Dash itself out', async ({ request }) => {
  const response = await request.get('/api/v1/dashboard');
  expect(response.status()).toBe(200);
  expect(response.headers()['cache-control']).toBe('no-store');
  const body = (await response.json()) as DashboardBody;
  expect(body.discovery.enabled).toBe(true);
  const ids = body.groups.flatMap((group) => group.apps.map((app) => app.id));
  // hack/kind/dash-values.yaml sets defaultEnable: the chart's opt-out alone keeps them off.
  expect(ids).not.toContain('ingress/dash/dash-frontend');
  expect(ids).not.toContain('ingress/dash/dash-backend');
});
