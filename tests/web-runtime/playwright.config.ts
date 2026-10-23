// SPDX-License-Identifier: AGPL-3.0-only
import { defineConfig } from '@playwright/test';
import { readFileSync } from 'node:fs';
const settings = JSON.parse(readFileSync(process.env.DBM_TEST_CONFIG!, 'utf8'));
export default defineConfig({
  testDir: '.', testMatch: ['runtime.spec.ts', 'tables.spec.ts', 'rows.spec.ts'], workers: 1,
  timeout: 60000, expect: { timeout: 10000 },
  preserveOutput: 'never',
  outputDir: settings.outputDir,
  use: { baseURL: settings.baseURL, browserName: 'chromium', trace: 'off', video: 'off', screenshot: 'off' },
});
