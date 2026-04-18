import { writable, get } from "svelte/store";
import { apiFetch } from "../api";
import { type AlbumGroup, type Track, isLocalID } from "@pneuma/shared";

export const loading = writable(false);
export const albumGroups = writable<AlbumGroup[]>([]);
export const albumGroupsTotal = writable(0);

const PAGE_SIZE = 50;
const TRACK_CACHE_TTL_MS = 5 * 60 * 1000;
const TRACK_CACHE_MAX_ENTRIES = 1200;

const inFlightAlbumGroupLoads = new Map<string, Promise<void>>();
const inFlightTrackLoads = new Map<string, Promise<Track[]>>();
const trackCache = new Map<
  string,
  { track: Track; expiresAt: number; lastAccessedAt: number }
>();

function uniqueRemoteTrackIDs(ids: string[]) {
  const seen = new Set<string>();

  return ids.filter((id) => {
    if (isLocalID(id) || seen.has(id)) return false;

    seen.add(id);
    return true;
  });
}

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

function cacheTracks(tracks: Track[]) {
  if (tracks.length === 0) return;

  const now = Date.now();
  const expiresAt = now + TRACK_CACHE_TTL_MS;

  tracks.forEach((track) => {
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

export async function loadAlbumGroupsPage(offset = 0, search = "") {
  const key = `${offset}:${search}`;
  const existingRequest = inFlightAlbumGroupLoads.get(key);
  if (existingRequest) {
    return existingRequest;
  }

  const request = (async () => {
    loading.set(true);
    try {
      const params = new URLSearchParams();
      params.set("offset", String(offset));
      params.set("limit", String(PAGE_SIZE));
      if (search) params.set("filter", search);

      const r = await apiFetch(`/api/library/albumgroups?${params}`);
      if (!r.ok) return;

      const data = await r.json();
      const groups: AlbumGroup[] = data.groups ?? data ?? [];
      const total: number = data.total ?? groups.length;

      if (offset === 0) {
        albumGroups.set(groups);
      } else {
        albumGroups.update((prev) => [...prev, ...groups]);
      }
      albumGroupsTotal.set(total);
    } finally {
      loading.set(false);
      inFlightAlbumGroupLoads.delete(key);
    }
  })();

  inFlightAlbumGroupLoads.set(key, request);
  return request;
}

export async function loadMoreAlbumGroups(search = "") {
  const current = get(albumGroups);
  await loadAlbumGroupsPage(current.length, search);
}

export async function fetchAlbumTracks(
  albumName: string,
  albumArtist: string
): Promise<Track[]> {
  const params = new URLSearchParams();
  params.set("album_name", albumName);
  if (albumArtist) params.set("album_artist", albumArtist);

  const r = await apiFetch(`/api/library/tracks?${params}`);
  if (!r.ok) return [];

  const data = await r.json();
  const fetched: Track[] = Array.isArray(data) ? data : (data.tracks ?? []);

  return fetched.sort(
    (a, b) =>
      (a.disc_number ?? 0) - (b.disc_number ?? 0) ||
      (a.track_number ?? 0) - (b.track_number ?? 0)
  );
}

export async function fetchTracksByIDs(ids: string[]): Promise<Track[]> {
  if (ids.length === 0) return [];

  const remoteIds = ids.filter((id) => !isLocalID(id));
  if (remoteIds.length === 0) return [];

  const uniqueRemoteIDs = uniqueRemoteTrackIDs(remoteIds);
  const now = Date.now();
  const resolvedByID = new Map<string, Track>();
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

        const r = await apiFetch(`/api/library/tracks?${params}`);
        if (!r.ok) return [];

        const data = await r.json();
        if (!data) return [];

        return (Array.isArray(data) ? data : (data.tracks ?? [])) as Track[];
      })().finally(() => {
        inFlightTrackLoads.delete(requestKey);
      });

      inFlightTrackLoads.set(requestKey, request);
    }

    const fetched = await request;
    cacheTracks(fetched);
    fetched.forEach((track) => {
      resolvedByID.set(track.id, track);
    });
  }

  return remoteIds
    .map((id) => resolvedByID.get(id))
    .filter((track): track is Track => Boolean(track));
}

export async function searchTracks(query: string): Promise<Track[]> {
  const params = new URLSearchParams();
  params.set("q", query);

  const r = await apiFetch(`/api/library/search?${params}`);
  if (!r.ok) return [];

  const data = await r.json();
  return Array.isArray(data) ? data : (data.tracks ?? []);
}

export async function searchAlbumGroups(query: string): Promise<AlbumGroup[]> {
  const params = new URLSearchParams();
  params.set("filter", query);
  params.set("limit", "10");

  const r = await apiFetch(`/api/library/albumgroups?${params}`);
  if (!r.ok) return [];

  const data = await r.json();
  return data.groups ?? [];
}
