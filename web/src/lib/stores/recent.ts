import { writable, get } from "svelte/store";
import { apiFetch } from "../api";

export interface RecentAlbum {
  album_name: string;
  album_artist: string;
  first_track_id: string;
  played_at: string;
}

export interface RecentPlaylist {
  playlist_id: string;
  name: string;
  artwork_path?: string;
  played_at: string;
}

export const recentAlbums = writable<RecentAlbum[]>([]);
export const recentPlaylists = writable<RecentPlaylist[]>([]);

/** Fetch recent items from the server. Called on auth and after recording. */
export async function loadRecent() {
  try {
    const res = await apiFetch("/api/recent");
    if (!res.ok) return;
    const data = await res.json();
    recentAlbums.set(data.albums ?? []);
    recentPlaylists.set(data.playlists ?? []);
  } catch {
    console.error("Failed to load recent items");
  }
}

/** Record an album play on the server, then refresh. */
export async function recordRecentAlbum(album: {
  album_name: string;
  album_artist: string;
  first_track_id: string;
}) {
  try {
    await apiFetch("/api/recent/albums", {
      method: "POST",
      body: JSON.stringify(album)
    });
    loadRecent();
  } catch {
    console.error("Failed to record recent album");
  }
}

/** Record a playlist play on the server, then refresh. */
export async function recordRecentPlaylist(pl: { playlist_id: string }) {
  try {
    await apiFetch("/api/recent/playlists", {
      method: "POST",
      body: JSON.stringify(pl)
    });
    loadRecent();
  } catch {
    console.error("Failed to record recent playlist");
  }
}

/** Remove a playlist from recents (e.g. on delete). */
export async function removeRecentPlaylist(playlistId: string) {
  try {
    await apiFetch(`/api/recent/playlists/${playlistId}`, {
      method: "DELETE"
    });
    recentPlaylists.update((list) =>
      list.filter((p) => p.playlist_id !== playlistId)
    );
  } catch {
    console.error("Failed to remove recent playlist");
  }
}
