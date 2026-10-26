// SPDX-License-Identifier: AGPL-3.0-only
// Service worker template. The build (pwa plugin in vite.config.ts) replaces the two placeholders
// below and emits the result as sw.js. Scope: app shell only. The worker never answers /api,
// /healthz, /readyz or any request that is not an exact GET of a precached shell file, and it never
// stores a response in the cache other than during installation.
'use strict';

const BUILD_ID = '9545f752e56149ede6a07eea52f808691682b908';
const SHELL = /** @type {string[]} */ (JSON.parse('["index.html","manifest.webmanifest","icon.svg","icon-192.png","icon-512.png","icon-maskable-512.png","apple-touch-icon.png","favicon-32.png","assets/ConfirmDestructiveDialog-CSv6taNs.js","assets/PlanPreview-CXTuyKb9.js","assets/PlanPreview-DIdf5lVd.css","assets/circle-check-D0HqYeNO.js","assets/client-LSdyyzKw.js","assets/feature-7Lq1Bwpj.js","assets/feature-ADYQo2EN.css","assets/feature-B87oFnCN.js","assets/feature-BPqPrbnZ.css","assets/feature-BWF60C02.js","assets/feature-BXrbnqoD.css","assets/feature-Bb6ZvVSj.css","assets/feature-BcixFSIF.css","assets/feature-Bi78JpLS.js","assets/feature-BzsSHduT.css","assets/feature-C3VbvhN6.js","assets/feature-Cc6hvP7B.js","assets/feature-CcT1h_AV.js","assets/feature-CzdzV0ta.css","assets/feature-D2dwx9Kw.js","assets/feature-N75mQuD4.js","assets/feature-S9VUcT0E.js","assets/feature-TAIK-rSh.css","assets/feature-h-4sSlrU.css","assets/feature-pIB27gao.js","assets/feature-waj8jLyE.css","assets/feature-xHmDGcod.js","assets/feature-xM-9dNS5.css","assets/i18n-Kc9hv7_9.js","assets/index-D6brZ7xF.js","assets/index-DDp0P-Oi.css"]'));
const CACHE_PREFIX = 'db-manager-shell-';
const CACHE_NAME = CACHE_PREFIX + BUILD_ID;
const sw = /** @type {ServiceWorkerGlobalScope} */ (/** @type {unknown} */ (self));
const shellURLs = new Set(SHELL.map((file) => new URL(file, sw.registration.scope).href));
const indexURL = new URL('index.html', sw.registration.scope).href;
const rootURL = new URL('./', sw.registration.scope).href;

sw.addEventListener('install', (event) => {
  event.waitUntil(
    caches.open(CACHE_NAME).then(async (cache) => {
      // Fetch past the HTTP cache so a half-updated server cannot poison the shell cache.
      for (const url of shellURLs) {
        const response = await fetch(new Request(url, { cache: 'reload', credentials: 'omit' }));
        if (!response.ok) {
          throw new Error('Shell file unavailable: ' + url);
        }
        await cache.put(url, response);
      }
    }),
  );
});

sw.addEventListener('activate', (event) => {
  event.waitUntil(
    caches
      .keys()
      .then((names) =>
        Promise.all(
          names
            .filter((name) => name.startsWith(CACHE_PREFIX) && name !== CACHE_NAME)
            .map((name) => caches.delete(name)),
        ),
      )
      .then(() => sw.clients.claim()),
  );
});

sw.addEventListener('message', (event) => {
  if (event.data && event.data.type === 'SKIP_WAITING') {
    void sw.skipWaiting();
  }
});

sw.addEventListener('fetch', (event) => {
  const request = event.request;
  // Anything outside the narrow shell whitelist is left to the browser untouched.
  if (request.method !== 'GET' || request.headers.has('authorization')) {
    return;
  }
  const url = new URL(request.url);
  if (url.origin !== sw.location.origin) {
    return;
  }
  const clean = url.origin + url.pathname;
  if (request.mode === 'navigate') {
    if (clean === rootURL || clean === indexURL) {
      event.respondWith(navigate(request));
    }
    return;
  }
  if (shellURLs.has(clean) && url.search === '') {
    event.respondWith(shellFile(request, clean));
  }
});

/** Network first so a new release is picked up; the cached shell only answers when the network fails. */
async function navigate(request) {
  try {
    return await fetch(request);
  } catch (error) {
    const cached = await caches.match(indexURL, { cacheName: CACHE_NAME });
    if (cached) {
      return cached;
    }
    throw error;
  }
}

async function shellFile(request, clean) {
  const cached = await caches.match(clean, { cacheName: CACHE_NAME });
  return cached || fetch(request);
}
