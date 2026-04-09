import { writable, get } from "svelte/store";
import { apiFetch } from "../api";
import type { PlaylistSummary, PlaylistItem, Track } from "@pneuma/shared";
import {
  dedupeFavoriteTrackItems,
  favoriteTrackIDsFromItems,
  findFavoritesPlaylist,
  favoritesPlaylistMarker,
  favoritesPlaylistName,
  isFavoritesPlaylistMeta,
  isFavoritesPlaylist as isFavoritesPlaylistShared,
  localTrackToSharedTrack,
  pickCanonicalFavoritesPlaylist,
  toFavoritesWriteItem,
  toFavoritesWriteItemFromTrack,
  visiblePlaylistsForAddMenu as visiblePlaylistsForAddMenuShared
} from "@pneuma/shared";
import { addToast } from "@pneuma/shared";

export const playlists = writable<PlaylistSummary[]>([]);
export const selectedPlaylist = writable<PlaylistSummary | null>(null);
export const selectedPlaylistItems = writable<PlaylistItem[]>([]);
export const playlistsLoading = writable(false);

export const favoriteTrackIDs = writable<Set<string>>(new Set());

export const favoritesPlaylistId = writable<string | null>(null);

async function fetchRemotePlaylists(): Promise<PlaylistSummary[] | null> {
  const res = await apiFetch("/api/playlists");
  if (!res.ok) return null;

  const data = await res.json();
  return (
    Array.isArray(data) ? data : (data.playlists ?? [])
  ) as PlaylistSummary[];
}

async function fetchPlaylistItems(playlistId: string): Promise<PlaylistItem[]> {
  const res = await apiFetch(`/api/playlists/${playlistId}/items`);
  if (!res.ok) return [];

  const data = await res.json();
  return (Array.isArray(data) ? data : (data.items ?? [])) as PlaylistItem[];
}

async function normalizeFavoritesOnServer(
  candidates: PlaylistSummary[]
): Promise<string | null> {
  if (candidates.length === 0) {
    const created = await apiFetch("/api/playlists", {
      method: "POST",
      body: JSON.stringify({
        name: favoritesPlaylistName,
        description: favoritesPlaylistMarker
      })
    });

    if (!created.ok) return null;
    const data = await created.json();
    return (data?.id as string | undefined) ?? null;
  }

  const canonical = pickCanonicalFavoritesPlaylist(candidates);

  if (candidates.length > 1) {
    const merged: PlaylistItem[] = await Promise.all(
      candidates.map((playlist) => fetchPlaylistItems(playlist.id))
    ).then((items) => items.flat());

    const deduped = dedupeFavoriteTrackItems(merged);
    await apiFetch(`/api/playlists/${canonical.id}/items`, {
      method: "PUT",
      body: JSON.stringify(deduped)
    });

    for (const playlist of candidates) {
      if (playlist.id === canonical.id) continue;
      await apiFetch(`/api/playlists/${playlist.id}`, { method: "DELETE" });
    }
  }

  // ensure the canonical favorites playlist has the right name and marker
  if (
    canonical.name !== favoritesPlaylistName ||
    (canonical.description ?? "") !== favoritesPlaylistMarker
  ) {
    await apiFetch(`/api/playlists/${canonical.id}`, {
      method: "PUT",
      body: JSON.stringify({
        name: favoritesPlaylistName,
        description: favoritesPlaylistMarker
      })
    });
  }

  return canonical.id;
}

function refreshFavoritesCache(nextPlaylists: PlaylistSummary[]) {
  const favorites = findFavoritesPlaylist(nextPlaylists);
  favoritesPlaylistId.set(favorites?.id ?? null);
}

async function refreshFavoriteTrackIDsFromPlaylist(playlistId: string | null) {
  if (!playlistId) {
    favoriteTrackIDs.set(new Set());
    return;
  }

  const res = await apiFetch(`/api/playlists/${playlistId}/items`);
  if (!res.ok) {
    favoriteTrackIDs.set(new Set());
    return;
  }

  const data = await res.json();
  const items = (
    Array.isArray(data) ? data : (data.items ?? [])
  ) as PlaylistItem[];
  favoriteTrackIDs.set(
    favoriteTrackIDsFromItems(
      items.map((item) => ({
        source: item.source,
        track_id: item.track_id,
        local_path: ""
      }))
    )
  );
}

async function refreshFavoritesState(nextPlaylists: PlaylistSummary[]) {
  refreshFavoritesCache(nextPlaylists);
  const favorites = findFavoritesPlaylist(nextPlaylists);
  await refreshFavoriteTrackIDsFromPlaylist(favorites?.id ?? null);
}

export function isTrackFavorited(trackId: string): boolean {
  return get(favoriteTrackIDs).has(trackId);
}

export function isFavoritesPlaylist(
  playlist: PlaylistSummary | null | undefined
): boolean {
  return isFavoritesPlaylistShared(playlist);
}

export function visiblePlaylistsForAddMenu(
  list: PlaylistSummary[]
): PlaylistSummary[] {
  return visiblePlaylistsForAddMenuShared(list);
}

export async function ensureFavoritesPlaylist(): Promise<string | null> {
  const remotePlaylists = await fetchRemotePlaylists();
  if (!remotePlaylists) return get(favoritesPlaylistId);

  const candidates = remotePlaylists.filter((pl) =>
    isFavoritesPlaylistMeta(pl.name, pl.description)
  );

  const favoritesID = await normalizeFavoritesOnServer(candidates);
  if (!favoritesID) return null;

  await loadPlaylists();
  favoritesPlaylistId.set(favoritesID);
  return favoritesID;
}

export async function toggleFavoriteTrack(track: Track | null) {
  if (!track?.id) return;

  const favoritesID = await ensureFavoritesPlaylist();
  if (!favoritesID) {
    addToast("Failed to open Favorites playlist", "error");
    return;
  }

  const res = await apiFetch(`/api/playlists/${favoritesID}/items`);
  if (!res.ok) {
    addToast("Failed to update Favorites", "error");
    return;
  }

  const data = await res.json();
  const existingItems = (
    Array.isArray(data) ? data : (data.items ?? [])
  ) as PlaylistItem[];
  const alreadyFavorite = existingItems.some(
    (item) => item.track_id === track.id
  );

  const nextItems = alreadyFavorite
    ? existingItems
        .filter((item) => item.track_id !== track.id)
        .map(toFavoritesWriteItem)
    : [
        ...existingItems.map(toFavoritesWriteItem),
        toFavoritesWriteItemFromTrack(track)
      ];

  const write = await apiFetch(`/api/playlists/${favoritesID}/items`, {
    method: "PUT",
    body: JSON.stringify(nextItems)
  });
  if (!write.ok) {
    addToast("Failed to update Favorites", "error");
    return;
  }

  favoriteTrackIDs.update((prev) => {
    const next = new Set(prev);
    if (alreadyFavorite) next.delete(track.id);
    else next.add(track.id);
    return next;
  });

  if (get(selectedPlaylist)?.id === favoritesID) {
    await selectPlaylist(favoritesID);
  }

  await loadPlaylists();
  addToast(
    alreadyFavorite
      ? `Removed "${track.title}" from Favorites`
      : `Added "${track.title}" to Favorites`,
    "success"
  );
}

export async function loadPlaylists() {
  const r = await apiFetch("/api/playlists");
  if (!r.ok) return;

  const data = await r.json();
  const next = (
    Array.isArray(data) ? data : (data.playlists ?? [])
  ) as PlaylistSummary[];
  playlists.set(next);
  await refreshFavoritesState(next);
}

export async function selectPlaylist(id: string) {
  playlistsLoading.set(true);
  try {
    const [plRes, itemsRes] = await Promise.all([
      apiFetch(`/api/playlists/${id}`),
      apiFetch(`/api/playlists/${id}/items`)
    ]);

    if (plRes.ok) {
      selectedPlaylist.set(await plRes.json());
    }
    if (itemsRes.ok) {
      const data = await itemsRes.json();
      selectedPlaylistItems.set(
        Array.isArray(data) ? data : (data.items ?? [])
      );
    }

    if (id === get(favoritesPlaylistId)) {
      await refreshFavoriteTrackIDsFromPlaylist(id);
    }
  } finally {
    playlistsLoading.set(false);
  }
}

export async function createPlaylist(
  name: string,
  description: string
): Promise<string | null> {
  const r = await apiFetch("/api/playlists", {
    method: "POST",
    body: JSON.stringify({ name, description })
  });

  if (!r.ok) return null;

  const data = await r.json();
  await loadPlaylists();
  return data.id ?? null;
}

export async function deletePlaylist(id: string) {
  const target = get(playlists).find((pl) => pl.id === id);
  if (isFavoritesPlaylist(target)) {
    addToast("Favorites playlist cannot be deleted", "warning");
    return;
  }

  await apiFetch(`/api/playlists/${id}`, { method: "DELETE" });
  await loadPlaylists();

  // Clear selection if we deleted the selected playlist
  if (get(selectedPlaylist)?.id === id) {
    selectedPlaylist.set(null);
    selectedPlaylistItems.set([]);
  }
}

export async function updatePlaylist(
  id: string,
  name: string,
  description: string
) {
  const target = get(playlists).find((pl) => pl.id === id);
  if (isFavoritesPlaylist(target)) {
    addToast("Favorites playlist cannot be edited", "warning");
    return;
  }

  await apiFetch(`/api/playlists/${id}`, {
    method: "PUT",
    body: JSON.stringify({ name, description })
  });
  await loadPlaylists();
  // Refresh selected if it's the same playlist
  if (get(selectedPlaylist)?.id === id) {
    const r = await apiFetch(`/api/playlists/${id}`);
    if (r.ok) selectedPlaylist.set(await r.json());
  }
}

export async function addTracksToPlaylist(playlistId: string, tracks: Track[]) {
  const existing =
    get(selectedPlaylist)?.id === playlistId
      ? get(selectedPlaylistItems)
      : await (async () => {
          const r = await apiFetch(`/api/playlists/${playlistId}/items`);
          if (!r.ok) return [] as PlaylistItem[];

          const data = await r.json();
          return Array.isArray(data) ? data : (data.items ?? []);
        })();

  const existingItems = existing.map((item: PlaylistItem) => ({
    source: item.source || "remote",
    track_id: item.track_id,
    ref_title: item.ref_title,
    ref_album: item.ref_album,
    ref_album_artist: item.ref_album_artist,
    ref_duration_ms: item.ref_duration_ms
  }));

  const newItems = tracks.map((t) => ({
    source: "remote",
    track_id: t.id,
    ref_title: t.title,
    ref_album: t.album_name,
    ref_album_artist: t.album_artist,
    ref_duration_ms: t.duration_ms
  }));

  const items = [...existingItems, ...newItems];

  await apiFetch(`/api/playlists/${playlistId}/items`, {
    method: "PUT",
    body: JSON.stringify(items)
  });

  // Refresh if this is the selected playlist
  if (get(selectedPlaylist)?.id === playlistId) {
    await selectPlaylist(playlistId);
  }
}

export async function handleAddToPlaylist(
  track: Track | null,
  playlistId: string
) {
  if (!track) return;
  await addTracksToPlaylist(playlistId, [track]);
}

export async function removePlaylistItem(playlistId: string, position: number) {
  const current = get(selectedPlaylistItems);
  const remaining = current
    .filter((item) => item.position !== position)
    .map((item) => ({
      track_id: item.track_id,
      ref_title: item.ref_title,
      ref_album: item.ref_album,
      ref_album_artist: item.ref_album_artist,
      ref_duration_ms: item.ref_duration_ms
    }));

  await apiFetch(`/api/playlists/${playlistId}/items`, {
    method: "PUT",
    body: JSON.stringify(remaining)
  });

  await selectPlaylist(playlistId);
}

export function itemToTrack(item: PlaylistItem): Track {
  const track = localTrackToSharedTrack({
    path: "",
    title: item.ref_title || "Unknown",
    artist: item.ref_album_artist || "",
    album: item.ref_album || "",
    album_artist: item.ref_album_artist || "",
    genre: "",
    year: 0,
    track_number: item.position + 1,
    disc_number: 0,
    duration_ms: item.ref_duration_ms
  });

  return {
    ...track,
    id: item.track_id || `missing-${item.position}`
  };
}
