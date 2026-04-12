import { writable } from "svelte/store";
import type { Track } from "@pneuma/shared";
import { serverFetch, connected } from "../utils/api";
import { get } from "svelte/store";

/** Album group derived from the tracks table and is reliable regardless of albums table state. */
export interface RemoteAlbumGroup {
  key: string; // "name|||artist" or "__unorganized__"
  name: string;
  artist: string;
  track_count: number;
  first_track_id: string;
  artwork_id: string;
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
  const r = await serverFetch(`/api/library/tracks?ids=${ids.join(",")}`);
  const data = await r.json();
  return Array.isArray(data) ? data : [];
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
