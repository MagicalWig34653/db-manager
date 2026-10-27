// SPDX-License-Identifier: AGPL-3.0-only
// Service worker template. The build (pwa plugin in vite.config.ts) replaces the two placeholders
// below and emits the result as sw.js. Scope: app shell only. The worker never answers /api,
// /healthz, /readyz or any request that is not an exact GET of a precached shell file, and it never
// stores a response in the cache other than during installation.
'use strict';

const BUILD_ID = 'e38e9b547e6c569f30ff3751a8dcd412b98aef5e';
const SHELL = /** @type {string[]} */ (JSON.parse('["index.html","manifest.webmanifest","icon.svg","icon-192.png","icon-512.png","icon-maskable-512.png","apple-touch-icon.png","favicon-32.png","assets/ConfirmDestructiveDialog-xHQVdG8T.js","assets/circle-check-Dfbb3BGP.js","assets/client-D5T0UylA.js","assets/feature-ADYQo2EN.css","assets/feature-B8v2xUGD.js","assets/feature-BPqPrbnZ.css","assets/feature-BVvus33E.css","assets/feature-BXrbnqoD.css","assets/feature-Bb6ZvVSj.css","assets/feature-BcixFSIF.css","assets/feature-BiD1B_5Q.js","assets/feature-BmVo0Iuz.js","assets/feature-BnCsQdcT.js","assets/feature-Bqqhz6OV.js","assets/feature-BsKKOydX.js","assets/feature-BzsSHduT.css","assets/feature-C8u_i5Ku.js","assets/feature-CcBFAzkX.js","assets/feature-Cf7YsQML.js","assets/feature-ClJcg_Xq.js","assets/feature-CwtWOtcC.js","assets/feature-D4ucDQNz.js","assets/feature-D6UELlh3.css","assets/feature-D9OaIK9T.css","assets/feature-DFsXZ0aV.css","assets/feature-DUrwG4Gi.css","assets/feature-TAIK-rSh.css","assets/feature-X2RTqbh_.js","assets/feature-fUfNtdAT.js","assets/feature-h-4sSlrU.css","assets/feature-waj8jLyE.css","assets/feature-xM-9dNS5.css","assets/feature-zIQiKwnj.js","assets/feature-zakpxM4s.js","assets/format-BlwxehPh.css","assets/format-Bpck049n.js","assets/i18n-DdSghXWq.js","assets/index-BgHzpxJQ.js","assets/index-DWa3HOql.css"]'));
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
