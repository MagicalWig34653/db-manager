// SPDX-License-Identifier: AGPL-3.0-only
// Real-browser check of the embedded PWA against a running Go server.
// Usage: PLAYWRIGHT_CORE=<path to playwright-core/index.mjs> node scripts/check-pwa-embedded.mjs http://127.0.0.1:PORT
const { chromium } = await import(process.env.PLAYWRIGHT_CORE ?? 'playwright-core');
const base = process.argv[2];
const out = {};
const browser = await chromium.launch();
const ctx = await browser.newContext();
const page = await ctx.newPage();
const violations = [], apiHits = [];
page.on('console', m => { if (/Content Security Policy|violat/i.test(m.text())) violations.push(m.text()); });
await page.addInitScript(() => document.addEventListener('securitypolicyviolation', e => console.log('CSP violation ' + e.violatedDirective + ' ' + e.blockedURI)));
await page.goto(base + '/');
await page.waitForFunction(() => navigator.serviceWorker.ready.then(() => true));
await page.evaluate(() => navigator.serviceWorker.ready);
await page.reload(); await page.waitForTimeout(500);
const sw = await page.evaluate(async () => { const r = await navigator.serviceWorker.ready; return { scope: r.scope, state: r.active?.state, controlled: !!navigator.serviceWorker.controller }; });
out.sw = sw;
const cdp = await ctx.newCDPSession(page);
const inst = await cdp.send('Page.getInstallabilityErrors');
out.installabilityErrors = inst.installabilityErrors;
const mf = await cdp.send('Page.getAppManifest');
out.manifest = { url: mf.url, errors: mf.errors.length };
// headers
const hdr = {};
for (const p of ['/', '/index.html', '/manifest.webmanifest', '/sw.js', '/icon.svg', '/icon-192.png', '/icon-512.png', '/icon-maskable-512.png', '/apple-touch-icon.png', '/favicon-32.png', '/healthz']) {
  const r = await ctx.request.get(base + p); const h = r.headers();
  hdr[p] = [r.status(), h['content-type'], h['cache-control'], h['x-content-type-options'], h['service-worker-allowed'] ?? 'absent'].join(' | ');
}
const csp = (await ctx.request.get(base + '/')).headers()['content-security-policy'];
out.headers = hdr; out.csp = csp;
// API never from cache: online request goes to network; offline fails
await ctx.route('**/healthz', r => r.continue());
const cacheKeys = await page.evaluate(async () => { const o = {}; for (const k of await caches.keys()) o[k] = (await (await caches.open(k)).keys()).map(r => new URL(r.url).pathname); return o; });
out.cached = cacheKeys;
out.apiInCache = Object.values(cacheKeys).flat().some(p => p.startsWith('/api') || p.startsWith('/healthz') || p.startsWith('/readyz'));
const reqs = []; page.on('requestfinished', async r => { if (/\/(api|healthz)/.test(r.url())) reqs.push(r.url()); });
const before = await page.evaluate(() => fetch('/healthz', { cache: 'no-store' }).then(r => r.status));
await ctx.setOffline(true);
const off = await page.evaluate(() => fetch('/healthz', { cache: 'no-store' }).then(r => 'served ' + r.status, e => 'network-error'));
const offApi = await page.evaluate(() => fetch('/api/v1/connections/test', { method: 'POST', headers: { Authorization: 'Bearer x' }, body: '{}' }).then(r => 'served ' + r.status, e => 'network-error'));
out.api = { onlineHealthz: before, offlineHealthz: off, offlineApi: offApi };
// offline restart shows shell
const p2 = await ctx.newPage(); p2.on('console', m => { if (/violat/i.test(m.text())) violations.push(m.text()); });
await p2.goto(base + '/'); await p2.waitForTimeout(1000);
out.offlineShell = { title: await p2.title(), html: (await p2.content()).includes('lang="de"'), text: (await p2.locator('body').innerText()).slice(0, 200) };
await ctx.setOffline(false);
out.cspViolations = violations;
console.log(JSON.stringify(out, null, 1));
await browser.close();
