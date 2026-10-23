// SPDX-License-Identifier: AGPL-3.0-only
import { chromium, expect } from '@playwright/test';
import { readFileSync, mkdirSync } from 'node:fs';
import { fileURLToPath } from 'node:url';
const config = JSON.parse(readFileSync(process.env.DBM_CAPTURE_CONFIG, 'utf8'));
const output = fileURLToPath(new URL('../assets/screenshots/', import.meta.url));
mkdirSync(output, { recursive: true });
const browser = await chromium.launch();
try {
  const page = await browser.newPage({ viewport: { width: 1440, height: 1000 }, deviceScaleFactor: 1 });
  await page.goto(config.base);
  await page.getByRole('button', { name: 'Backend-Zugang' }).click();
  await page.getByLabel('Zugriffstoken').fill(config.token);
  await page.getByRole('button', { name: 'Übernehmen' }).click();
  await page.getByRole('button', { name: 'Verbindung hinzufügen', exact: true }).first().click();
  await page.getByLabel('Name optional').fill('Projektübersicht · Demo');
  await page.getByLabel('Datenbank', { exact: true }).fill('pages_demo');
  await page.getByRole('button', { name: 'Verbinden', exact: true }).click();
  await page.getByRole('button', { name: /projekte$/ }).click();
  await expect(page.getByRole('button', { name: 'Zeile hinzufügen', exact: true })).toBeVisible();
  await expect(page.getByText('Website-Relaunch', { exact: true })).toBeVisible();
  const capture = async name => {
    await page.evaluate(() => document.fonts.ready);
    // Authentication UI must be closed; no secret values may appear in images.
    await expect(page.getByLabel('Zugriffstoken')).toHaveCount(0);
    await page.screenshot({ path: output + name, animations: 'disabled' });
  };
  await capture('tabellenansicht.png');
  await page.getByRole('button', { name: 'Zeile 1 bearbeiten', exact: true }).click();
  await page.getByLabel('Angabe für status', { exact: true }).selectOption('value');
  await page.getByLabel('Wert für status', { exact: true }).fill('Bereit zur Abnahme');
  await page.getByRole('button', { name: 'Änderung prüfen', exact: true }).click();
  await expect(page.getByRole('checkbox', { name: 'Ich bestätige diesen einzelnen Schreibvorgang.' })).toBeVisible();
  await capture('zeileneditor.png');
  await page.getByRole('button', { name: 'Schließen', exact: true }).click();
  await page.setViewportSize({ width: 1024, height: 900 });
  await capture('tablet-web.png');
  await page.setViewportSize({ width: 1440, height: 1000 });
  await page.getByRole('button', { name: 'SQL', exact: true }).click();
  await page.getByRole('textbox', { name: 'SQL-Abfrage', exact: true }).fill("SELECT projekt, status, budget, notiz\nFROM projekte\nORDER BY id;");
  await page.getByRole('button', { name: 'Abfrage ausführen' }).click();
  await expect(page.getByRole('button', { name: 'null: NULL' }).first()).toBeVisible();
  await capture('sql-ergebnis.png');
} finally {
  await browser.close();
}
