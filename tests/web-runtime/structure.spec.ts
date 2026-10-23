// SPDX-License-Identifier: AGPL-3.0-only
// Adapted from reviewed Web source 8ef738d22a1164291e2c8183d80e72d1f043c0f8, tests/real/structure.spec.ts.
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

for (const driver of ['sqlite', 'postgresql']) {
  test(`${driver}: actual index terms constraints coverage compatibility and late-session response`, async ({
    page,
  }) => {
    await connect(page, driver);
    let sqlRequests = 0;
    let writes = 0;
    const structureRequests: unknown[] = [];
    page.on('request', (request) => {
      if (request.url().endsWith('/api/v1/query')) sqlRequests++;
      if (/\/api\/v[34]\/rows\//.test(request.url())) writes++;
      if (request.url().endsWith('/api/v5/tables/structure'))
        structureRequests.push(request.postDataJSON());
    });
    await page.getByRole('button', { name: /f5_child$/ }).click();
    const response = page.waitForResponse(
      (r) => r.url().endsWith('/api/v5/tables/structure') && r.request().method() === 'POST',
    );
    await page.getByRole('tab', { name: /Struktur/ }).click();
    const actualResponse = await response;
    expect(actualResponse.status()).toBe(200);
    const panel = page.getByRole('region', { name: 'Index- und Constraintstruktur', exact: true });
    const expression = panel.getByRole('article', {
      name: 'Index f5_expression_partial',
      exact: true,
    });
    await expect(expression).toBeVisible();
    // Read the real catalog independently after the UI has consumed its response;
    // Avoid depending on the lifetime of Chromium CDP's response-body cache.
    const observed = structureRequests[0] as { connection: unknown; table: unknown };
    const probe = await page.request.post('/api/v5/tables/structure', {
      headers: { Authorization: 'Bearer ' + config.token },
      data: observed,
    });
    expect(probe.status()).toBe(200);
    const actual = await probe.json();

    await expect(
      expression.getByText('Ausdruck (Definition nicht geliefert)', { exact: true }),
    ).toBeVisible();
    await expect(expression.getByText('Absteigend', { exact: true }).first()).toBeVisible();
    await expect(
      expression.locator('dt').filter({ hasText: 'Teilindex' }).locator('..'),
    ).toContainText('Ja');
    const fk = panel.getByRole('article', { name: /^Fremdschlüssel/ });
    await expect(fk).toContainText('parent_id');
    await expect(fk).toContainText('f5_parent');
    await expect(fk).toContainText('Weitergeben (CASCADE)');
    await expect(fk).toContainText('Auf NULL setzen');
    await expect(fk.getByRole('link')).toHaveCount(0);
    if (driver === 'sqlite') {
      expect(actual.coverage).toEqual({
        indexes: 'catalog',
        constraints: 'partial',
        checks: 'unavailable',
      });
      await expect(panel.getByText('Teilweise erfasst', { exact: true })).toBeVisible();
      await expect(panel.getByText('Nicht verfügbar', { exact: true })).toBeVisible();
      await expect(expression.getByText('Hilfsterm', { exact: true })).toBeVisible();
      await expect(expression.getByText('Zeilenkennung (rowid)', { exact: true })).toBeVisible();
      await expect(panel.getByRole('article', { name: /^Prüfbedingung/ })).toHaveCount(0);
      await expect(
        fk.locator('dt').filter({ hasText: 'Anfangs aufgeschoben' }).locator('..'),
      ).toContainText('Unbekannt');
    } else {
      expect(actual.coverage.checks).toBe('catalog');
      await expect(
        panel
          .getByRole('article', { name: 'Index f5_include', exact: true })
          .getByText('Mitgeführter Wert (INCLUDE)', { exact: true }),
      ).toBeVisible();
      await expect(
        panel.getByRole('article', { name: 'Prüfbedingung (CHECK) child_check', exact: true }),
      ).toContainText('label');
      await expect(
        fk.locator('dt').filter({ hasText: 'Anfangs aufgeschoben' }).locator('..'),
      ).toContainText('Ja');
    }
    expect(structureRequests).toHaveLength(1);
    expect(structureRequests[0]).not.toHaveProperty('sql');
    expect(sqlRequests).toBe(0);
    expect(writes).toBe(0);
    if (driver === 'sqlite') {
      await page.getByRole('button', { name: /f5_empty_identifier$/ }).click();
      await page.getByRole('tab', { name: /Struktur/ }).click();
      const primary = panel.getByRole('article', { name: /^Primärschlüssel/ });
      await expect(primary).toBeVisible();
      await expect(
        primary
          .locator('dt')
          .filter({ hasText: /^Spalten$/ })
          .locator('..'),
      ).toContainText('""');
      await page.getByRole('button', { name: /f5_child$/ }).click();
      await page.getByRole('tab', { name: /Struktur/ }).click();
      await expect(expression).toBeVisible();
    }
    await page.getByRole('button', { name: /f5_view\s*View$/ }).click();
    await page.getByRole('tab', { name: /Struktur/ }).click();
    await expect(page.getByRole('rowheader', { name: 'label', exact: true })).toBeVisible();
    await expect(
      page.getByText(
        'Für diese Objektart gibt es keine erweiterten Strukturangaben. Die Spaltenstruktur bleibt verfügbar.',
        { exact: true },
      ),
    ).toBeVisible();
    // Explicit HTTP404 compatibility boundary; core metadata above came from the real backend.
    await page.route('**/api/v5/capabilities', (route) =>
      route.fulfill({
        status: 404,
        contentType: 'application/json',
        body: JSON.stringify({
          error: { code: 'not_found', message: 'Synthetic legacy boundary' },
        }),
      }),
    );
    await page.getByRole('button', { name: /f5_child$/ }).click();
    await page.getByRole('tab', { name: /Struktur/ }).click();
    await expect(
      page.getByText(
        'Diese Instanz bietet keine erweiterten Index-/Constraintangaben. Die Spaltenstruktur bleibt verfügbar.',
        { exact: true },
      ),
    ).toBeVisible();
    await expect(page.getByRole('rowheader', { name: 'label', exact: true })).toBeVisible();
    await page.getByRole('tab', { name: 'Daten', exact: true }).click();
    await expect(page.getByRole('button', { name: 'Ergebnis herunterladen' })).toBeVisible();
    await page.unroute('**/api/v5/capabilities');
    await page.getByRole('tab', { name: /Struktur/ }).click();
    await expect(expression).toBeVisible();
    let release!: () => void;
    const barrier = new Promise<void>((resolve) => {
      release = resolve;
    });
    let held = false;
    await page.route('**/api/v5/tables/structure', async (route) => {
      const response = await route.fetch();
      expect(response.status()).toBe(200);
      held = true;
      await barrier;
      try {
        await route.fulfill({ response });
      } catch {
        /* Old session already aborted. */
      }
    });
    await page.getByRole('button', { name: 'Erweiterte Struktur neu laden', exact: true }).click();
    await expect.poll(() => held).toBe(true);
    await page.getByRole('button', { name: 'Backend-Zugang' }).click();
    await page.getByLabel('Zugriffstoken').fill(config.invalidToken);
    await page.getByRole('button', { name: 'Übernehmen' }).click();
    release();
    await expect(panel).toHaveCount(0);
    await expect(page.getByText('f5_expression_partial', { exact: true })).toHaveCount(0);
    await expect(page.getByText('Nicht verbunden', { exact: true })).toBeVisible();
    expect(await page.evaluate(() => window.cspViolations)).toEqual([]);
  });
}
