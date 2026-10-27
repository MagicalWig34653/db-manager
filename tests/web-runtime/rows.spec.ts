// SPDX-License-Identifier: AGPL-3.0-only
import { test, expect, type Page } from '@playwright/test';
import { readFileSync } from 'node:fs';
const config = JSON.parse(readFileSync(process.env.DBM_TEST_CONFIG!, 'utf8'));
const precise = '12345678901234567890.12345678901234567890';
function connection(driver: string) {
  return driver === 'sqlite' ? { driver, database: 'browser_fixture' } : config.postgres;
}
async function sql(page: Page, driver: string, statement: string) {
  const response = await page.request.post('/api/v1/query', {
    headers: { Authorization: 'Bearer ' + config.token },
    data: { connection: connection(driver), sql: statement },
  });
  expect(response.status()).toBe(200);
  return response.json();
}
async function connect(page: Page, driver: string) {
  await page.setViewportSize(
    driver === 'sqlite' ? { width: 1440, height: 1000 } : { width: 1024, height: 768 },
  );
  await page.addInitScript(() => {
    window.cspViolations = [];
    document.addEventListener('securitypolicyviolation', (event) =>
      window.cspViolations.push(event.violatedDirective),
    );
  });
  await page.goto('/');
  await page.getByRole('button', { name: 'Backend-Zugang' }).click();
  await page.getByLabel('Zugriffstoken').fill(config.token);
  await page.getByRole('button', { name: 'Übernehmen' }).click();
  await page.getByRole('button', { name: 'Verbindung hinzufügen', exact: true }).first().click();
  if (driver === 'postgresql') await page.getByRole('button', { name: 'PostgreSQL' }).click();
  await page
    .getByLabel('Datenbank', { exact: true })
    .fill(driver === 'sqlite' ? 'browser_fixture' : config.postgres.database);
  if (driver === 'postgresql') {
    await page.getByLabel('Host', { exact: true }).fill(config.postgres.host);
    await page.getByLabel('Port', { exact: true }).fill(String(config.postgres.port));
    await page.getByLabel('Benutzername').fill(config.postgres.username);
    await page.getByLabel('Passwort', { exact: true }).fill(config.postgres.password);
  }
  await page.getByRole('button', { name: 'Verbinden', exact: true }).click();
  await page.getByRole('button', { name: /f4_rows$/ }).click();
  await expect(page.getByRole('button', { name: 'Zeile hinzufügen', exact: true })).toBeVisible();
}
async function value(page: Page, column: string, text: string, kind = 'text') {
  await page.getByLabel('Angabe für ' + column, { exact: true }).selectOption('value');
  await page.getByLabel('Werttyp für ' + column, { exact: true }).selectOption(kind);
  await page.getByLabel('Wert für ' + column, { exact: true }).fill(text);
}
async function confirm(page: Page, action: string, review = true) {
  if (review) await page.getByRole('button', { name: 'Änderung prüfen', exact: true }).click();
  await page
    .getByRole('checkbox', { name: 'Ich bestätige diesen einzelnen Schreibvorgang.' })
    .check();
  await page
    .getByRole('button', { name: action, exact: true })
    .evaluate((button: HTMLButtonElement) => {
      button.click();
      button.click();
    });
}
async function closeSuccess(page: Page) {
  await expect(page.getByRole('button', { name: 'Schließen und Tabelle neu laden' })).toBeVisible();
  await page.getByRole('button', { name: 'Schließen und Tabelle neu laden' }).click();
  await expect(page.getByRole('button', { name: 'Zeile hinzufügen', exact: true })).toBeEnabled();
}
for (const driver of ['sqlite', 'postgresql']) {
  test(`${driver}: real single-row insert update delete defaults typed snapshot and conflict`, async ({
    page,
  }) => {
    const writes: { path: string; body: any }[] = [];
    page.on('request', (r) => {
      if (/\/api\/v[34]\/rows\/(insert|update|delete)$/.test(r.url()))
        writes.push({ path: new URL(r.url()).pathname, body: r.postDataJSON() });
    });
    await connect(page, driver);
    await page.getByRole('button', { name: 'Zeile hinzufügen', exact: true }).click();
    await expect(page.getByText('Schlüssel wird von der Datenbank vergeben')).toBeVisible();
    await expect(page.getByLabel('Angabe für generated_text', { exact: true })).toHaveCount(0);
    await page.getByLabel('Angabe für optional', { exact: true }).selectOption('null');
    await value(page, 'empty_text', '');
    await value(page, 'precise', precise, driver === 'sqlite' ? 'text' : 'decimal');
    await value(page, 'payload', 'AP8=', 'binary');
    await confirm(page, 'Zeile verbindlich hinzufügen');
    await expect(
      page.getByRole('dialog').getByText('default-label', { exact: true }),
    ).toBeVisible();
    await expect(page.getByRole('dialog').getByText(precise, { exact: true })).toBeVisible();
    await expect(
      page.getByRole('dialog').getByRole('button', { name: 'binary: AP8=' }),
    ).toBeVisible();
    await expect(
      page.getByRole('dialog').getByRole('button', { name: 'null: NULL' }),
    ).toBeVisible();
    expect(writes).toHaveLength(1);
    expect(
      writes[0].body.values.some(
        (c: any) => c.column === 'id' || c.column === 'label' || c.column === 'generated_text',
      ),
    ).toBe(false);
    expect(writes[0].body.values).toContainEqual({ column: 'optional', value: { kind: 'null' } });
    expect(writes[0].body.values).toContainEqual({
      column: 'empty_text',
      value: { kind: 'text', value: '' },
    });
    await closeSuccess(page);
    await page.getByRole('button', { name: 'Zeile 1 bearbeiten', exact: true }).click();
    await expect(page.getByLabel('Angabe für id', { exact: true })).toHaveCount(0);
    await expect(page.getByLabel('Angabe für generated_text', { exact: true })).toHaveCount(0);
    await value(page, 'label', 'edited-once');
    await confirm(page, 'Änderung verbindlich speichern');
    await expect(page.getByRole('dialog').getByText('edited-once', { exact: true })).toBeVisible();
    expect(writes).toHaveLength(2);
    expect(writes[1].body.expected.columns).toEqual([
      'id',
      'label',
      'optional',
      'empty_text',
      'precise',
      'payload',
      'generated_text',
    ]);
    expect(writes[1].body.expected.values).toContainEqual({
      kind: driver === 'sqlite' ? 'text' : 'decimal',
      value: precise,
    });
    await closeSuccess(page);
    await page.getByRole('button', { name: 'Zeile 1 bearbeiten', exact: true }).click();
    await value(page, 'label', 'must-not-overwrite');
    await page.getByRole('button', { name: 'Änderung prüfen', exact: true }).click();
    await sql(page, driver, "UPDATE f4_rows SET label='parallel-change' WHERE label='edited-once'");
    await confirm(page, 'Änderung verbindlich speichern', false);
    await expect(page.getByRole('dialog').getByRole('alert')).toContainText('zwischenzeitlich');
    expect(writes).toHaveLength(3);
    await expect(page.getByRole('button', { name: 'Änderung verbindlich speichern' })).toHaveCount(
      0,
    );
    await page.getByRole('button', { name: 'Entwurf verwerfen und Daten neu laden' }).click();
    await expect(page.getByText('parallel-change', { exact: true })).toBeVisible();
    await page.getByRole('button', { name: 'Zeile 1 löschen', exact: true }).click();
    await confirm(page, 'Zeile endgültig löschen', false);
    await closeSuccess(page);
    expect(writes).toHaveLength(4);
    for (const table of ['f4_view', 'f4_no_pk']) {
      await page
        .getByRole('button', {
          name: new RegExp(table + (table.endsWith('view') ? '\\s*View$' : '$')),
        })
        .click();
      await expect(
        page.getByText(/Für diese Tabelle sind keine Zeilenänderungen freigegeben/),
      ).toBeVisible();
      await expect(page.getByRole('button', { name: 'Zeile hinzufügen', exact: true })).toHaveCount(
        0,
      );
      await expect(page.getByRole('button', { name: /Zeile 1 bearbeiten/ })).toHaveCount(0);
    }
  });
  test(`${driver}: actual committed insert with lost response is never retried and session is separated`, async ({
    page,
  }) => {
    await connect(page, driver);
    let realWrites = 0;
    let committed = false;
    await page.route('**/api/v4/rows/insert', async (route) => {
      realWrites++;
      const response = await route.fetch(); // Forward the actual request; never fabricate database output.
      expect(response.status()).toBe(200);
      const payload = await response.json();
      expect(payload.outcome).toBe('committed');
      committed = true;
      await route.abort('connectionreset'); // Lose only the real committed response.
    });
    await page.getByRole('button', { name: 'Zeile hinzufügen', exact: true }).click();
    await value(page, 'label', 'lost-response');
    await confirm(page, 'Zeile verbindlich hinzufügen');
    await expect(page.getByRole('dialog').getByRole('alert')).toContainText('unbekannt');
    expect(committed).toBe(true);
    expect(realWrites).toBe(1);
    const actual = await sql(
      page,
      driver,
      "SELECT COUNT(*) AS n FROM f4_rows WHERE label='lost-response'",
    );
    expect(actual.results[0].rows[0][0]).toEqual({ kind: 'integer', value: '1' });
    await expect(page.getByRole('button', { name: 'Zeile verbindlich hinzufügen' })).toHaveCount(0);
    await page.getByRole('button', { name: 'Entwurf verwerfen und Daten neu laden' }).click();
    await page.unroute('**/api/v4/rows/insert');
    let release!: () => void;
    const barrier = new Promise<void>((resolve) => {
      release = resolve;
    });
    let heldCommit = false;
    await page.route('**/api/v4/rows/insert', async (route) => {
      realWrites++;
      const response = await route.fetch();
      expect(response.status()).toBe(200);
      heldCommit = true;
      await barrier;
      try {
        await route.abort('connectionreset');
      } catch {
        /* Browser session already aborted. */
      }
    });
    await page.getByRole('button', { name: 'Zeile hinzufügen', exact: true }).click();
    await value(page, 'label', 'old-session-commit');
    await confirm(page, 'Zeile verbindlich hinzufügen');
    await expect.poll(() => heldCommit).toBe(true);
    // Web review F-1: the editor cannot be dismissed while its write request is open (the lock stays held).
    await page.keyboard.press('Escape');
    await expect(page.getByRole('dialog')).toBeVisible();
    release();
    await expect(page.getByRole('dialog').getByRole('alert')).toContainText('unbekannt');
    await page.getByRole('button', { name: 'Entwurf verwerfen und Daten neu laden' }).click();
    await page.getByRole('button', { name: 'Backend-Zugang' }).click();
    await page.getByLabel('Zugriffstoken').fill(config.invalidToken);
    await page.getByRole('button', { name: 'Übernehmen' }).click();
    await expect(page.getByText('Nicht verbunden', { exact: true })).toBeVisible();
    await expect(
      page.getByRole('region', { name: 'Tabellenarbeitsplatz', exact: true }),
    ).not.toBeVisible();
    await expect(page.getByText('old-session-commit', { exact: true })).toHaveCount(0);
    expect(realWrites).toBe(2);
    expect(await page.evaluate(() => window.cspViolations)).toEqual([]);
    const final = await sql(
      page,
      driver,
      "SELECT COUNT(*) AS n FROM f4_rows WHERE label='old-session-commit'",
    );
    expect(final.results[0].rows[0][0]).toEqual({ kind: 'integer', value: '1' });
    const storage = await page.evaluate(() =>
      JSON.stringify({
        local: { ...localStorage },
        session: { ...sessionStorage },
        cookie: document.cookie,
        url: location.href,
      }),
    );
    expect(
      storage.includes(config.token) ||
        storage.includes(config.postgres.password) ||
        storage.includes('old-session-commit'),
    ).toBe(false);
  });
}
