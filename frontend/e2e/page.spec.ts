import { expect, test } from '@playwright/test';

// The page of hack/kind/dash-values.yaml: a custom app Headlamp whose logo auto-match finds, with the
// globe of a rule; a bookmark with a ConfigMap icon and a text chip.

test('the page shows the tiles, their icons and badges and the bookmarks beside them', async ({
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

  await expect(page.getByRole('heading', { level: 2, name: 'Apps', exact: true })).toBeVisible();
  await expect(
    page.getByRole('heading', { level: 2, name: 'Bookmarks', exact: true }),
  ).toBeVisible();

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

test('the greeting welcomes to the environment under its light strip', async ({
  page,
}, testInfo) => {
  await page.goto('/');
  await expect(page.getByTestId('greeting')).toHaveText(
    /^\s*Good (morning|afternoon|evening|night), welcome to staging\.\s*$/,
  );
  const strip = page.locator('.overview app-light-strip');
  const light = strip.locator('.lights.scanner .light');
  await expect(light).toHaveCSS('animation-name', /scan-in, .*scan$/);
  // The light half the strip wide, its orange in the scheme's shade (tokens.css --strip-orange).
  const orange = testInfo.project.use.colorScheme === 'dark' ? '255, 159, 90' : '196, 87, 14';
  const width = await strip.evaluate((e) => e.getBoundingClientRect().width);
  expect(await light.evaluate((e) => e.getBoundingClientRect().width)).toBeCloseTo(width / 2, 0);
  expect(await light.evaluate((e) => getComputedStyle(e).backgroundImage)).toContain(orange);
  // The grain, a speck per device pixel.
  await expect
    .poll(() =>
      strip
        .locator('canvas')
        .evaluate(
          (c: HTMLCanvasElement) =>
            c.width === Math.round(c.clientWidth * devicePixelRatio) && c.height > 0,
        ),
    )
    .toBe(true);
});

test('the light strip stands still at full strength for reduced motion', async ({
  page,
}, testInfo) => {
  await page.emulateMedia({ reducedMotion: 'reduce' });
  await page.goto('/');
  const strip = page.locator('.overview app-light-strip');
  const light = strip.locator('.lights .light');
  await expect(light).toHaveCSS('animation-name', 'none');
  const orange = testInfo.project.use.colorScheme === 'dark' ? '255, 159, 90' : '196, 87, 14';
  await expect(light).toHaveCSS('background-color', `rgb(${orange})`);
  const width = await strip.evaluate((e) => e.getBoundingClientRect().width);
  expect(await light.evaluate((e) => e.getBoundingClientRect().width)).toBe(width);
});
