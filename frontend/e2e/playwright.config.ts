import { defineConfig, devices } from '@playwright/test';

/**
 * The end-to-end suite (docs/adr/0003): the built images in a kind cluster behind haproxy-ingress
 * and cert-manager, reached through both Ingress objects on https://localhost. hack/e2e.sh sets
 * E2E_BASE_URL and E2E_TLS_FINGERPRINT. Chromium and WebKit, each in both colour schemes.
 */
const baseURL = process.env['E2E_BASE_URL'] ?? 'https://localhost:18443';

export default defineConfig({
  testDir: '.',
  outputDir: 'test-results',
  fullyParallel: true,
  forbidOnly: !!process.env['CI'],
  retries: 0,
  reporter: [['list'], ['html', { outputFolder: 'playwright-report', open: 'never' }]],
  use: {
    baseURL,
    // The certificate is cert-manager's self-signed one; the suite checks it is the one served.
    ignoreHTTPSErrors: true,
    trace: 'retain-on-failure',
    video: 'retain-on-failure',
  },
  projects: [
    { name: 'chromium-dark', use: { ...devices['Desktop Chrome'], colorScheme: 'dark' } },
    { name: 'chromium-light', use: { ...devices['Desktop Chrome'], colorScheme: 'light' } },
    { name: 'webkit-dark', use: { ...devices['Desktop Safari'], colorScheme: 'dark' } },
    { name: 'webkit-light', use: { ...devices['Desktop Safari'], colorScheme: 'light' } },
  ],
});
