import type { Track } from "./types";

/**
 * CachePlugin allows apps to customize caching behavior.
 */
export interface CachePlugin {
  get(id: string): Track | null;
  set(id: string, track: Track): void;
  invalidate(id: string): void;
  clear(): void;
}

/**
 * No-op cache plugin for apps that don't want caching.
 */
class NoCache implements CachePlugin {
  get(): null {
    return null;
  }
  set(): void {}
  invalidate(): void {}
  clear(): void {}
}

/**
 * Simple browser-compatible event emitter.
 */
class SimpleEventEmitter {
  private listeners = new Map<string, Set<(id: string) => void>>();

  on(event: string, handler: (id: string) => void): void {
    if (!this.listeners.has(event)) {
      this.listeners.set(event, new Set());
    }
    this.listeners.get(event)!.add(handler);
  }

  off(event: string, handler: (id: string) => void): void {
    const handlers = this.listeners.get(event);
    if (handlers) {
      handlers.delete(handler);
    }
  }

  emit(event: string, ...args: [string]): void {
    const handlers = this.listeners.get(event);
    if (handlers) {
      handlers.forEach((handler) => handler(...args));
    }
  }
}

export interface TrackResolverOpts {
  offline?: boolean;
}

/**
 * TrackResolver is the single authority for unified track loading.
 * It deduplicates in-flight requests and optionally caches results.
 * Apps inject their own cache strategy (or none).
 */
export interface TrackResolver {
  /**
   * Resolve remote tracks by ID. Deduplicates concurrent requests for the same IDs.
   * Returns partial results: (Track | null)[] with null for failed/missing tracks.
   * If offline=true, only returns cached results and queued items; never fetches.
   */
  getTracks(ids: string[], opts?: TrackResolverOpts): Promise<(Track | null)[]>;

  /**
   * Resolve local tracks by filesystem path. Uses backend-specific resolution.
   * No caching; always fresh from backend.
   */
  resolveLocalTracks(paths: string[]): Promise<Track[]>;

  /**
   * Invalidate a track in cache and emit trackInvalidated event.
   * Called by ws.ts when track.deleted arrives from server.
   */
  invalidate(id: string): void;

  /**
   * Subscribe to invalidation events.
   */
  on(event: "trackInvalidated", handler: (id: string) => void): void;
  off(event: "trackInvalidated", handler: (id: string) => void): void;
}

/**
 * ApiClient interface expected by the resolver.
 * Apps inject their own implementation.
 */
export interface TrackResolverApiClient {
  /**
   * Fetch tracks by ID.
   */
  fetchTracksByIDs(ids: string[]): Promise<(Track | null)[]>;

  /**
   * Resolve local tracks by path. Backend-specific.
   */
  resolveLocalTracksByPaths(paths: string[]): Promise<Track[]>;
}

/**
 * Factory to create a TrackResolver instance.
 */
export function createTrackResolver(
  apiClient: TrackResolverApiClient,
  cache?: CachePlugin
): TrackResolver {
  const _cache = cache ?? new NoCache();
  const _emitter = new SimpleEventEmitter();

  // map in-flight requests by ID to deduplicate.
  const _inflightRequests = new Map<string, Promise<(Track | null)[]>>();

  return {
    async getTracks(
      ids: string[],
      opts?: TrackResolverOpts
    ): Promise<(Track | null)[]> {
      if (ids.length === 0) return [];

      const { offline = false } = opts ?? {};

      const cached = ids.map((id) => _cache.get(id));
      const missingIndices = cached
        .map((t, i) => (t === null ? i : -1))
        .filter((i) => i >= 0);

      if (offline) {
        return cached;
      }

      if (missingIndices.length === 0) {
        return cached;
      }

      // Fetch missing IDs. Use in-flight dedup: if the same set of IDs
      // is already being fetched, reuse that promise.
      const missingIds = missingIndices.map((i) => ids[i]);
      const missingKey = missingIds.sort().join("|");

      let inflightPromise = _inflightRequests.get(missingKey);
      if (!inflightPromise) {
        inflightPromise = apiClient.fetchTracksByIDs(missingIds);
        _inflightRequests.set(missingKey, inflightPromise);

        // Clean up after resolution.
        inflightPromise
          .then(() => _inflightRequests.delete(missingKey))
          .catch(() => _inflightRequests.delete(missingKey));
      }

      const fetchedMissing = await inflightPromise;

      // Build result array, merging cached + fetched.
      const result: (Track | null)[] = [...cached];
      fetchedMissing.forEach((track, idx) => {
        const originalIdx = missingIndices[idx];
        result[originalIdx] = track;
        if (track) {
          _cache.set(track.id, track);
        }
      });

      return result;
    },

    async resolveLocalTracks(paths: string[]): Promise<Track[]> {
      if (paths.length === 0) return [];
      return apiClient.resolveLocalTracksByPaths(paths);
    },

    invalidate(id: string): void {
      _cache.invalidate(id);
      _emitter.emit("trackInvalidated", id);
    },

    on(event: "trackInvalidated", handler: (id: string) => void): void {
      _emitter.on(event, handler);
    },

    off(event: "trackInvalidated", handler: (id: string) => void): void {
      _emitter.off(event, handler);
    }
  };
}
