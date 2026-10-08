import { defineConfig, devices } from '@playwright/test';
import { mkdtempSync } from 'node:fs';
import { tmpdir } from 'node:os';
import { join } from 'node:path';

const baseURL = 'http://127.0.0.1:19126';
const database = join(mkdtempSync(join(tmpdir(), 'warden-rollup-e2e-')), 'warden.db');

export default defineConfig({
  testDir: './tests/sqlite-rollup',
  workers: 1,
  retries: 0,
  forbidOnly: !!process.env.CI,
  reporter: 'list',
  use: { ...devices['Desktop Chrome'], baseURL, trace: 'retain-on-failure' },
  webServer: {
    command: `go run ./tests/sqlite-rollup/seed && ${process.env.WARDEN_E2E_BINARY || 'go run ../cmd/dashboard'}`,
    url: `${baseURL}/readyz`,
    reuseExistingServer: false,
    timeout: 120_000,
    env: {
      DB_TYPE: 'sqlite',
      SQLITE_ROLLUP_BATCH_SIZE: process.env.SQLITE_ROLLUP_BATCH_SIZE || '10',
      DB_PATH: database,
      LISTEN_ADDR: '127.0.0.1:19126',
      OBSERVABILITY_ADDR: '',
      HTTP_DIAGNOSTICS_ENABLED: 'true',
      ROLLUP_DIAGNOSTICS_ENABLED: 'true',
      COOKIE_SECURE: 'false',
    },
  },
});
