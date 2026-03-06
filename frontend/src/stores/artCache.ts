/**
 * In-memory artwork blob-URL cache with in-flight deduplication.
 *
 * - `cachedArtUrl(key, rawUrl)` — returns a blob URL on success, or the
 *   original rawUrl as a fallback so <img> never breaks.
 * - Concurrent calls for the same key share a single in-flight fetch.
 * - Evicts oldest entries when the cache exceeds MAX_ENTRIES.
 */

const MAX_ENTRIES = 100 

/** Resolved blob URLs, keyed by cache key. */
const blobCache = new Map<string, string>()

/** In-flight fetches, keyed by cache key. */
const inflight = new Map<string, Promise<string>>()

/** LRU order (oldest first). */
const lruOrder: string[] = []

function evict() {
  while (lruOrder.length > MAX_ENTRIES) {
    const oldest = lruOrder.shift()!
    const blobUrl = blobCache.get(oldest)
    if (blobUrl) URL.revokeObjectURL(blobUrl)
    blobCache.delete(oldest)
  }
}

function touch(key: string) {
  const idx = lruOrder.indexOf(key)
  if (idx !== -1) lruOrder.splice(idx, 1)
  lruOrder.push(key)
}

/**
 * Returns a blob URL for the given artwork.
 * Falls back to `rawUrl` if the network request fails or rawUrl is empty.
 */
export async function cachedArtUrl(key: string, rawUrl: string): Promise<string> {
  if (!key || !rawUrl) return rawUrl

  // Cache hit
  if (blobCache.has(key)) {
    touch(key)
    return blobCache.get(key)!
  }

  // Deduplicate concurrent requests for the same key
  if (inflight.has(key)) return inflight.get(key)!

  const p = fetch(rawUrl)
    .then(r => {
      if (!r.ok) throw new Error(`HTTP ${r.status}`)
      return r.blob()
    })
    .then(blob => {
      const url = URL.createObjectURL(blob)
      blobCache.set(key, url)
      touch(key)
      evict()
      inflight.delete(key)
      return url
    })
    .catch(() => {
      inflight.delete(key)
      return rawUrl // graceful fallback
    })

  inflight.set(key, p)
  return p
}

/** Returns the cached blob URL synchronously, or null if not yet loaded. */
export function getCachedArtUrlSync(key: string): string | null {
  return blobCache.get(key) ?? null
}
