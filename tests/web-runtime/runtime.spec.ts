// SPDX-License-Identifier: AGPL-3.0-only
import { test, expect, type Page } from '@playwright/test';
import { readFileSync } from 'node:fs';
const settings = JSON.parse(readFileSync(process.env.DBM_TEST_CONFIG!, 'utf8'));
declare global { interface Window { cspViolations: string[]; } }
async function setToken(page: Page, token: string) {
  await page.getByRole('button', { name: 'Backend-Zugang' }).click();
  await page.getByLabel('Zugriffstoken').fill(token);
  await page.getByRole('button', { name: 'Übernehmen' }).click();
}
async function connect(page: Page, driver: string) {
  await page.getByRole('button', { name: 'Verbindung hinzufügen', exact: true }).first().click();
  if (driver === 'postgres') await page.getByRole('button', { name: 'PostgreSQL' }).click();
  await page.getByLabel('Name optional').fill(driver === 'postgres' ? 'TLS-Testdatenbank' : 'SQLite-Testdatenbank');
  await page.getByLabel('Datenbank', { exact: true }).fill(driver === 'postgres' ? settings.postgres.database : 'browser_fixture');
  if (driver === 'postgres') {
    await page.getByLabel('Host', { exact: true }).fill(settings.postgres.host);
    await page.getByLabel('Port', { exact: true }).fill(String(settings.postgres.port));
    await page.getByLabel('Benutzername').fill(settings.postgres.username);
    await page.getByLabel('Passwort', { exact: true }).fill(settings.postgres.password);
  }
  await page.getByRole('button', { name: 'Verbindung testen' }).click();
  await expect(page.getByText('Verbindung erfolgreich. Die Datenbank ist erreichbar.')).toBeVisible();
  await page.getByRole('button', { name: 'Verbinden', exact: true }).click();
  await expect(page.getByText('Verbunden', { exact: true })).toBeVisible();
}
async function query(page: Page, sql: string) {
  await page.getByRole('textbox', { name: 'SQL-Abfrage', exact: true }).fill(sql);
  await page.getByRole('button', { name: 'Abfrage ausführen' }).click();
}
// Observe the actual engine before aborting, then prove its DB work/lock is released.
async function databaseActive(page: Page, driver: string) {
  const connection = driver === 'postgres' ? settings.postgres : { driver: 'sqlite', database: 'browser_fixture' };
  const sql = driver === 'postgres'
    ? "SELECT count(*) FROM pg_stat_activity WHERE datname=current_database() AND state='active' AND query LIKE 'SELECT pg_sleep(%'"
    : 'SELECT 1';
  const response = await page.request.post('/api/v1/query', {
    headers: { Authorization: 'Bearer ' + settings.token }, data: { connection, sql }, timeout: 5000
  });
  const body = await response.json();
  if (driver === 'sqlite') {
    if (response.status() === 422 && body.error?.code === 'database_busy') return true;
    expect(response.status()).toBe(200);
    return false;
  }
  expect(response.status()).toBe(200);
  return body.results[0].rows[0][0].value !== '0';
}
for (const driver of ['sqlite', 'postgres']) {
  test(`${driver}: real SQL catalog precision null empty binary truncation cancel and token context`, async ({ page }) => {
    await page.setViewportSize(driver === 'postgres' ? { width: 1024, height: 768 } : { width: 1440, height: 1000 });
    const origins = new Set<string>(); let queryCount = 0;
    page.on('request', (request) => { origins.add(new URL(request.url()).origin); if (request.url().endsWith('/api/v1/query')) queryCount++; });
    await page.addInitScript(() => { window.cspViolations = []; document.addEventListener('securitypolicyviolation', (event) => window.cspViolations.push(event.violatedDirective)); });
    await page.goto('/');
    await page.getByRole('button', { name: 'Verbindung hinzufügen', exact: true }).first().click();
    await expect(page.getByText(/Der Zugriffstoken fehlt oder ist ungültig/)).toBeVisible();
    await page.getByRole('button', { name: 'Schließen' }).click();
    await setToken(page, settings.token); await connect(page, driver);
    await page.getByRole('button', { name: 'f2b_values' }).click();
    await expect(page.getByRole('textbox', { name: 'SQL-Abfrage', exact: true })).toHaveValue(/SELECT \* FROM/);
    expect(queryCount).toBe(0);
    await query(page, 'SELECT id, precise, optional, empty_text, payload FROM f2b_values');
    await expect(page.getByText('9223372036854775807', { exact: true })).toBeVisible();
    await expect(page.getByText('12345678901234567890.12345678901234567890', { exact: true })).toBeVisible();
    await expect(page.getByRole('button', { name: 'null: NULL' })).toBeVisible();
    await expect(page.getByRole('button', { name: 'text:', exact: true })).toHaveText('""');
    await expect(page.getByRole('button', { name: 'binary: AP8=' })).toBeVisible();
    if (driver === 'postgres') {
      await query(page, 'SELECT ssl FROM pg_stat_ssl WHERE pid = pg_backend_pid()');
      await expect(page.getByRole('button', { name: 'boolean: true' })).toBeVisible();
    }
    const many = driver === 'postgres' ? 'SELECT value FROM generate_series(1,101) AS value' : 'WITH RECURSIVE n(value) AS (SELECT 1 UNION ALL SELECT value+1 FROM n WHERE value<101) SELECT value FROM n';
    await query(page, many);
    await expect(page.getByText(/Ergebnis begrenzt/)).toBeVisible();
    const slow = driver === 'postgres' ? 'SELECT pg_sleep(30)' : 'WITH RECURSIVE n(value) AS (SELECT 1 UNION ALL SELECT value+1 FROM n WHERE value<100000000) SELECT sum(value) FROM n';
    await query(page, slow);
    await expect(page.getByRole('button', { name: 'Abbrechen' })).toBeVisible();
    await expect.poll(() => databaseActive(page, driver), { timeout: 10000 }).toBe(true);
    await page.keyboard.press('Escape');
    await expect(page.getByText('Abbruch angefordert', { exact: true })).toBeVisible();
    await expect.poll(() => databaseActive(page, driver), { timeout: 10000 }).toBe(false);
    await query(page, 'SELECT 42 AS recovered');
    await expect(page.getByRole('button', { name: 'integer: 42' })).toBeVisible();
    await query(page, slow);
    await expect(page.getByRole('button', { name: 'Abbrechen' })).toBeVisible();
    await expect.poll(() => databaseActive(page, driver), { timeout: 10000 }).toBe(true);
    await setToken(page, settings.invalidToken);
    await expect.poll(() => databaseActive(page, driver), { timeout: 10000 }).toBe(false);
    await expect(page.getByText('Nicht verbunden', { exact: true })).toBeVisible();
    await expect(page.getByRole('textbox', { name: 'SQL-Abfrage', exact: true })).toHaveValue('');
    await expect(page.getByRole('button', { name: 'f2b_values' })).not.toBeVisible();
    await expect(page.getByRole('button', { name: 'integer: 42' })).not.toBeVisible();
    await expect(page.getByText('Platz für deine Ergebnisse', { exact: true })).toBeVisible();
    await setToken(page, settings.token);
    const persisted = await page.evaluate(() => JSON.stringify({ local: { ...localStorage }, session: { ...sessionStorage }, cookies: document.cookie }));
    expect(persisted.includes(settings.token) || persisted.includes(settings.postgres.password)).toBe(false);
    expect(await page.evaluate(() => window.cspViolations)).toEqual([]);
    expect([...origins]).toEqual([new URL(settings.baseURL).origin]);
  });
}
