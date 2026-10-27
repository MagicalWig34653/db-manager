// SPDX-License-Identifier: AGPL-3.0-only
// Adapted from reviewed Web source 8ef738d22a1164291e2c8183d80e72d1f043c0f8, tests/real/export.spec.ts.
import { test, expect, type Page } from '@playwright/test';
import { readFileSync } from 'node:fs';
const config = JSON.parse(readFileSync(process.env.DBM_TEST_CONFIG!, 'utf8'));
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
    await page.getByLabel('Benutzername').fill('postgres');
    await page.getByLabel('Passwort', { exact: true }).fill(config.postgres.password);
  }
  await page.getByRole('button', { name: 'Verbinden', exact: true }).click();
}

async function download(page: Page, format: string) {
  await page.getByLabel('Exportformat', { exact: true }).selectOption(format);
  const event = page.waitForEvent('download');
  await page.getByRole('button', { name: 'Ergebnis herunterladen', exact: true }).click();
  const file = await event;
  expect(file.suggestedFilename()).toBe(
    format === 'json' ? 'db-manager-result.json' : 'db-manager-result.csv',
  );
  const path = await file.path();
  expect(path).not.toBeNull();
  return readFileSync(path!, 'utf8');
}
for (const driver of ['sqlite', 'postgresql']) {
  test(`${driver}: actual downloads preserve visible typed snapshot CSV choice and page bounds without new requests`, async ({
    page,
  }) => {
    await page.addInitScript(() => {
      const active = new Set<string>();
      (window as unknown as { exportCreates: number }).exportCreates = 0;
      const create = URL.createObjectURL.bind(URL);
      const revoke = URL.revokeObjectURL.bind(URL);
      (window as unknown as { exportURLs: Set<string> }).exportURLs = active;
      URL.createObjectURL = (blob) => {
        (window as unknown as { exportCreates: number }).exportCreates++;
        const url = create(blob);
        active.add(url);
        return url;
      };
      URL.revokeObjectURL = (url) => {
        active.delete(url);
        revoke(url);
      };
    });
    await connect(page, driver);
    // The table view refreshes insert eligibility asynchronously (observed after networkidle); as does the capability refresh; neither is an export request.
    const requests: string[] = [];
    page.on('request', (request) => {
      if (
        new URL(request.url()).pathname.startsWith('/api/') &&
        !/\/(insert-eligibility|tables\/eligibility|capabilities)$/.test(new URL(request.url()).pathname)
      )
        requests.push(`${request.method()} ${new URL(request.url()).pathname}`);
    });
    const binary = driver === 'sqlite' ? "X'00FF'" : "decode('00ff','hex')";
    const decimal =
      driver === 'sqlite'
        ? "'12345678901234567890.12345678901234567890'"
        : "CAST('12345678901234567890.12345678901234567890' AS NUMERIC(40,20))";
    const statement = `SELECT 9223372036854775807 AS same, NULL AS same, '' AS empty_text, ${decimal} AS precise, ${binary} AS payload, '=1+2' AS formula, 'a,"b' AS quoted, '\\N' AS marker`;
    await page.getByRole('textbox', { name: 'SQL-Abfrage', exact: true }).fill(statement);
    await page.getByRole('button', { name: 'Abfrage ausführen' }).click();
    await expect(page.getByRole('button', { name: 'Ergebnis herunterladen' })).toBeVisible();
    // Late catalog/state requests of the app may still be in flight; settle first so only downloads are measured.
    await page.waitForLoadState('networkidle');
    const before = requests.length;
    const json = JSON.parse(await download(page, 'json'));
    expect(requests.slice(before)).toEqual([]);
    expect(json.scope).toEqual({ kind: 'sql_result' });
    expect(json.schema_version).toBe(1);
    expect(json.truncated).toBe(false);
    expect(json.columns.map((column: { name: string }) => column.name)).toEqual([
      'same',
      'same',
      'empty_text',
      'precise',
      'payload',
      'formula',
      'quoted',
      'marker',
    ]);
    expect(json.rows[0].slice(0, 3)).toEqual([
      { kind: 'integer', value: '9223372036854775807' },
      { kind: 'null' },
      { kind: 'text', value: '' },
    ]);
    expect(json.rows[0][3]).toEqual({
      kind: driver === 'sqlite' ? 'text' : 'decimal',
      value: '12345678901234567890.12345678901234567890',
    });
    expect(json.rows[0][4]).toEqual({ kind: 'binary', value: 'AP8=' });
    const raw = await download(page, 'csv-raw');
    expect(raw).toContain('"=1+2"');
    expect(raw).toContain(',\\N,"",');
    expect(raw).toContain('"a,""b"');
    expect(raw.startsWith('"same","same",')).toBe(true);
    const spreadsheet = await download(page, 'csv-spreadsheet');
    expect(spreadsheet.startsWith('"\'same","\'same",')).toBe(true);
    expect(spreadsheet).toContain('"\'=1+2"');
    expect(requests.slice(before)).toEqual([]);
    expect(
      await page.evaluate(() => (window as unknown as { exportURLs: Set<string> }).exportURLs.size),
    ).toBe(1);

    // QA-F6-01: actual DB control characters remain exact in JSON but never start CSV.
    const controls = driver === 'sqlite' ? [0, 1, 9] : [1, 9]; // PostgreSQL text prohibits NUL.
    const chars = controls
      .map((code) => `${driver === 'sqlite' ? 'char' : 'chr'}(${code})`)
      .join(' || ');
    await page
      .getByRole('textbox', { name: 'SQL-Abfrage', exact: true })
      .fill(`SELECT ${chars} AS controls`);
    await page.getByRole('button', { name: 'Abfrage ausführen' }).click();
    await expect(page.getByRole('button', { name: 'Ergebnis herunterladen' })).toBeVisible();
    const controlled = JSON.parse(await download(page, 'json'));
    expect(controlled.rows[0][0]).toEqual({
      kind: 'text',
      value: String.fromCharCode(...controls),
    });
    for (const variant of ['csv-raw', 'csv-spreadsheet']) {
      const creates = await page.evaluate(
        () => (window as unknown as { exportCreates: number }).exportCreates,
      );
      await page.getByLabel('Exportformat', { exact: true }).selectOption(variant);
      await page.getByRole('button', { name: 'Ergebnis herunterladen' }).click();
      await expect(page.getByRole('alert')).toContainText('JSON unverändert');
      expect(
        await page.evaluate(() => (window as unknown as { exportCreates: number }).exportCreates),
      ).toBe(creates);
    }
    const char = driver === 'sqlite' ? 'char' : 'chr';
    await page
      .getByRole('textbox', { name: 'SQL-Abfrage', exact: true })
      .fill(`SELECT ${char}(13)||${char}(10)||'"'||${char}(92)||'N' AS allowed`);
    await page.getByRole('button', { name: 'Abfrage ausführen' }).click();
    await expect(page.getByRole('button', { name: 'Ergebnis herunterladen' })).toBeVisible();
    const allowed = JSON.parse(await download(page, 'json'));
    expect(allowed.rows[0][0]).toEqual({
      kind: 'text',
      value: '\r\n"\\N',
    });
    const allowedRaw = await download(page, 'csv-raw');
    expect(allowedRaw).toContain('\r\n');
    expect(allowedRaw).toContain('""\\N');
    await download(page, 'csv-spreadsheet');
    // A new SQL result destroys the previous export controls and URL.
    await page.getByLabel('Zeilenlimit', { exact: true }).selectOption('100');
    await page
      .getByRole('textbox', { name: 'SQL-Abfrage', exact: true })
      .fill('SELECT id FROM "odd""name" ORDER BY id');
    await page.getByRole('button', { name: 'Abfrage ausführen' }).click();
    await expect(page.getByText(/Nur dieses angezeigte Ergebnis: 100 Zeilen/)).toBeVisible();
    expect(
      await page.evaluate(() => (window as unknown as { exportURLs: Set<string> }).exportURLs.size),
    ).toBe(0);
    const limited = JSON.parse(await download(page, 'json'));
    expect(limited.truncated).toBe(true);
    expect(limited.rows).toHaveLength(100);
    await page.getByRole('button', { name: /odd"name$/ }).click();
    await expect(page.getByText(/Nur geladene Seite: 100 Zeilen/)).toBeVisible();
    await page.getByLabel('Zeilen pro Seite').selectOption('25');
    await page.getByRole('button', { name: 'Anwenden', exact: true }).click();
    await expect(page.getByText(/Nur geladene Seite: 25 Zeilen ab Position 1/)).toBeVisible();
    await page.getByRole('button', { name: 'Weiter', exact: true }).click();
    await expect(page.getByText(/Nur geladene Seite: 25 Zeilen ab Position 26/)).toBeVisible();
    await page.waitForLoadState('networkidle');
    const pageBefore = requests.length;
    const paged = JSON.parse(await download(page, 'json'));
    expect(requests.slice(pageBefore)).toEqual([]);
    expect(paged.scope).toEqual({ kind: 'table_page', offset: 25, page_size: 25, has_more: true });
    expect(paged.rows).toHaveLength(25);
    const serialized = JSON.stringify(paged);
    expect(serialized).not.toContain(config.token);
    expect(serialized).not.toContain(config.postgres.password);
    expect(Object.keys(paged).sort()).toEqual([
      'columns',
      'rows',
      'schema_version',
      'scope',
      'truncated',
    ]);
    const started = page.waitForEvent('download');
    await page.getByRole('button', { name: 'Ergebnis herunterladen', exact: true }).click();
    const pendingFile = await started;
    await page.getByRole('button', { name: 'Backend-Zugang' }).click();
    await page.getByLabel('Zugriffstoken').fill(config.invalidToken);
    await page.getByRole('button', { name: 'Übernehmen' }).click();
    await expect(page.getByRole('button', { name: 'Ergebnis herunterladen' })).toHaveCount(0);
    expect(
      await page.evaluate(() => (window as unknown as { exportURLs: Set<string> }).exportURLs.size),
    ).toBe(0);
    expect(JSON.parse(readFileSync((await pendingFile.path())!, 'utf8'))).toEqual(paged);
    expect(await page.evaluate(() => window.cspViolations)).toEqual([]);
  });
}
