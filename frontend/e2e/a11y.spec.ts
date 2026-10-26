import AxeBuilder from '@axe-core/playwright';
import { expect, test } from '@playwright/test';

// axe in both schemes and both engines: no violation of serious or critical impact.
test('the page has no serious or critical accessibility violation', async ({ page }) => {
  await page.goto('/');
  await expect(page.getByRole('link', { name: /Headlamp/ })).toBeVisible();
  const results = await new AxeBuilder({ page }).analyze();
  const severe = results.violations.filter(
    (v) => v.impact === 'serious' || v.impact === 'critical',
  );
  expect(
    severe.map((v) => `${v.id}: ${v.nodes.map((n) => n.target.join(' ')).join(', ')}`),
  ).toEqual([]);
});
