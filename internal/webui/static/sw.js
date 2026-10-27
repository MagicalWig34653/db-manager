// SPDX-License-Identifier: AGPL-3.0-only
// Service worker template. The build (pwa plugin in vite.config.ts) replaces the two placeholders
// below and emits the result as sw.js. Scope: app shell only. The worker never answers /api,
// /healthz, /readyz or any request that is not an exact GET of a precached shell file, and it never
// stores a response in the cache other than during installation.
'use strict';

const BUILD_ID = 'f163862cfe8c85c4982f4709ae4c973fac17dc9c';
const SHELL = /** @type {string[]} */ (JSON.parse('["index.html","manifest.webmanifest","icon.svg","icon-192.png","icon-512.png","icon-maskable-512.png","apple-touch-icon.png","favicon-32.png","assets/ConfirmDestructiveDialog-BiS10I6M.js","assets/circle-check-B9PRF0gR.js","assets/client-CY7sqZrP.js","assets/cm-5YgG4MGT.js","assets/feature-3Dxv5jO3.js","assets/feature-9dAReZ4r.js","assets/feature-ADYQo2EN.css","assets/feature-BPqPrbnZ.css","assets/feature-BVvus33E.css","assets/feature-BXrbnqoD.css","assets/feature-Bb6ZvVSj.css","assets/feature-BcixFSIF.css","assets/feature-BncwRnpM.js","assets/feature-BzsSHduT.css","assets/feature-C6-Mfmjt.js","assets/feature-C9tl5kyT.js","assets/feature-Cknv6iiB.js","assets/feature-D4KAXAKx.js","assets/feature-D6UELlh3.css","assets/feature-D9OaIK9T.css","assets/feature-DBU6j9gm.js","assets/feature-DFsXZ0aV.css","assets/feature-DHopDVem.js","assets/feature-DSC_fEpe.js","assets/feature-DUrwG4Gi.css","assets/feature-Dcn5vAaV.js","assets/feature-De11ib0I.js","assets/feature-DfxVBvbA.js","assets/feature-FJ9TMwhs.css","assets/feature-K2eIhZGq.js","assets/feature-TAIK-rSh.css","assets/feature-cxa5Qh7m.js","assets/feature-g0PsCG-I.js","assets/feature-h-4sSlrU.css","assets/feature-rq0g_Yda.js","assets/feature-waj8jLyE.css","assets/feature-xM-9dNS5.css","assets/format-BlwxehPh.css","assets/format-eiQl0gn9.js","assets/i18n-CfdGSK00.js","assets/index-DkS_Y5AZ.css","assets/index-vyjmrbs_.js"]'));
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
