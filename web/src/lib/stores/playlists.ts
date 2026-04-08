import { writable, get } from "svelte/store";
import { apiFetch } from "../api";
import type { PlaylistSummary, PlaylistItem, Track } from "@pneuma/shared";
import { localTrackToSharedTrack } from "@pneuma/shared";

export const playlists = writable<PlaylistSummary[]>([]);
export const selectedPlaylist = writable<PlaylistSummary | null>(null);
export const selectedPlaylistItems = writable<PlaylistItem[]>([]);
export const playlistsLoading = writable(false);

export async function loadPlaylists() {
  const r = await apiFetch("/api/playlists");
  if (!r.ok) return;

  const data = await r.json();
  playlists.set(Array.isArray(data) ? data : (data.playlists ?? []));
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
