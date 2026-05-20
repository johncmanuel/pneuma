import {
  createTrackResolver,
  TTLCachePlugin,
  type TrackResolver,
  type TrackResolverApiClient
} from "@pneuma/shared";
import { apiFetch } from "./api";
import { type Track } from "@pneuma/shared";

/**
 * Web implementation of TrackResolverApiClient.
 * Fetches remote tracks via fetch API.
 */
class WebTrackResolverApiClient implements TrackResolverApiClient {
  async fetchTracksByIDs(ids: string[]): Promise<(Track | null)[]> {
    if (ids.length === 0) return [];

    try {
      const params = new URLSearchParams();
      params.set("ids", ids.join(","));

      const r = await apiFetch(`/api/library/tracks?${params}`);
      if (!r.ok) {
        return ids.map(() => null);
      }

      const data = await r.json();
      const fetched = (Array.isArray(data) ? data : []) as Track[];

      // Map fetched tracks by ID to maintain order.
      const fetchedByID = new Map(fetched.map((t) => [t.id, t]));
      return ids.map((id) => fetchedByID.get(id) ?? null);
    } catch {
      return ids.map(() => null);
    }
  }

  // web app can't support local tracks on client's disk; only desktop can do this.
  async resolveLocalTracksByPaths(): Promise<Track[]> {
    return [];
  }
}

let _resolver: TrackResolver | null = null;

/**
 * Get or create the track resolver for the web app.
 * Use TTL-based caching.
 */
export function getTrackResolver(): TrackResolver {
  if (!_resolver) {
    const apiClient = new WebTrackResolverApiClient();
    const cachePlugin = new TTLCachePlugin();
    _resolver = createTrackResolver(apiClient, cachePlugin);
  }
  return _resolver;
}
