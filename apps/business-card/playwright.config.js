import { defineConfig, devices } from '@playwright/test';

// Chromium only in this configuration: Firefox and WebKit are not installed
// in the CI image. Set PLAYWRIGHT_ALL_BROWSERS=1 after `npx playwright
// install` to add them.
const executablePath = process.env.PLAYWRIGHT_CHROMIUM_PATH || undefined;
const chromium = {
  ...devices['Desktop Chrome'],
  launchOptions: { executablePath },
};
const mobile = { ...devices['Pixel 7'], launchOptions: { executablePath } };

export default defineConfig({
  testDir: 'tests/e2e',
  fullyParallel: true,
  forbidOnly: !!process.env.CI,
  retries: 0,
  reporter: process.env.CI ? [['list'], ['html', { open: 'never' }]] : 'list',
  use: { baseURL: 'http://127.0.0.1:4173', trace: 'retain-on-failure' },
  webServer: {
    command: 'node scripts/serve.mjs',
    url: 'http://127.0.0.1:4173/',
    reuseExistingServer: !process.env.CI,
    env: { PORT: '4173' },
  },
  projects: [
    { name: 'desktop-chromium', use: chromium },
    { name: 'mobile-chromium', use: mobile },
    ...(process.env.PLAYWRIGHT_ALL_BROWSERS
      ? [
          { name: 'desktop-firefox', use: devices['Desktop Firefox'] },
          { name: 'mobile-safari', use: devices['iPhone 13'] },
        ]
      : []),
  ],
});
