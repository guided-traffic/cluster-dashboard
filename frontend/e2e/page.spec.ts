import { expect, test } from '@playwright/test';

// The page of hack/kind/dash-values.yaml: a custom app Headlamp whose logo auto-match finds, with the
// globe of a rule; a bookmark with a ConfigMap icon and a text chip.

test('the page shows the tiles, their icons and badges, the summary and the bookmarks', async ({
  page,
}) => {
  await page.goto('/');
  const tile = page.getByRole('link', { name: /Headlamp/ });
  await expect(tile).toBeVisible();
  await expect(tile).toHaveAttribute('href', 'https://headlamp.example.com');
  const logo = tile.locator('app-item-icon img');
  await expect(logo).toHaveAttribute('src', /\/api\/v1\/icons\/sh@[0-9a-f]{12}\/headlamp\.svg$/);
  await expect
    .poll(() => logo.evaluate((img: HTMLImageElement) => img.complete && img.naturalWidth > 0))
    .toBe(true);
  await expect(tile.getByRole('img', { name: 'reachable worldwide' })).toBeVisible();

  await expect(page.getByRole('heading', { level: 2, name: /^apps/ })).toBeVisible();
  await expect(page.locator('app-summary .legend')).toHaveText(/all healthy|unknown/);

  const bookmark = page.getByRole('link', { name: /ConfigMap icon/ });
  await expect(bookmark).toHaveAttribute('href', 'https://example.com');
  await expect(bookmark.getByRole('img', { name: 'cm' })).toBeVisible();
});

test('the page lays one column out at phone width, without scrolling sideways', async ({
  page,
}) => {
  await page.setViewportSize({ width: 390, height: 844 });
  await page.goto('/');
  await expect(page.getByRole('link', { name: /Headlamp/ })).toBeVisible();
  const overflow = await page.evaluate(
    () => document.documentElement.scrollWidth - document.documentElement.clientWidth,
  );
  expect(overflow).toBeLessThanOrEqual(0);
});
