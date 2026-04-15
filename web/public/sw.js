/* global URL, self, caches, fetch, console */

const swVersion = "v1";
const swKey = "pneuma-player-shell";
const shellCacheName = `${swKey}-${swVersion}`;

const cacheableAssetExtensions = [
  ".js",
  ".css",
  ".png",
  ".svg",
  ".ico",
  ".webp",
  ".woff2",
  ".woff",
  ".ttf"
];

function isHttpRequest(request) {
  return (
    request.url.startsWith("http://") || request.url.startsWith("https://")
  );
}

function isSameOrigin(request) {
  return new URL(request.url).origin === self.location.origin;
}

function hasCacheableAssetExtension(pathname) {
  return cacheableAssetExtensions.some((ext) => pathname.endsWith(ext));
}

function scopePathname() {
  return new URL(self.registration.scope).pathname;
}

async function clearOldCaches() {
  const keys = await caches.keys();

  await Promise.all(
    keys
      .filter((key) => key.startsWith(`${swKey}-`) && key !== shellCacheName)
      .map((key) => caches.delete(key))
  );
}

self.addEventListener("install", (event) => {
  event.waitUntil(self.skipWaiting());
});

self.addEventListener("activate", (event) => {
  event.waitUntil(
    (async () => {
      await clearOldCaches();
      await self.clients.claim();
    })()
  );
});

self.addEventListener("fetch", (event) => {
  const { request } = event;

  if (request.method !== "GET") return;
  if (!isHttpRequest(request)) return;
  if (!isSameOrigin(request)) return;

  const url = new URL(request.url);

  if (
    url.pathname.startsWith("/api/") ||
    url.pathname === "/ws" ||
    request.headers.has("range")
  ) {
    return;
  }

  const acceptsHtml = request.headers.get("accept")?.includes("text/html");

  if (acceptsHtml) {
    event.respondWith(
      (async () => {
        try {
          const network = await fetch(request);
          const cache = await caches.open(shellCacheName);
          cache.put(request, network.clone()).catch(() => {
            console.warn("[pwa] failed to cache html response", request.url);
          });
          return network;
        } catch {
          const cached = await caches.match(request);
          if (cached) return cached;

          const fallback = await caches.match(scopePathname());
          if (fallback) return fallback;

          throw new Error("offline and no cached html available");
        }
      })()
    );
    return;
  }

  if (!hasCacheableAssetExtension(url.pathname)) return;

  event.respondWith(
    (async () => {
      const cached = await caches.match(request);
      if (cached) return cached;

      const response = await fetch(request);
      if (response.ok) {
        const cache = await caches.open(shellCacheName);
        cache.put(request, response.clone()).catch(() => {
          console.warn("[pwa] failed to cache asset response", request.url);
        });
      }

      return response;
    })()
  );
});
