import { writable, get } from "svelte/store";
import { apiFetch } from "../api";
import { type AlbumGroup, type Track, isLocalID } from "@pneuma/shared";

export const loading = writable(false);
export const albumGroups = writable<AlbumGroup[]>([]);
export const albumGroupsTotal = writable(0);
const PAGE_SIZE = 50;

const inFlightAlbumGroupLoads = new Map<string, Promise<void>>();

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

  const params = new URLSearchParams();
  params.set("ids", remoteIds.join(","));

  const r = await apiFetch(`/api/library/tracks?${params}`);
  if (!r.ok) return [];

  const data = await r.json();
  if (!data) return [];

  return Array.isArray(data) ? data : (data.tracks ?? []);
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
