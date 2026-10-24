// SPDX-License-Identifier: AGPL-3.0-only
// Real-browser check of the embedded PWA against a running Go server; exits non-zero on any failed assertion.
// Usage: PLAYWRIGHT_CORE=<path to playwright-core/index.mjs> node scripts/check-pwa-embedded.mjs http://127.0.0.1:PORT
import assert from 'node:assert/strict';

const { chromium } = await import(process.env.PLAYWRIGHT_CORE ?? 'playwright-core');
const base = process.argv[2];
assert.ok(base, 'base URL argument required');
const browser = await chromium.launch();
try {
  const ctx = await browser.newContext();
  const page = await ctx.newPage();
  const violations = [];
  page.on('console', (m) => /violat/i.test(m.text()) && violations.push(m.text()));
  await page.addInitScript(() =>
    document.addEventListener('securitypolicyviolation', (e) =>
      console.log('CSP violation ' + e.violatedDirective + ' ' + e.blockedURI),
    ),
  );
  await page.goto(base + '/');
  await page.evaluate(() => navigator.serviceWorker.ready);
  await page.reload();
  await page.waitForTimeout(500);
  const sw = await page.evaluate(async () => {
    const r = await navigator.serviceWorker.ready;
    return { scope: r.scope, state: r.active?.state, controlled: !!navigator.serviceWorker.controller };
  });
  assert.equal(sw.scope, base + '/');
  assert.equal(sw.state, 'activated');
  assert.ok(sw.controlled, 'page is not controlled by the service worker');

  const cdp = await ctx.newCDPSession(page);
  assert.deepEqual((await cdp.send('Page.getInstallabilityErrors')).installabilityErrors, []);
  assert.equal((await cdp.send('Page.getAppManifest')).errors.length, 0);

  const types = {
    '/manifest.webmanifest': 'application/manifest+json',
    '/sw.js': 'text/javascript; charset=utf-8',
    '/icon.svg': 'image/svg+xml',
    '/icon-192.png': 'image/png',
    '/icon-512.png': 'image/png',
    '/icon-maskable-512.png': 'image/png',
    '/apple-touch-icon.png': 'image/png',
    '/favicon-32.png': 'image/png',
  };
  for (const [path, type] of Object.entries(types)) {
    const r = await ctx.request.get(base + path);
    const h = r.headers();
    assert.equal(r.status(), 200, path);
    assert.equal(h['content-type'], type, path);
    assert.equal(h['cache-control'], 'no-store', path);
    assert.equal(h['x-content-type-options'], 'nosniff', path);
    assert.equal(h['service-worker-allowed'], undefined, path);
  }
  const csp = (await ctx.request.get(base + '/')).headers()['content-security-policy'];
  assert.match(csp, /manifest-src 'self'; worker-src 'self'/);
  assert.doesNotMatch(csp, /unsafe-eval|script-src[^;]*unsafe-inline/);

  const cached = await page.evaluate(async () => {
    const paths = [];
    for (const k of await caches.keys())
      for (const r of await (await caches.open(k)).keys()) paths.push(new URL(r.url).pathname);
    return paths;
  });
  const swText = await (await ctx.request.get(base + '/sw.js')).text();
  const shell = JSON.parse(/JSON\.parse\('(\[[^']*\])'\)/.exec(swText)?.[1] ?? 'null');
  assert.ok(Array.isArray(shell) && shell.some((f) => f.startsWith('assets/')), 'SHELL list not found in sw.js');
  assert.deepEqual([...new Set(cached)].sort(), shell.map((f) => '/' + f).sort(), 'worker cache differs from SHELL');
  assert.ok(cached.includes('/index.html'));
  assert.ok(!cached.some((p) => /^\/(api|healthz|readyz)/.test(p)), 'API paths cached');

  assert.equal(await page.evaluate(() => fetch('/healthz', { cache: 'no-store' }).then((r) => r.status)), 200);
  await ctx.setOffline(true);
  const failing = (url, init) =>
    page.evaluate(([u, i]) => fetch(u, i).then(() => 'served', () => 'network-error'), [url, init]);
  assert.equal(await failing('/healthz', { cache: 'no-store' }), 'network-error');
  assert.equal(
    await failing('/api/v1/connections/test', { method: 'POST', headers: { Authorization: 'Bearer x' }, body: '{}' }),
    'network-error',
  );
  for (const file of shell) {
    const status = await page.evaluate((u) => fetch(u).then((r) => r.status, () => 0), '/' + file);
    assert.equal(status, 200, 'offline shell file unavailable: ' + file);
  }
  const offline = await ctx.newPage();
  offline.on('console', (m) => /violat/i.test(m.text()) && violations.push(m.text()));
  await offline.goto(base + '/');
  await offline.waitForTimeout(1000);
  assert.ok((await offline.content()).includes('lang="de"'), 'offline shell missing');
  assert.match(await offline.title(), /DB Manager/);
  await ctx.setOffline(false);
  assert.deepEqual(violations, []);
  console.log('PWA embedded check passed.');
} catch (error) {
  console.error(error);
  process.exitCode = 1;
} finally {
  await browser.close();
}
