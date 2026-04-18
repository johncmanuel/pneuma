import { get, writable } from "svelte/store";
import { apiFetch } from "../api";

interface RecentAlbum {
  album_name: string;
  album_artist: string;
  first_track_id: string;
  played_at: string;
}

interface RecentPlaylist {
  playlist_id: string;
  name: string;
  artwork_path?: string;
  played_at: string;
}

export const recentAlbums = writable<RecentAlbum[]>([]);
export const recentPlaylists = writable<RecentPlaylist[]>([]);

const MAX_RECENT_ITEMS = 50;

function upsertRecentAlbum(album: {
  album_name: string;
  album_artist: string;
  first_track_id: string;
}) {
  const playedAt = new Date().toISOString();

  recentAlbums.update((list) => {
    const next = [
      {
        album_name: album.album_name,
        album_artist: album.album_artist,
        first_track_id: album.first_track_id,
        played_at: playedAt
      },
      ...list.filter(
        (entry) =>
          entry.album_name !== album.album_name ||
          entry.album_artist !== album.album_artist
      )
    ];

    return next.slice(0, MAX_RECENT_ITEMS);
  });
}

function upsertRecentPlaylist(playlist: {
  playlist_id: string;
  name?: string;
  artwork_path?: string;
}) {
  const playedAt = new Date().toISOString();

  recentPlaylists.update((list) => {
    const existing = list.find(
      (entry) => entry.playlist_id === playlist.playlist_id
    );

    const next = [
      {
        playlist_id: playlist.playlist_id,
        name: playlist.name?.trim() || existing?.name || "Playlist",
        artwork_path:
          playlist.artwork_path?.trim() || existing?.artwork_path || "",
        played_at: playedAt
      },
      ...list.filter((entry) => entry.playlist_id !== playlist.playlist_id)
    ];

    return next.slice(0, MAX_RECENT_ITEMS);
  });
}

/** Fetch recent items from the server. */
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

/** Record an album play on the server with optimistic local update. */
export async function recordRecentAlbum(album: {
  album_name: string;
  album_artist: string;
  first_track_id: string;
}) {
  const previous = get(recentAlbums);
  upsertRecentAlbum(album);

  try {
    const res = await apiFetch("/api/recent/albums", {
      method: "POST",
      body: JSON.stringify(album)
    });

    if (!res.ok) {
      recentAlbums.set(previous);
      void loadRecent();
    }
  } catch {
    recentAlbums.set(previous);
    void loadRecent();
    console.error("Failed to record recent album");
  }
}

/** Record a playlist play on the server with optimistic local update. */
export async function recordRecentPlaylist(pl: {
  playlist_id: string;
  name?: string;
  artwork_path?: string;
}) {
  const previous = get(recentPlaylists);
  upsertRecentPlaylist(pl);

  try {
    const res = await apiFetch("/api/recent/playlists", {
      method: "POST",
      body: JSON.stringify({ playlist_id: pl.playlist_id })
    });

    if (!res.ok) {
      recentPlaylists.set(previous);
      void loadRecent();
    }
  } catch {
    recentPlaylists.set(previous);
    void loadRecent();
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
