import { defineConfig, devices } from '@playwright/test';

const port = Number(process.env.UI_TEST_PORT || 4191);

// Run npm run build first. API fixtures never use a shared backend or VM.
export default defineConfig({
  testDir: './tests/ui',
  outputDir: process.env.UI_TEST_OUTPUT || './test-results/ui',
  fullyParallel: true,
  workers: 2,
  retries: 0,
  reporter: 'list',
  use: {
    baseURL: `http://127.0.0.1:${port}`,
    trace: 'retain-on-failure',
  },
  projects: [
    { name: 'desktop', use: { ...devices['Desktop Chrome'] } },
    { name: 'mobile', use: { ...devices['iPhone 13'], defaultBrowserType: 'chromium' } },
  ],
  webServer: {
    command: 'node tests/ui/serve-production.mjs',
    env: { UI_TEST_PORT: String(port) },
    url: `http://127.0.0.1:${port}`,
    reuseExistingServer: false,
  },
});
