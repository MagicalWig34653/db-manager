// SPDX-License-Identifier: AGPL-3.0-only
import { chromium, expect } from '@playwright/test';
import AxeBuilder from '@axe-core/playwright';
import { createServer } from 'node:http';
import { readFile, stat, mkdir } from 'node:fs/promises';
import path from 'node:path';
const root = path.resolve('dist');
const server = createServer(async (request, response) => {
  try {
    const pathname = decodeURIComponent(new URL(request.url, 'http://localhost').pathname);
    let file = path.resolve(root, '.' + pathname);
    if (!file.startsWith(root + path.sep)) throw new Error('path');
    if ((await stat(file)).isDirectory()) file = path.join(file, 'index.html');
    const types = { '.html': 'text/html; charset=utf-8', '.css': 'text/css', '.js': 'application/javascript', '.json': 'application/json', '.png': 'image/png', '.svg': 'image/svg+xml' };
    response.setHeader('Content-Type', types[path.extname(file)] ?? 'text/plain');
    response.end(await readFile(file));
  } catch {
    response.writeHead(404).end('Not found');
  }
});
await new Promise(resolve => server.listen(0, '127.0.0.1', resolve));
const base = `http://127.0.0.1:${server.address().port}`;
const browser = await chromium.launch();
const violations = [];
const errors = [];
const foreign = [];
try {
  const context = await browser.newContext();
  await context.addInitScript(() => {
    window.sitePolicyViolations = [];
    document.addEventListener('securitypolicyviolation', event => window.sitePolicyViolations.push(event.violatedDirective));
  });
  const page = await context.newPage();
  page.on('pageerror', error => errors.push(error.message));
  page.on('request', request => { if (new URL(request.url()).origin !== base) foreign.push(request.url()); });
  await mkdir('test-results', { recursive: true });
  await page.setViewportSize({ width: 320, height: 960 });
  await page.goto(`${base}/db-manager/`);
  const spacedNavigation = await page.evaluate(() => {
    const sheet = [...document.styleSheets].find(sheet => sheet.href?.endsWith('site.css'));
    const index = sheet.insertRule('.header nav { letter-spacing: 0.12em; }', sheet.cssRules.length);
    try {
      return { viewport: innerWidth, document: document.documentElement.scrollWidth };
    } finally {
      sheet.deleteRule(index);
    }
  });
  console.log(`320px spaced navigation: ${JSON.stringify(spacedNavigation)}`);
  expect(spacedNavigation.document, 'Navigation must reflow with wider text metrics').toBeLessThanOrEqual(spacedNavigation.viewport + 1);
  for (const destination of ['docs/PARITAET/', 'docs/STATUS/', 'docs/ARBEITSPLAN/', 'docs/security/VAULT-FORMAT-V1/']) {
    const response = await page.goto(`${base}/db-manager/${destination}`);
    expect(response.status()).toBe(200);
    const metrics = await page.evaluate(() => ({ viewport: innerWidth, document: document.documentElement.scrollWidth }));
    console.log(`320px long-document ${destination}: ${JSON.stringify(metrics)}`);
    expect(metrics.document, `Document overflow at ${destination}`).toBeLessThanOrEqual(metrics.viewport + 1);
    const scrolling = await page.locator('main').evaluate(main => {
      const records = [...main.querySelectorAll('pre, table')].map(element => {
        const overflow = getComputedStyle(element).overflowX;
        element.scrollLeft = element.scrollWidth;
        return { overflow, hasOverflow: element.scrollWidth > element.clientWidth, scrollLeft: element.scrollLeft };
      });
      return { bodyClipped: getComputedStyle(document.body).overflowX === 'hidden', contentClipped: getComputedStyle(main).overflowX === 'hidden', records };
    });
    expect(scrolling.bodyClipped).toBe(false);
    expect(scrolling.contentClipped).toBe(false);
    for (const record of scrolling.records) {
      expect(['auto', 'scroll']).toContain(record.overflow);
      if (record.hasOverflow) expect(record.scrollLeft).toBeGreaterThan(0);
    }
    expect(await page.evaluate(() => window.sitePolicyViolations)).toEqual([]);
  }
  for (const width of [1440, 1024, 390]) {
    await page.setViewportSize({ width, height: 960 });
    for (const destination of ['', 'dokumentation/', 'roadmap/', 'EINRICHTUNG/', 'suche/', 'lizenzen/']) {
      const response = await page.goto(`${base}/db-manager/${destination}`);
      expect(response.status()).toBe(200);
      await expect(page.locator('main h1')).toHaveCount(1);
      expect(await page.evaluate(() => document.documentElement.scrollWidth <= innerWidth + 1)).toBe(true);
      expect(await page.evaluate(() => window.sitePolicyViolations)).toEqual([]);
      const axe = await new AxeBuilder({ page }).withTags(['wcag2a', 'wcag2aa', 'wcag21aa']).analyze();
      violations.push(...axe.violations.map(issue => `${width}/${destination}: ${issue.id}`));
    }
    await page.goto(`${base}/db-manager/`);
    await page.screenshot({ path: `test-results/site-${width}.png`, fullPage: true });
  }
  await page.goto(`${base}/db-manager/suche/`);
  await page.getByRole('searchbox').fill('SQLite TLS');
  await page.getByRole('button', { name: 'Suchen', exact: true }).click();
  await expect(page.locator('#search-results article').first()).toBeVisible();
  const href = await page.locator('#search-results article a').first().getAttribute('href');
  expect(href.startsWith(base + '/db-manager/')).toBe(true);
  await page.getByRole('searchbox').fill('<img src=x onerror=alert(1)>');
  await page.getByRole('button', { name: 'Suchen', exact: true }).click();
  await expect(page.getByText('Keine Treffer. Bitte andere Suchbegriffe versuchen.')).toBeVisible();
  await expect(page.locator('#search-results img')).toHaveCount(0);
  await page.goto(`${base}/db-manager/`);
  await page.keyboard.press('Tab');
  await expect(page.getByRole('link', { name: 'Zum Inhalt' })).toBeFocused();
  await page.keyboard.press('Enter');
  await expect(page.locator('main')).toBeFocused();
  expect(await page.evaluate(() => Object.keys(localStorage))).toEqual([]);
  expect(await context.cookies()).toEqual([]);
  expect(violations).toEqual([]);
  expect(errors).toEqual([]);
  expect(foreign).toEqual([]);
  console.log('18 responsive page checks, four 320px long-document regressions and wider navigation text; WCAG A/AA, keyboard, search/XSS, CSP, storage and zero external requests passed');
} finally {
  await browser.close();
  await new Promise(resolve => server.close(resolve));
}
