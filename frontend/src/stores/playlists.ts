import { writable, get } from "svelte/store";
import {
  CreateLocalPlaylist,
  GetLocalPlaylists,
  GetLocalPlaylistItems,
  DeleteLocalPlaylist,
  UpdateLocalPlaylist,
  SetLocalPlaylistItems,
  AddLocalPlaylistItem,
  ResolvePlaylistItems,
  UploadPlaylistToServer,
  PickPlaylistArtwork
} from "../../wailsjs/go/desktop/App";
import { addToast } from "./toasts";
import { playerState, type Track } from "./player";
import { fetchTracksByIDs } from "./library";
import { resolveLocalTracksByPaths, isLocalId } from "./localLibrary";
import { recordRecentPlaylist } from "./recentAlbums";
import { wsSend } from "./ws";

export interface PlaylistSummary {
  id: string;
  name: string;
  description: string;
  artwork_path: string;
  remote_playlist_id: string;
  item_count: number;
  total_duration_ms: number;
  created_at: string;
  updated_at: string;
}

export interface PlaylistItem {
  position: number;
  source: "remote" | "local_ref";
  track_id: string;
  local_path: string;
  ref_title: string;
  ref_album: string;
  ref_album_artist: string;
  ref_duration_ms: number;
  added_at: string;
  resolved: boolean;
  missing: boolean;
}

export const playlists = writable<PlaylistSummary[]>([]);

export const selectedPlaylistId = writable<string | null>(null);

export const selectedPlaylistItems = writable<PlaylistItem[]>([]);

export const selectedPlaylist = writable<PlaylistSummary | null>(null);

export const playlistsLoading = writable(false);

export const playingPlaylistId = writable<string | null>(null);

/** Load all local playlists from the desktop DB. */
export async function loadPlaylists() {
  try {
    const list = await GetLocalPlaylists();
    playlists.set(list ?? []);
  } catch (e: any) {
    console.error("loadPlaylists:", e);
  }
}

export async function createPlaylist(name: string, description = "") {
  try {
    const pl = await CreateLocalPlaylist(name, description);
    if (pl) {
      await loadPlaylists();
      addToast(`Playlist "${name}" created`, "success");
      return pl.id;
    }
  } catch (e: any) {
    addToast(`Failed to create playlist: ${e}`, "error");
  }
  return null;
}

export async function deletePlaylist(id: string) {
  try {
    await DeleteLocalPlaylist(id);
    await loadPlaylists();
    // If we're viewing the deleted playlist, clear selection.
    if (get(selectedPlaylistId) === id) {
      selectedPlaylistId.set(null);
      selectedPlaylistItems.set([]);
      selectedPlaylist.set(null);
    }
    addToast("Playlist deleted", "success");
  } catch (e: any) {
    addToast(`Failed to delete playlist: ${e}`, "error");
  }
}

/** Update playlist metadata. */
export async function updatePlaylist(
  id: string,
  name: string,
  description: string,
  artworkPath = ""
) {
  try {
    await UpdateLocalPlaylist(id, name, description, artworkPath);
    await loadPlaylists();
    // Refresh selected if it's the one we updated.
    if (get(selectedPlaylistId) === id) {
      await selectPlaylist(id);
    }
  } catch (e: any) {
    addToast(`Failed to update playlist: ${e}`, "error");
  }
}

export async function selectPlaylist(id: string) {
  selectedPlaylistId.set(id);
  playlistsLoading.set(true);

  try {
    const list = get(playlists);
    const summary = list.find((p) => p.id === id) ?? null;
    selectedPlaylist.set(summary);

    const items = (await ResolvePlaylistItems(id)) as PlaylistItem[] | null;
    selectedPlaylistItems.set(items ?? []);
  } catch (e: any) {
    console.error("selectPlaylist:", e);
  } finally {
    playlistsLoading.set(false);
  }
}

/** Add a track (local or remote) to a playlist. */
export async function addTrackToPlaylist(
  playlistId: string,
  track:
    | Track
    | {
        path: string;
        title: string;
        album: string;
        album_artist: string;
        duration_ms: number;
      },
  isLocal: boolean
) {
  const pl = get(playlists).find((p) => p.id === playlistId);
  const playlistName = pl?.name ?? "playlist";

  // use cached items if this is the selected playlist, else fetch them.
  let currentItems: PlaylistItem[];
  if (get(selectedPlaylistId) === playlistId) {
    currentItems = get(selectedPlaylistItems);
  } else {
    try {
      currentItems = ((await GetLocalPlaylistItems(playlistId)) ??
        []) as PlaylistItem[];
    } catch {
      currentItems = [];
    }
  }
  const isDuplicate = currentItems.some((item) =>
    isLocal
      ? item.local_path && item.local_path === (track as any).path
      : item.track_id && item.track_id === (track as Track).id
  );

  if (isDuplicate) {
    const proceed = window.confirm(
      `"${track.title}" is already in "${playlistName}". Add it again?`
    );
    if (!proceed) return;
  }

  try {
    const item: any = {
      position: 0,
      source: isLocal ? "local_ref" : "remote",
      track_id: isLocal ? "" : (track as Track).id,
      local_path: isLocal ? (track as any).path : "",
      ref_title: track.title || "",
      ref_album: (track as any).album || (track as any).album_name || "",
      ref_album_artist: (track as any).album_artist || "",
      ref_duration_ms: track.duration_ms || 0,
      added_at: "",
      resolved: false,
      missing: false
    };
    await AddLocalPlaylistItem(playlistId, item);
    addToast(`Added \"${track.title}\" to \"${playlistName}\"`, "success");

    // Sync queue if this playlist is currently playing.
    if (playlistId === get(playingPlaylistId)) {
      const newId = isLocal ? (track as any).path : (track as Track).id;
      if (newId) {
        playerState.update((s) => ({
          ...s,
          queue: [...s.queue, newId],
          baseQueue: [...s.baseQueue, newId]
        }));
      }
    }

    if (get(selectedPlaylistId) === playlistId) {
      await selectPlaylist(playlistId);
    }

    await loadPlaylists();
  } catch (_) {
    addToast(
      `Failed to add \"${track.title}\" to \"${playlistName}\"`,
      "error"
    );
  }
}

/**
 * Add multiple tracks to a playlist in one batch.
 * Duplicates are silently skipped and reported in a single toast.
 */
export async function addTracksToPlaylist(
  playlistId: string,
  tracks: Track[],
  isLocal: boolean
) {
  const pl = get(playlists).find((p) => p.id === playlistId);
  const playlistName = pl?.name ?? "playlist";

  // Fetch current items once for duplicate detection.
  let currentItems: PlaylistItem[];
  if (get(selectedPlaylistId) === playlistId) {
    currentItems = get(selectedPlaylistItems);
  } else {
    try {
      currentItems = ((await GetLocalPlaylistItems(playlistId)) ??
        []) as PlaylistItem[];
    } catch {
      currentItems = [];
    }
  }

  let added = 0;
  let skipped = 0;
  for (const track of tracks) {
    const isDuplicate = currentItems.some((item) =>
      isLocal
        ? item.local_path && item.local_path === (track as any).path
        : item.track_id && item.track_id === track.id
    );

    if (isDuplicate) {
      skipped++;
      continue;
    }

    try {
      const item: any = {
        position: 0,
        source: isLocal ? "local_ref" : "remote",
        track_id: isLocal ? "" : track.id,
        local_path: isLocal ? (track as any).path : "",
        ref_title: track.title || "",
        ref_album: track.album_name || "",
        ref_album_artist: track.album_artist || "",
        ref_duration_ms: track.duration_ms || 0,
        added_at: "",
        resolved: false,
        missing: false
      };
      await AddLocalPlaylistItem(playlistId, item);

      // Optimistically add to local list to catch same-batch duplicates.
      currentItems = [...currentItems, item];
      added++;
    } catch (e: any) {
      console.error("addTracksToPlaylist: failed to add track", track.title, e);
    }
  }

  if (get(selectedPlaylistId) === playlistId) {
    await selectPlaylist(playlistId);
  }
  await loadPlaylists();

  const parts: string[] = [];
  if (added > 0)
    parts.push(
      `Added ${added} track${added !== 1 ? "s" : ""} to "${playlistName}"`
    );
  if (skipped > 0) parts.push(`${skipped} already in playlist`);
  addToast(parts.join(" · "), added > 0 ? "success" : "info");
}

export async function reorderPlaylistItems(
  playlistId: string,
  items: PlaylistItem[]
) {
  try {
    const reindexed = items.map((item, i) => ({ ...item, position: i }));
    await SetLocalPlaylistItems(playlistId, reindexed);
    selectedPlaylistItems.set(reindexed);

    // Sync queue if this playlist is currently playing.
    if (playlistId === get(playingPlaylistId)) {
      const currentTrackId = get(playerState).trackId;
      const newQueueIds = reindexed
        .filter((item) => !item.missing)
        .map((item) =>
          item.source === "local_ref" ? item.local_path : item.track_id
        )
        .filter((id): id is string => !!id);
      const foundIdx = newQueueIds.indexOf(currentTrackId);
      playerState.update((s) => ({
        ...s,
        queue: newQueueIds,
        baseQueue: newQueueIds,
        queueIndex:
          foundIdx >= 0
            ? foundIdx
            : Math.min(s.queueIndex, Math.max(0, newQueueIds.length - 1))
      }));
    }

    await loadPlaylists();
  } catch (e: any) {
    addToast(`Failed to reorder playlist: ${e}`, "error");
  }
}

export async function removePlaylistItem(playlistId: string, position: number) {
  const items = get(selectedPlaylistItems).filter(
    (i) => i.position !== position
  );
  await reorderPlaylistItems(playlistId, items);
}

export async function uploadPlaylist(playlistId: string) {
  try {
    const remoteId = await UploadPlaylistToServer(playlistId);
    addToast("Playlist uploaded to server", "success");
    await loadPlaylists();
    if (get(selectedPlaylistId) === playlistId) {
      await selectPlaylist(playlistId);
    }
    return remoteId;
  } catch (e: any) {
    addToast(`Failed to upload playlist: ${e}`, "error");
    return null;
  }
}

/** Play a playlist from a given index (builds queue from playlist items). */
export async function playPlaylist(
  items: PlaylistItem[],
  startIndex: number,
  playlistId?: string
) {
  playingPlaylistId.set(playlistId ?? null);

  if (playlistId) {
    const pl = get(playlists).find((p) => p.id === playlistId);
    if (pl)
      recordRecentPlaylist({
        id: pl.id,
        name: pl.name,
        artworkPath: pl.artwork_path
      });
  }

  // Build queue IDs from resolved items.
  const queueIds: string[] = [];
  for (const item of items) {
    if (item.source === "local_ref" && item.local_path) {
      queueIds.push(item.local_path);
    } else if (item.source === "remote" && item.track_id) {
      queueIds.push(item.track_id);
    } else {
      // Missing/unresolved, so use a placeholder that will be skipped.
      queueIds.push("");
    }
  }

  // Filter out empty entries and adjust startIndex.
  const validIds: string[] = [];
  let adjustedStart = 0;
  for (let i = 0; i < queueIds.length; i++) {
    if (queueIds[i]) {
      if (i === startIndex) adjustedStart = validIds.length;
      validIds.push(queueIds[i]);
    }
  }

  if (validIds.length === 0) {
    addToast("No playable tracks in this playlist", "warning");
    return;
  }

  // Resolve starting track metadata so now-playing displays immediately.
  const startId = validIds[adjustedStart];
  let startTrack: Track | null = null;
  try {
    if (isLocalId(startId)) {
      const locals = await resolveLocalTracksByPaths([startId]);
      if (locals.length > 0) {
        const lt = locals[0];
        startTrack = {
          id: lt.path,
          path: lt.path,
          title: lt.title,
          artist_id: "",
          album_id: "",
          artist_name: lt.artist,
          album_artist: lt.album_artist,
          album_name: lt.album,
          genre: lt.genre,
          year: lt.year,
          track_number: lt.track_number,
          disc_number: lt.disc_number,
          duration_ms: lt.duration_ms,
          bitrate_kbps: 0,
          replay_gain_track: 0,
          artwork_id: ""
        } as Track;
      }
    } else {
      const remotes = await fetchTracksByIDs([startId]);
      if (remotes.length > 0) startTrack = remotes[0];
    }
  } catch {}

  playerState.update((s) => ({
    ...s,
    queue: [...validIds],
    baseQueue: [...validIds],
    queueIndex: adjustedStart,
    trackId: startId,
    track: startTrack,
    paused: false,
    positionMs: 0
  }));

  // Notify the server about the new track and queue so the server session
  // stays in sync with the client. Without this, the server retains stale
  // state from the previous play, and a subsequent seek causes the server
  // to echo back the wrong track_id which switches playback to a different
  // song. Only send for remote tracks (local paths are unknown to the server).
  if (!isLocalId(startId)) {
    const queueAllRemote = validIds.every((id) => !isLocalId(id));
    if (queueAllRemote) {
      wsSend("playback.queue", {
        device_id: "desktop",
        track_ids: validIds,
        start_index: adjustedStart
      });
    }
    wsSend("playback.play", {
      device_id: "desktop",
      track_id: startId,
      position_ms: 0
    });
  }
}

/** Pick and set custom artwork for a playlist (opens file dialog). */
export async function pickPlaylistArtwork(playlistId: string) {
  try {
    const artFile = await PickPlaylistArtwork(playlistId);

    if (!artFile) return; // user cancelled

    await loadPlaylists();

    if (get(selectedPlaylistId) === playlistId) {
      await selectPlaylist(playlistId);
    }

    addToast("Playlist artwork updated", "success");
  } catch (e: any) {
    addToast(`Failed to set artwork: ${e}`, "error");
  }
}

export async function initPlaylists() {
  await loadPlaylists();
}
