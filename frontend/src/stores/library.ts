import { writable } from "svelte/store";
import { type Track, isLocalID } from "@pneuma/shared";
import { serverFetch, connected } from "../utils/api";
import { get } from "svelte/store";

/** Album group derived from the tracks table and is reliable regardless of albums table state. */
export interface RemoteAlbumGroup {
  key: string; // "name|||artist" or "__unorganized__"
  name: string;
  artist: string;
  track_count: number;
  first_track_id: string;
}

// Unique key for albums without the appropriate metadata
export const UNORGANIZED_KEY = "__unorganized__";

export const tracks = writable<Track[]>([]);
export const loading = writable(false);
const searchResults = writable<Track[]>([]);
const albumSearchResults = writable<RemoteAlbumGroup[]>([]);

export const remoteAlbumGroups = writable<RemoteAlbumGroup[]>([]);
export const remoteAlbumGroupsTotal = writable(0);
const remoteAlbumGroupsOffset = writable(0);

const ALBUM_GROUP_PAGE_SIZE = 50;
const TRACK_CACHE_TTL_MS = 5 * 60 * 1000;
const TRACK_CACHE_MAX_ENTRIES = 1200;

const inFlightTrackLoads = new Map<string, Promise<Track[]>>();
const trackCache = new Map<
  string,
  { track: Track; expiresAt: number; lastAccessedAt: number }
>();

function getCachedTrack(id: string, now = Date.now()) {
  const entry = trackCache.get(id);
  if (!entry) return null;

  if (entry.expiresAt <= now) {
    trackCache.delete(id);
    return null;
  }

  entry.lastAccessedAt = now;
  return entry.track;
}

function pruneTrackCache(now = Date.now()) {
  for (const [id, entry] of trackCache) {
    if (entry.expiresAt <= now) {
      trackCache.delete(id);
    }
  }

  if (trackCache.size <= TRACK_CACHE_MAX_ENTRIES) return;

  const overflow = trackCache.size - TRACK_CACHE_MAX_ENTRIES;
  const lru = [...trackCache.entries()]
    .sort((a, b) => a[1].lastAccessedAt - b[1].lastAccessedAt)
    .slice(0, overflow);

  lru.forEach(([id]) => {
    trackCache.delete(id);
  });
}

function cacheTracks(tracksToCache: Track[]) {
  if (tracksToCache.length === 0) return;

  const now = Date.now();
  const expiresAt = now + TRACK_CACHE_TTL_MS;

  tracksToCache.forEach((track) => {
    if (!track.id || isLocalID(track.id)) return;

    trackCache.set(track.id, {
      track,
      expiresAt,
      lastAccessedAt: now
    });
  });

  pruneTrackCache(now);
}

export function invalidateCachedTrack(trackID: string) {
  trackCache.delete(trackID);
}

export async function loadRemoteAlbumGroupsPage(offset = 0, filter = "") {
  if (!get(connected)) return;

  const params = new URLSearchParams({
    offset: String(offset),
    limit: String(ALBUM_GROUP_PAGE_SIZE)
  });

  if (filter) params.set("filter", filter);

  try {
    const r = await serverFetch(`/api/library/albumgroups?${params}`);
    if (!r.ok) return;
    const data = await r.json();
    remoteAlbumGroups.set(data.groups ?? []);
    remoteAlbumGroupsTotal.set(data.total ?? 0);
    remoteAlbumGroupsOffset.set(data.offset ?? 0);
  } catch (e) {
    console.warn("Failed to load remote album groups:", e);
  }
}

export async function loadMoreRemoteAlbumGroups(filter = "") {
  if (!get(connected)) return;

  const currentOffset = get(remoteAlbumGroupsOffset);
  const total = get(remoteAlbumGroupsTotal);
  const nextOffset = currentOffset + ALBUM_GROUP_PAGE_SIZE;

  if (nextOffset >= total) return;

  const params = new URLSearchParams({
    offset: String(nextOffset),
    limit: String(ALBUM_GROUP_PAGE_SIZE)
  });

  if (filter) params.set("filter", filter);

  try {
    const r = await serverFetch(`/api/library/albumgroups?${params}`);
    if (!r.ok) return;
    const data = await r.json();
    remoteAlbumGroups.update((existing) => [
      ...existing,
      ...(data.groups ?? [])
    ]);
    remoteAlbumGroupsOffset.set(nextOffset);
  } catch (e) {
    console.warn("Failed to load more remote album groups:", e);
  }
}

/** Fetch tracks by IDs (for queue resolution). */
export async function fetchTracksByIDs(ids: string[]): Promise<Track[]> {
  if (!get(connected) || ids.length === 0) return [];

  const remoteIDs = ids.filter((id) => !isLocalID(id));
  if (remoteIDs.length === 0) return [];

  const now = Date.now();
  const resolvedByID = new Map<string, Track>();
  const uniqueRemoteIDs = [...new Set(remoteIDs)];
  const missingIDs: string[] = [];

  uniqueRemoteIDs.forEach((id) => {
    const cached = getCachedTrack(id, now);
    if (cached) {
      resolvedByID.set(id, cached);
      return;
    }

    missingIDs.push(id);
  });

  if (missingIDs.length > 0) {
    const requestKey = missingIDs.slice().sort().join(",");
    let request = inFlightTrackLoads.get(requestKey);

    if (!request) {
      request = (async () => {
        const params = new URLSearchParams();
        params.set("ids", missingIDs.join(","));

        const r = await serverFetch(`/api/library/tracks?${params}`);
        if (!r.ok) return [];

        const data = await r.json();
        const fetched = (Array.isArray(data) ? data : []) as Track[];
        cacheTracks(fetched);
        return fetched;
      })().finally(() => {
        inFlightTrackLoads.delete(requestKey);
      });

      inFlightTrackLoads.set(requestKey, request);
    }

    const fetched = await request;
    fetched.forEach((track) => {
      resolvedByID.set(track.id, track);
    });
  }

  return remoteIDs
    .map((id) => resolvedByID.get(id))
    .filter((track): track is Track => Boolean(track));
}

export async function searchTracks(q: string): Promise<Track[]> {
  if (!get(connected)) return [];
  try {
    const r = await serverFetch(
      `/api/library/search?q=${encodeURIComponent(q)}`
    );
    if (!r.ok) {
      searchResults.set([]);
      return [];
    }
    const results = await r.json();
    const arr = Array.isArray(results) ? results : [];
    searchResults.set(arr);
    return arr;
  } catch {
    searchResults.set([]);
    return [];
  }
}

export async function searchAlbumGroups(
  q: string
): Promise<RemoteAlbumGroup[]> {
  if (!get(connected)) return [];

  const limit = 10;

  try {
    const r = await serverFetch(
      `/api/library/albumgroups?filter=${encodeURIComponent(q)}&limit=${limit}`
    );
    if (!r.ok) {
      albumSearchResults.set([]);
      return [];
    }
    const data = await r.json();
    const arr: RemoteAlbumGroup[] = data.groups ?? [];
    albumSearchResults.set(arr);
    return arr;
  } catch {
    albumSearchResults.set([]);
    return [];
  }
}
