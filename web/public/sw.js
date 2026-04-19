/* eslint-disable no-undef */

const swVersion = new URL(self.location.href).searchParams.get("v") ?? "dev";
const swKey = "pneuma-player-shell";
const shellCacheName = `${swKey}-${swVersion}`;

const rootScopePathname = new URL(self.registration.scope).pathname;
const appShellPathname = `${rootScopePathname}index.html`;
const offlineFallbackPathname = `${rootScopePathname}offline.html`;
const manifestPathname = `${rootScopePathname}site.webmanifest`;
const icon192Pathname = `${rootScopePathname}web-app-manifest-192x192.png`;
const icon512Pathname = `${rootScopePathname}web-app-manifest-512x512.png`;
const faviconPathname = `${rootScopePathname}favicon.ico`;
const svgFaviconPathname = `${rootScopePathname}favicon.svg`;

const precachePaths = [
  rootScopePathname,
  appShellPathname,
  offlineFallbackPathname,
  manifestPathname,
  icon192Pathname,
  icon512Pathname,
  faviconPathname,
  svgFaviconPathname
];

const cacheableAssetExtensions = [
  ".js",
  ".css",
  ".png",
  ".svg",
  ".ico",
  ".webmanifest",
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

function resolveScopePathname(pathLike) {
  return new URL(pathLike, self.registration.scope).pathname;
}

function isPrecacheCandidate(pathname) {
  return (
    pathname.startsWith(rootScopePathname) &&
    (pathname === rootScopePathname ||
      pathname === appShellPathname ||
      pathname === offlineFallbackPathname ||
      pathname === manifestPathname ||
      hasCacheableAssetExtension(pathname))
  );
}

function extractPrecacheCandidatesFromHTML(html) {
  const matches = html.matchAll(/(?:href|src)=["']([^"']+)["']/g);
  const candidates = new Set();

  for (const match of matches) {
    const raw = match[1];
    if (!raw || raw.startsWith("data:")) continue;

    try {
      const pathname = resolveScopePathname(raw);
      if (isPrecacheCandidate(pathname)) {
        candidates.add(pathname);
      }
    } catch {
      continue;
    }
  }

  return [...candidates];
}

async function precacheShellLinkedAssets(cache) {
  try {
    const response = await fetch(rootScopePathname, { cache: "no-store" });
    if (!response.ok) return;

    const html = await response.text();
    const linkedPaths = extractPrecacheCandidatesFromHTML(html);

    await Promise.all(
      linkedPaths.map(async (path) => {
        try {
          await cache.add(path);
        } catch {
          console.warn("[pwa] failed to precache linked asset", path);
        }
      })
    );
  } catch {
    console.warn("[pwa] failed to precache shell linked assets");
  }
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
  event.waitUntil(
    (async () => {
      const cache = await caches.open(shellCacheName);

      await Promise.all(
        precachePaths.map(async (path) => {
          try {
            await cache.add(path);
          } catch {
            console.warn("[pwa] failed to precache", path);
          }
        })
      );

      await precacheShellLinkedAssets(cache);
    })()
  );
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

          const appShell = await caches.match(rootScopePathname);
          if (appShell) return appShell;

          const appShellHTML = await caches.match(appShellPathname);
          if (appShellHTML) return appShellHTML;

          const fallback = await caches.match(offlineFallbackPathname);
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

      try {
        const response = await fetch(request);
        if (response.ok) {
          const cache = await caches.open(shellCacheName);
          cache.put(request, response.clone()).catch(() => {
            console.warn("[pwa] failed to cache asset response", request.url);
          });
        }

        return response;
      } catch {
        const manifestFallback = await caches.match(manifestPathname);
        if (url.pathname === manifestPathname && manifestFallback) {
          return manifestFallback;
        }

        if (request.destination === "style") {
          return new Response("", {
            status: 200,
            headers: {
              "Content-Type": "text/css; charset=utf-8"
            }
          });
        }

        return new Response("", {
          status: 503,
          statusText: "Offline"
        });
      }
    })()
  );
});

self.addEventListener("message", (event) => {
  if (event.data?.type === "SKIP_WAITING") {
    self.skipWaiting();
  }
});
