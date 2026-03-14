import { writable } from "svelte/store";
import type { Track } from "./player";
import { serverFetch, connected } from "../utils/api";
import { get } from "svelte/store";

export interface Album {
  id: string;
  title: string;
  artist_id: string;
  year: number;
  artwork_id: string;
  artist_name?: string;
}

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
export const albums = writable<Album[]>([]);
export const loading = writable(false);
export const searchResults = writable<Track[]>([]);
export const albumSearchResults = writable<RemoteAlbumGroup[]>([]);

export const remoteAlbumGroups = writable<RemoteAlbumGroup[]>([]);
export const remoteAlbumGroupsTotal = writable(0);
export const remoteAlbumGroupsOffset = writable(0);

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
  } catch {}
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
  } catch {}
}

export const tracksTotal = writable(0);
export const tracksOffset = writable(0);
export const albumsTotal = writable(0);
export const albumsOffset = writable(0);

const PAGE_SIZE = 50;

/** Fetch the next page of albums and append. */
export async function loadMoreAlbums(filter = "") {
  if (!get(connected)) return;

  const currentOffset = get(albumsOffset);
  const total = get(albumsTotal);
  const nextOffset = currentOffset + PAGE_SIZE;

  if (nextOffset >= total) return;

  const params = new URLSearchParams({
    offset: String(nextOffset),
    limit: String(PAGE_SIZE)
  });

  if (filter) params.set("filter", filter);

  const r = await serverFetch(`/api/library/albums?${params}`);
  const data = await r.json();
  albums.update((existing) => [...existing, ...(data.albums ?? [])]);
  albumsOffset.set(nextOffset);
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

export function clearSearch() {
  searchResults.set([]);
  albumSearchResults.set([]);
}

export async function triggerScan() {
  if (!get(connected)) return;
  await serverFetch("/api/library/scan", { method: "POST" });
}
