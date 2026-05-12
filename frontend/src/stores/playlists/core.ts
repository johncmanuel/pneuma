import { get } from "svelte/store";
import {
  AddLocalPlaylistItem,
  CreateLocalPlaylist,
  DeleteLocalPlaylist,
  GenerateRandomPlaylist,
  GetLocalPlaylistItems,
  PickPlaylistArtwork,
  SetLocalPlaylistItems,
  UpdateLocalPlaylist,
  UploadPlaylistToServer
} from "../../../wailsjs/go/desktop/App";
import {
  addToast,
  isLocalID,
  localTrackToSharedTrack,
  type Track,
  type LocalPlaylistItem
} from "@pneuma/shared";
import {
  playlists,
  selectedPlaylistId,
  selectedPlaylistItems,
  selectedPlaylist,
  playingPlaylistId,
  setPlayingPlaylistContext,
  selectPlaylist
} from "./state";
import {
  isFavoritesPlaylist,
  visiblePlaylistsForAddMenu,
  buildLocalPlaylistItem,
  errorMessage,
  trackID,
  type PlaylistTrackInput
} from "./helpers";
import { loadPlaylists } from "./favorites";
import { playerState } from "../player";
import { fetchTracksByIDs } from "../library";
import { resolveLocalTracksByPaths } from "../localLibrary";
import { recordRecentPlaylist, removeRecentPlaylist } from "../recentAlbums";
import { wsSend } from "../ws";
import { serverFetch } from "../../utils/api";

export { isFavoritesPlaylist, visiblePlaylistsForAddMenu };

export async function createPlaylist(name: string, description = "") {
  try {
    const pl = await CreateLocalPlaylist(name, description);
    if (pl) {
      await loadPlaylists();
      addToast(`Playlist "${name}" created`, "success");
      return pl.id;
    }
  } catch (e) {
    addToast(`Failed to create playlist: ${errorMessage(e)}`, "error");
  }
  return null;
}

export async function deletePlaylist(id: string) {
  try {
    const pl = get(playlists).find((p) => p.id === id);
    if (isFavoritesPlaylist(pl)) {
      addToast("Favorites playlist cannot be deleted", "warning");
      return;
    }

    await DeleteLocalPlaylist(id);
    removeRecentPlaylist(id);

    if (pl?.remote_playlist_id) {
      serverFetch(`/api/playlists/${pl.remote_playlist_id}`, {
        method: "DELETE"
      }).catch((e) => console.warn("Failed to delete remote playlist:", e));
    }

    await loadPlaylists();
    if (get(selectedPlaylistId) === id) {
      selectedPlaylistId.set(null);
      selectedPlaylistItems.set([]);
      selectedPlaylist.set(null);
    }
    addToast("Playlist deleted", "success");
  } catch (e) {
    addToast(`Failed to delete playlist: ${errorMessage(e)}`, "error");
  }
}

export async function updatePlaylist(
  id: string,
  name: string,
  description: string,
  artworkPath = ""
) {
  try {
    const target = get(playlists).find((p) => p.id === id);
    if (isFavoritesPlaylist(target)) {
      addToast("Favorites playlist cannot be edited", "warning");
      return;
    }

    await UpdateLocalPlaylist(id, name, description, artworkPath);
    await loadPlaylists();

    if (get(selectedPlaylistId) === id) {
      await selectPlaylist(id);
    }
  } catch (e) {
    console.error("Failed to update playlist:", e);
    addToast("Failed to update playlist", "error");
  }
}

export async function addTrackToPlaylist(
  playlistId: string,
  track: PlaylistTrackInput,
  isLocal: boolean
) {
  const pl = get(playlists).find((p) => p.id === playlistId);
  const playlistName = pl?.name ?? "playlist";

  let currentItems: LocalPlaylistItem[];
  if (get(selectedPlaylistId) === playlistId) {
    currentItems = get(selectedPlaylistItems);
  } else {
    try {
      currentItems = ((await GetLocalPlaylistItems(playlistId)) ??
        []) as LocalPlaylistItem[];
    } catch {
      currentItems = [];
    }
  }

  const isDuplicate = currentItems.some((item) =>
    isLocal
      ? item.local_path && item.local_path === track.path
      : item.track_id && item.track_id === trackID(track)
  );

  if (isDuplicate) {
    const proceed = window.confirm(
      `"${track.title}" is already in "${playlistName}". Add it again?`
    );
    if (!proceed) return;
  }

  try {
    const item = buildLocalPlaylistItem(track, isLocal, 0);
    await AddLocalPlaylistItem(playlistId, item);
    addToast(`Added "${track.title}" to "${playlistName}"`, "success");

    if (playlistId === get(playingPlaylistId)) {
      const newId = isLocal ? track.path : trackID(track);
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
  } catch {
    addToast(`Failed to add "${track.title}" to "${playlistName}"`, "error");
  }
}

export async function addTracksToPlaylist(
  playlistId: string,
  tracks: Track[],
  isLocal: boolean
) {
  const pl = get(playlists).find((p) => p.id === playlistId);
  const playlistName = pl?.name ?? "playlist";

  let currentItems: LocalPlaylistItem[];
  if (get(selectedPlaylistId) === playlistId) {
    currentItems = get(selectedPlaylistItems);
  } else {
    try {
      currentItems = ((await GetLocalPlaylistItems(playlistId)) ??
        []) as LocalPlaylistItem[];
    } catch {
      currentItems = [];
    }
  }

  const newTracks = tracks.filter((track) => {
    return !currentItems.some((item) =>
      isLocal ? item.local_path === track.path : item.track_id === track.id
    );
  });

  const skipped = tracks.length - newTracks.length;
  let added = 0;

  for (const track of newTracks) {
    try {
      const item = buildLocalPlaylistItem(track, isLocal, 0);
      await AddLocalPlaylistItem(playlistId, item);
      currentItems.push(item);
      added++;
    } catch (e) {
      console.error("addTracksToPlaylist: failed to add track", track.title, e);
    }
  }

  if (get(selectedPlaylistId) === playlistId) {
    await selectPlaylist(playlistId);
  }

  await loadPlaylists();

  const toastMessage = [
    added > 0 &&
      `Added ${added} track${added !== 1 ? "s" : ""} to "${playlistName}"`,
    skipped > 0 && `${skipped} already in playlist`
  ]
    .filter(Boolean)
    .join(" · ");

  if (toastMessage) {
    addToast(toastMessage, added > 0 ? "success" : "info");
  }
}

async function reorderPlaylistItems(
  playlistId: string,
  items: LocalPlaylistItem[]
) {
  try {
    const reindexed = items.map((item, i) => ({ ...item, position: i }));
    await SetLocalPlaylistItems(playlistId, reindexed);
    selectedPlaylistItems.set(reindexed);

    if (playlistId === get(playingPlaylistId)) {
      const currentTrackId = get(playerState).trackId;
      const newQueueIds = reindexed
        .filter((item) => !item.missing)
        .map((item) =>
          item.source === "local_ref" ? item.local_path : item.track_id
        )
        .filter((id): id is string => Boolean(id));

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
  } catch (e) {
    addToast(`Failed to reorder playlist: ${errorMessage(e)}`, "error");
  }
}

export async function removePlaylistItem(playlistId: string, position: number) {
  const current = get(selectedPlaylistItems);
  const item = current.find((i) => i.position === position) ?? null;
  const playlist = get(playlists).find((pl) => pl.id === playlistId) ?? null;

  if (
    item &&
    playlist &&
    isFavoritesPlaylist(playlist) &&
    item.source === "remote"
  ) {
    const track = localTrackToSharedTrack({
      path: item.track_id || "",
      title: item.ref_title || "",
      artist: item.ref_album_artist || "",
      album: item.ref_album || "",
      album_artist: item.ref_album_artist || "",
      genre: "",
      year: 0,
      track_number: 0,
      disc_number: 0,
      duration_ms: item.ref_duration_ms
    });

    const { toggleFavoriteTrack } = await import("./favorites");
    await toggleFavoriteTrack({ ...track, id: item.track_id || track.id });
    return;
  }

  const items = current.filter((i) => i.position !== position);
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
  } catch (e) {
    addToast(`Failed to upload playlist: ${errorMessage(e)}`, "error");
    return null;
  }
}

export async function playPlaylist(
  items: LocalPlaylistItem[],
  startIndex: number,
  playlistId?: string
) {
  setPlayingPlaylistContext(playlistId ?? null);

  if (playlistId) {
    const pl = get(playlists).find((p) => p.id === playlistId);
    if (pl)
      recordRecentPlaylist({
        id: pl.id,
        name: pl.name,
        artworkPath: pl.artwork_path
      });
  }

  const queueIds = items.map((item) => {
    if (item.source === "local_ref" && item.local_path) return item.local_path;
    if (item.source === "remote" && item.track_id) return item.track_id;
    return "";
  });

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

  const startId = validIds[adjustedStart];
  let startTrack: Track | null = null;
  try {
    if (isLocalID(startId)) {
      const locals = await resolveLocalTracksByPaths([startId]);
      if (locals.length > 0) {
        const lt = locals[0];
        startTrack = localTrackToSharedTrack(lt);
      }
    } else {
      const remotes = await fetchTracksByIDs([startId]);
      if (remotes.length > 0) startTrack = remotes[0];
    }
  } catch {
    console.error("Failed to resolve starting track");
  }

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

  if (!isLocalID(startId)) {
    const queueAllRemote = validIds.every((id) => !isLocalID(id));
    if (queueAllRemote) {
      wsSend("playback.queue", {
        track_ids: validIds,
        start_index: adjustedStart
      });
    }
    wsSend("playback.play", {
      track_id: startId,
      position_ms: 0
    });
  }
}

export async function pickPlaylistArtwork(playlistId: string) {
  try {
    const target = get(playlists).find((pl) => pl.id === playlistId);
    if (isFavoritesPlaylist(target)) {
      addToast("Favorites artwork is disabled", "info");
      return;
    }

    const artFile = await PickPlaylistArtwork(playlistId);

    if (!artFile) return;

    await loadPlaylists();

    if (get(selectedPlaylistId) === playlistId) {
      await selectPlaylist(playlistId);
    }

    addToast("Playlist artwork updated", "success");
  } catch (e) {
    addToast(`Failed to set artwork: ${errorMessage(e)}`, "error");
  }
}

export async function generateRandomPlaylist(
  name: string,
  description: string,
  durationMinutes: number,
  useRemote: boolean
): Promise<string | null> {
  try {
    const pl = await GenerateRandomPlaylist(
      name,
      description,
      durationMinutes,
      useRemote
    );
    if (pl) {
      await loadPlaylists();
      addToast(
        `Playlist "${name}" generated with ${pl.item_count} songs`,
        "success"
      );
      return pl.id;
    }
  } catch (e) {
    addToast(`Failed to generate playlist: ${errorMessage(e)}`, "error");
  }
  return null;
}
