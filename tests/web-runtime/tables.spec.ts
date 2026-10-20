// SPDX-License-Identifier: AGPL-3.0-only
// Adapted from reviewed Web source 6c6eb33c93eed355a10cd114f61ce27be7f3f55e, tests/real/table.spec.ts.
import { test, expect } from '@playwright/test';
import { readFileSync } from 'node:fs';
const config = JSON.parse(readFileSync(process.env.DBM_TEST_CONFIG!, 'utf8'));
for (const driver of ['sqlite', 'postgresql']) {
  test(`${driver}: real structured browse pagination typed AND filters metadata and session`, async ({
    page,
  }) => {
    await page.setViewportSize(
      driver === 'sqlite' ? { width: 1440, height: 1000 } : { width: 1024, height: 768 },
    );
    let sqlRequests = 0;
    const requests: Record<string, unknown>[] = [];
    const origins = new Set<string>();
    page.on('request', (request) => {
      origins.add(new URL(request.url()).origin);
      if (request.url().endsWith('/api/v1/query')) sqlRequests++;
      if (request.url().endsWith('/api/v2/tables/browse')) requests.push(request.postDataJSON());
    });
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
    await page.getByRole('button', { name: 'Verbindung testen', exact: true }).click();
    await expect(
      page.getByText('Verbindung erfolgreich. Die Datenbank ist erreichbar.'),
    ).toBeVisible();
    await page.getByRole('button', { name: 'Verbinden', exact: true }).click();
    await page.getByRole('button', { name: /odd"name$/ }).click();
    const workspace = page.getByRole('region', { name: 'Tabellenarbeitsplatz', exact: true });
    await expect(workspace.getByText('Zeilen 1–100 · Gesamtzahl nicht ermittelt')).toBeVisible();
    await expect(workspace.getByRole('button', { name: 'null: NULL' }).first()).toBeVisible();
    await expect(workspace.getByRole('button', { name: 'text:', exact: true }).first()).toHaveText(
      '""',
    );
    await expect(workspace.getByRole('button', { name: 'binary: AP8=' }).first()).toBeVisible();
    await expect(
      workspace.getByText('12345678901234567890.12345678901234567890', { exact: true }).first(),
    ).toBeVisible();
    await page.getByLabel('Zeilen pro Seite').selectOption('25');
    await page.getByRole('button', { name: 'Anwenden', exact: true }).click();
    await expect(workspace.getByText('Zeilen 1–25 · Gesamtzahl nicht ermittelt')).toBeVisible();
    // QA-F3-01: a rejected draft must never change the applied query.
    await page.getByText('Sortierung und Filter', { exact: true }).click();
    await page.getByRole('button', { name: 'Filter hinzufügen' }).click();
    await page.getByLabel('Filterspalte 1').selectOption('empty_text');
    await page.getByLabel('Filterwert 1').fill('qa-never-present');
    await page.getByRole('button', { name: 'Sortierung hinzufügen' }).click();
    await page.getByLabel('Sortierspalte 1').selectOption('id');
    await page.getByRole('button', { name: 'Sortierung hinzufügen' }).click();
    await page.getByLabel('Sortierspalte 2').selectOption('id');
    await page.getByLabel('Zeilen pro Seite').selectOption('500');
    const beforeRejectedApply = requests.length;
    await page.getByRole('button', { name: 'Anwenden', exact: true }).click();
    await expect(workspace.getByRole('alert')).toHaveText(
      'Jede Sortierspalte darf nur einmal vorkommen.',
    );
    expect(requests.length).toBe(beforeRejectedApply);
    await page.getByRole('button', { name: 'Filter 1 entfernen' }).click();
    await page.getByRole('button', { name: 'Sortierung 2 entfernen' }).click();
    await page.getByRole('button', { name: 'Sortierung 1 entfernen' }).click();
    await page.getByLabel('Zeilen pro Seite').selectOption('25');
    await page.getByText('Sortierung und Filter', { exact: true }).click();
    await page.getByRole('button', { name: 'Weiter', exact: true }).click();
    await expect(workspace.getByText('Zeilen 26–50 · Gesamtzahl nicht ermittelt')).toBeVisible();
    expect(requests.at(-1)).toMatchObject({ offset: 25, page_size: 25, filters: [], sort: [] });
    const beforeRefresh = requests.length;
    await page.getByRole('button', { name: 'Aktualisieren', exact: true }).click();
    await expect.poll(() => requests.length).toBe(beforeRefresh + 1);
    await expect(workspace.getByText('Zeilen 26–50 · Gesamtzahl nicht ermittelt')).toBeVisible();
    expect(requests.at(-1)).toMatchObject({ offset: 25, page_size: 25, filters: [], sort: [] });
    await page.getByRole('button', { name: 'Zurück', exact: true }).click();
    await expect(workspace.getByText('Zeilen 1–25 · Gesamtzahl nicht ermittelt')).toBeVisible();
    await page.getByText('Sortierung und Filter', { exact: true }).click();
    await page.getByRole('button', { name: 'Sortierung hinzufügen' }).click();
    await page.getByLabel('Sortierspalte 1').selectOption('id');
    await page.getByLabel('Richtung 1').selectOption('desc');
    await page.getByRole('button', { name: 'Anwenden', exact: true }).click();
    await expect(
      workspace.getByRole('button', { name: 'integer: 9223372036854775807' }),
    ).toBeVisible();
    await page.getByRole('button', { name: 'Sortierung hinzufügen' }).click();
    await page.getByLabel('Sortierspalte 1').selectOption('empty_text');
    await page.getByLabel('Richtung 1').selectOption('asc');
    await page.getByLabel('Sortierspalte 2').selectOption('id');
    await page.getByLabel('Richtung 2').selectOption('desc');
    await page.getByRole('button', { name: 'Anwenden', exact: true }).click();
    await expect(
      workspace.getByRole('button', { name: 'integer: 9223372036854775807' }),
    ).toBeVisible();
    expect(requests.at(-1)?.sort).toEqual([
      { column: 'empty_text', direction: 'asc' },
      { column: 'id', direction: 'desc' },
    ]);
    await page.getByRole('button', { name: 'Filter hinzufügen' }).click();
    await page.getByLabel('Filterspalte 1').selectOption('nullable');
    await page.getByLabel('Bedingung 1').selectOption('is_null');
    await page.getByRole('button', { name: 'Filter hinzufügen' }).click();
    await page.getByLabel('Filterspalte 2').selectOption('empty_text');
    const before = requests.length;
    await page.getByLabel('Filterwert 2').fill('temporary');
    await page.getByLabel('Filterwert 2').fill('');
    await page.waitForTimeout(150);
    expect(requests.length).toBe(before);
    await page.getByRole('button', { name: 'Anwenden', exact: true }).click();
    await expect(workspace.getByRole('button', { name: 'null: NULL' }).first()).toBeVisible();
    expect(requests.at(-1)?.filters).toEqual([
      { column: 'nullable', operator: 'is_null' },
      { column: 'empty_text', operator: 'eq', value: { kind: 'text', value: '' } },
    ]);
    await page.getByLabel('Bedingung 1').selectOption('is_not_null');
    await page.getByRole('button', { name: 'Anwenden', exact: true }).click();
    await expect(workspace.getByRole('button', { name: 'text: present' }).first()).toBeVisible();
    await expect(workspace.getByRole('button', { name: 'null: NULL' })).toHaveCount(0);
    await page.getByRole('button', { name: 'Filter 2 entfernen' }).click();
    await page.getByRole('button', { name: 'Filter 1 entfernen' }).click();
    await page.getByRole('button', { name: 'Filter hinzufügen' }).click();
    await page.getByLabel('Filterspalte 1').selectOption('huge');
    await page.getByLabel('Werttyp 1').selectOption(driver === 'sqlite' ? 'text' : 'decimal');
    await page.getByLabel('Filterwert 1').fill('12345678901234567890.12345678901234567890');
    await page.getByRole('button', { name: 'Anwenden', exact: true }).click();
    await expect(
      workspace.getByRole('button', { name: 'integer: 9223372036854775807' }),
    ).toBeVisible();
    expect((requests.at(-1)?.filters as { value: { value: string } }[])[0].value.value).toBe(
      '12345678901234567890.12345678901234567890',
    );
    await page.getByLabel('Filterspalte 1').selectOption('empty_text');
    await page.getByLabel('Werttyp 1').selectOption('text');
    await page.getByLabel('Filterwert 1').fill("' OR 1=1 --");
    await page.getByRole('button', { name: 'Anwenden', exact: true }).click();
    await expect(
      workspace.getByText('Keine passenden Zeilen · Gesamtzahl nicht ermittelt'),
    ).toBeVisible();
    expect((requests.at(-1)?.filters as { value: { value: string } }[])[0].value.value).toBe(
      "' OR 1=1 --",
    );
    await page.getByRole('tab', { name: /Struktur/ }).click();
    await expect(workspace.getByRole('columnheader', { name: 'PK-Position' })).toBeVisible();
    await expect(workspace.getByRole('rowheader', { name: 'generated_text' })).toBeVisible();
    await expect(
      workspace.getByRole('rowheader', { name: 'nullable', exact: true }).locator('..'),
    ).toContainText('fallback');
    await expect(
      workspace.getByRole('cell', {
        name: driver === 'sqlite' ? 'Virtuell' : 'Gespeichert',
        exact: true,
      }),
    ).toBeVisible();
    await expect(workspace.getByText('Primärschlüssel: id', { exact: true })).toBeVisible();
    await page.getByRole('button', { name: /f3_view\s*View$/ }).click();
    await expect(
      workspace.getByText('Primärschlüssel: Nicht vorhanden', { exact: true }),
    ).toBeVisible();
    await expect(workspace.getByText(/Keine stabile Reihenfolge zugesagt/)).toBeVisible();
    await page.getByRole('button', { name: 'Backend-Zugang' }).click();
    await page.getByLabel('Zugriffstoken').fill(config.invalidToken);
    await page.getByRole('button', { name: 'Übernehmen' }).click();
    await expect(workspace).not.toBeVisible();
    await expect(page.getByText('Nicht verbunden', { exact: true })).toBeVisible();
    await expect(page.getByRole('textbox', { name: 'SQL-Abfrage', exact: true })).toHaveValue('');
    await expect(page.getByRole('button', { name: /odd"name$/ })).not.toBeVisible();
    expect(sqlRequests).toBe(0);
    expect(requests.every((request) => !('sql' in request))).toBe(true);
    expect([...origins]).toEqual([config.baseURL]);
    expect(await page.evaluate(() => window.cspViolations)).toEqual([]);
    const storage = await page.evaluate(() =>
      JSON.stringify({
        local: { ...localStorage },
        session: { ...sessionStorage },
        cookies: document.cookie,
        url: location.href,
      }),
    );
    expect(storage.includes(config.token) || storage.includes(config.postgres.password)).toBe(false);
  });
}
