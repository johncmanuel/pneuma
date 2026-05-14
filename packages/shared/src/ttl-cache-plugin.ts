import type { Track, CachePlugin } from "./track-resolver";

/**
 * TTL-based cache plugin with LRU eviction.
 */
export class TTLCachePlugin implements CachePlugin {
  private cache = new Map<
    string,
    { track: Track; expiresAt: number; lastAccessedAt: number }
  >();
  private ttlMs: number;
  private maxEntries: number;

  constructor(ttlMs = 5 * 60 * 1000, maxEntries = 1200) {
    this.ttlMs = ttlMs;
    this.maxEntries = maxEntries;
  }

  get(id: string): Track | null {
    const entry = this.cache.get(id);
    if (!entry) return null;

    const now = Date.now();
    if (entry.expiresAt <= now) {
      this.cache.delete(id);
      return null;
    }

    entry.lastAccessedAt = now;
    return entry.track;
  }

  set(id: string, track: Track): void {
    const now = Date.now();
    this.cache.set(id, {
      track,
      expiresAt: now + this.ttlMs,
      lastAccessedAt: now
    });

    this.prune(now);
  }

  invalidate(id: string): void {
    this.cache.delete(id);
  }

  clear(): void {
    this.cache.clear();
  }

  private prune(now = Date.now()): void {
    for (const [id, entry] of this.cache) {
      if (entry.expiresAt <= now) {
        this.cache.delete(id);
      }
    }

    // evict LRU if max entries exceeded
    if (this.cache.size <= this.maxEntries) return;

    const overflow = this.cache.size - this.maxEntries;
    const lru = [...this.cache.entries()]
      .sort((a, b) => a[1].lastAccessedAt - b[1].lastAccessedAt)
      .slice(0, overflow);

    lru.forEach(([id]) => {
      this.cache.delete(id);
    });
  }
}
