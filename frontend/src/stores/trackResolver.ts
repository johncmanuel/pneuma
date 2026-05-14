import {
  createTrackResolver,
  TTLCachePlugin,
  type TrackResolver,
  type TrackResolverApiClient
} from "@pneuma/shared";
import { GetLocalTracksByPaths } from "../../wailsjs/go/desktop/App";
import { serverFetch, connected } from "../utils/api";
import { get } from "svelte/store";
import { localTrackToSharedTrack, type Track } from "@pneuma/shared";

/**
 * Desktop implementation of TrackResolverApiClient.
 * Fetches remote tracks via serverFetch and resolves local tracks via Wails backend.
 */
class DesktopTrackResolverApiClient implements TrackResolverApiClient {
  async fetchTracksByIDs(ids: string[]): Promise<(Track | null)[]> {
    if (!get(connected) || ids.length === 0) return [];

    const params = new URLSearchParams();
    params.set("ids", ids.join(","));

    try {
      const r = await serverFetch(`/api/library/tracks?${params}`);
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

  async resolveLocalTracksByPaths(paths: string[]): Promise<Track[]> {
    if (paths.length === 0) return [];

    try {
      const localTracks = await GetLocalTracksByPaths(paths);
      if (!localTracks) return [];
      return localTracks.map(localTrackToSharedTrack);
    } catch {
      return [];
    }
  }
}

let _resolver: TrackResolver | null = null;

/**
 * Get or create the track resolver for the desktop app.
 * Desktop uses a conservative TTL cache to reduce repeated fetches.
 */
export function getTrackResolver(): TrackResolver {
  if (!_resolver) {
    const apiClient = new DesktopTrackResolverApiClient();
    const cachePlugin = new TTLCachePlugin(3 * 60 * 1000, 800);
    _resolver = createTrackResolver(apiClient, cachePlugin);
  }
  return _resolver;
}
