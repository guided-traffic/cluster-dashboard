import { expect, test } from '@playwright/test';
import { connect, type PeerCertificate } from 'node:tls';

/** The page's ground in each scheme (--bg of src/app/design/tokens.css, docs/adr/0012 D1). */
const stage = { dark: 'rgb(18, 19, 22)', light: 'rgb(242, 241, 238)' } as const;

/** The certificate the controller serves for localhost. */
function servedCertificate(url: string): Promise<PeerCertificate> {
  const { hostname, port } = new URL(url);
  return new Promise((resolve, reject) => {
    const socket = connect(
      { host: hostname, port: Number(port), servername: hostname, rejectUnauthorized: false },
      () => {
        resolve(socket.getPeerCertificate());
        socket.end();
      },
    );
    socket.on('error', reject);
  });
}

test('the shell arrives through the frontend Ingress with its security headers', async ({
  page,
}) => {
  const response = await page.goto('/');
  expect(response?.status()).toBe(200);
  expect(response?.headers()['cache-control']).toBe('no-store');
  expect(response?.headers()['content-security-policy']).toContain("script-src 'self'");
  await expect(page).toHaveTitle('Dash');
  await expect(page.getByRole('heading', { level: 1 })).toHaveText('Dash');
});

test('the backend answers through its own Ingress, on the same origin', async ({
  page,
  request,
}) => {
  const response = await request.get('/api/v1/version');
  expect(response.status()).toBe(200);
  const body = await response.json();
  expect(body.version).toBeTruthy();

  await page.goto('/');
  await expect(page.getByTestId('backend-version')).toHaveText(body.version);
});

test('a deep link resolves to the shell', async ({ page }) => {
  const response = await page.goto('/some/deep/link');
  expect(response?.status()).toBe(200);
  await expect(page.getByRole('heading', { level: 1 })).toHaveText('Dash');
});

test('the stage follows the browser scheme while the preference is system', async ({
  page,
}, testInfo) => {
  await page.goto('/');
  await expect(page.locator('html')).toHaveAttribute('data-theme', 'system');
  const scheme = testInfo.project.use.colorScheme === 'dark' ? 'dark' : 'light';
  await expect(page.locator('body')).toHaveCSS('background-color', stage[scheme]);
});

test('a chosen scheme survives a reload', async ({ page }) => {
  await page.goto('/');
  await page.getByRole('radio', { name: 'Dark' }).check();
  await expect(page.locator('html')).toHaveAttribute('data-theme', 'dark');
  await page.reload();
  await expect(page.locator('html')).toHaveAttribute('data-theme', 'dark');
  await expect(page.getByRole('radio', { name: 'Dark' })).toBeChecked();
  await expect(page.locator('body')).toHaveCSS('background-color', stage.dark);

  await page.getByRole('radio', { name: 'Light' }).check();
  await page.reload();
  await expect(page.locator('body')).toHaveCSS('background-color', stage.light);
});

test('the controller serves the certificate cert-manager issued from the frontend annotation', async ({
  baseURL,
}) => {
  const expected = process.env['E2E_TLS_FINGERPRINT'];
  expect(expected, 'E2E_TLS_FINGERPRINT: run the suite through hack/e2e.sh').toBeTruthy();
  const certificate = await servedCertificate(baseURL!);
  expect(certificate.fingerprint256).toBe(expected);
  expect(certificate.subjectaltname).toContain('DNS:localhost');
});
