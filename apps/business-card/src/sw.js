// Service worker: once visited, the card, its vCard and its QR code keep
// working without network (a basement, a trade-show hall, a plane).
// scripts/build.mjs fills in the version and the file list; any change to a
// file changes the version, so the browser installs the new worker.

const VERSION = '__VERSION__';
const CACHE = `card-${VERSION}`;
/** @type {string[]} */
const PRECACHE = /* PRECACHE */ [];
const scope = new URL(self.registration.scope);

self.addEventListener('install', (event) => {
  event.waitUntil(
    caches
      .open(CACHE)
      .then((cache) => cache.addAll(PRECACHE.map((p) => new URL(p, scope).href)))
      .then(() => self.skipWaiting()),
  );
});

self.addEventListener('activate', (event) => {
  event.waitUntil(
    caches
      .keys()
      .then((keys) =>
        Promise.all(
          keys.filter((k) => k.startsWith('card-') && k !== CACHE).map((k) => caches.delete(k)),
        ),
      )
      .then(() => self.clients.claim()),
  );
});

self.addEventListener('fetch', (event) => {
  const { request } = event;
  const url = new URL(request.url);
  if (request.method !== 'GET' || url.origin !== scope.origin) return;

  if (request.mode === 'navigate') {
    // Network first: an edited card is seen at once when online.
    event.respondWith(
      fetch(request)
        .then((response) => {
          if (response.ok && url.pathname === scope.pathname) {
            const copy = response.clone();
            void caches.open(CACHE).then((c) => c.put(scope.href, copy));
          }
          return response;
        })
        .catch(async () => {
          const cache = await caches.open(CACHE);
          const isCard =
            url.pathname === scope.pathname || url.pathname === `${scope.pathname}index.html`;
          return (
            // The vCard is opened by navigation on iOS: serve it offline too.
            (await cache.match(isCard ? scope.href : request, {
              ignoreSearch: true,
            })) ||
            (await cache.match(new URL('404.html', scope).href)) ||
            Response.error()
          );
        }),
    );
    return;
  }

  // Versioned assets: cache first, ignoring the ?v= query the pages add.
  event.respondWith(
    caches.open(CACHE).then(async (cache) => {
      const hit = await cache.match(request, { ignoreSearch: true });
      return hit ?? fetch(request);
    }),
  );
});
