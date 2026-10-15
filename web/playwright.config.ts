import { defineConfig } from '@playwright/test';

const port = 7899;

// Runs the real binary (with the freshly built UI embedded) against an empty
// data folder. Locally it uses the installed Google Chrome; CI installs
// Playwright's Chromium.
export default defineConfig({
  testDir: 'e2e',
  workers: 1,
  retries: process.env.CI ? 1 : 0,
  use: {
    baseURL: `http://127.0.0.1:${port}`,
    channel: process.env.CI ? undefined : 'chrome',
    trace: 'retain-on-failure',
  },
  webServer: {
    command: `npm run build && cd .. && go build -o bin/ahp-matrix-e2e ./cmd/ahp-matrix && ./bin/ahp-matrix-e2e --no-open --port ${port} --data-dir "$(mktemp -d)"`,
    url: `http://127.0.0.1:${port}/api/version`,
    reuseExistingServer: false,
    timeout: 180_000,
  },
});
