import { writable, get } from "svelte/store";
import {
  CreateLocalPlaylist,
  AddLocalPlaylistItem,
  DeleteLocalPlaylist,
  GenerateRandomPlaylist,
  GetLocalPlaylistItems,
  GetLocalPlaylists,
  PickPlaylistArtwork,
  ResolvePlaylistItems,
  SetLocalPlaylistItems,
  UpdateLocalPlaylist,
  UploadPlaylistToServer
} from "../../wailsjs/go/desktop/App";
import {
  addToast,
  dedupeFavoriteTrackItems,
  favoriteItemKey,
  favoriteTrackIDsFromItems,
  findFavoritesPlaylist,
  favoritesPlaylistName,
  hasSameFavoriteKeyOrder,
  isFavoritesPlaylistMeta,
  isFavoritesPlaylist as isFavoritesPlaylistShared,
  isLocalID,
  localTrackToSharedTrack,
  mergeRemoteAndLocalFavoriteItems,
  pickCanonicalFavoritesPlaylist,
  storageKeys,
  toFavoritesWriteItem,
  toFavoritesWriteItemFromTrack,
  visiblePlaylistsForAddMenu as visiblePlaylistsForAddMenuShared,
  type LocalPlaylistItem,
  type LocalPlaylistSummary,
  type Track
} from "@pneuma/shared";
import { playerState } from "./player";
import { fetchTracksByIDs } from "./library";
import { resolveLocalTracksByPaths } from "./localLibrary";
import { recordRecentPlaylist, removeRecentPlaylist } from "./recentAlbums";
import { wsSend } from "./ws";
import { connected, serverFetch } from "../utils/api";

export type { LocalPlaylistSummary as PlaylistSummary } from "@pneuma/shared";
export type { LocalPlaylistItem as PlaylistItem } from "@pneuma/shared";
export type { PlaylistMenuItem } from "@pneuma/shared";

export const playlists = writable<LocalPlaylistSummary[]>([]);

export const selectedPlaylistId = writable<string | null>(null);

export const selectedPlaylistItems = writable<LocalPlaylistItem[]>([]);

export const selectedPlaylist = writable<LocalPlaylistSummary | null>(null);

export const playlistsLoading = writable(false);

export const playingPlaylistId = writable<string | null>(null);

export const favoriteTrackIDs = writable<Set<string>>(new Set());

export const favoritesPlaylistId = writable<string | null>(null);

export const favoritesRemotePlaylistId = writable<string | null>(null);

const defaultFavoritesSyncEnabled = false;

function loadFavoritesSyncPreference() {
  const raw = localStorage.getItem(storageKeys.favoritesSyncEnabled);
  if (raw == null) return defaultFavoritesSyncEnabled;
  return raw === "1";
}

export const favoritesSyncEnabled = writable<boolean>(
  loadFavoritesSyncPreference()
);

favoritesSyncEnabled.subscribe((enabled) => {
  localStorage.setItem(storageKeys.favoritesSyncEnabled, enabled ? "1" : "0");
});

interface RemotePlaylistSummary {
  id: string;
  name: string;
  description: string;
  item_count: number;
  updated_at: string;
}

interface RemoteFavoriteItem {
  source?: string;
  track_id: string;
  ref_title: string;
  ref_album: string;
  ref_album_artist: string;
  ref_duration_ms: number;
  added_at?: string;
}

type PlaylistTrackInput =
  | Track
  | {
      path: string;
      title: string;
      album: string;
      album_artist: string;
      duration_ms: number;
    };

let favoritesSyncPromise: Promise<void> | null = null;

// pick the canonical favorites playlist from the given candidates
// if there is a preferred remote playlist id, pick the one with that remote playlist id
// otherwise, pick the one with the most items
// if there is a tie, pick the one with the most recent update time
function pickCanonicalFavoritesLocal(
  candidates: LocalPlaylistSummary[],
  preferredRemoteID: string | null
) {
  const withPreferredRemote = preferredRemoteID
    ? candidates.filter((c) => c.remote_playlist_id === preferredRemoteID)
    : [];

  const linked = candidates.filter((c) => Boolean(c.remote_playlist_id));
  const pool =
    withPreferredRemote.length > 0
      ? withPreferredRemote
      : linked.length > 0
        ? linked
        : candidates;

  return pickCanonicalFavoritesPlaylist(pool);
}

function trackAlbum(track: PlaylistTrackInput) {
  return "album_name" in track ? track.album_name : track.album;
}

function trackID(track: PlaylistTrackInput): string {
  return "id" in track ? track.id : "";
}

function buildLocalPlaylistItem(
  track: PlaylistTrackInput,
  isLocal: boolean,
  position: number
): LocalPlaylistItem {
  return {
    position,
    source: isLocal ? "local_ref" : "remote",
    track_id: isLocal ? "" : trackID(track),
    local_path: isLocal ? track.path : "",
    ref_title: track.title || "",
    ref_album: trackAlbum(track) || "",
    ref_album_artist: track.album_artist || "",
    ref_duration_ms: track.duration_ms || 0,
    added_at: "",
    resolved: false,
    missing: false
  };
}

// if there are multiple playlists that are favorites, merge them into one
// and delete the others
async function normalizeLocalFavorites(
  list: LocalPlaylistSummary[],
  preferredRemoteID: string | null
): Promise<{ normalized: LocalPlaylistSummary[]; canonicalID: string | null }> {
  const candidates = list.filter((pl) =>
    isFavoritesPlaylistMeta(pl.name, pl.description)
  );

  if (candidates.length === 0) {
    return { normalized: list, canonicalID: null };
  }

  const canonical = pickCanonicalFavoritesLocal(candidates, preferredRemoteID);

  if (candidates.length === 1) {
    return { normalized: list, canonicalID: canonical.id };
  }

  const items = (
    await Promise.all(
      candidates.map(async (pl) => {
        const items = (await GetLocalPlaylistItems(pl.id)) ?? [];
        return items;
      })
    )
  ).flat();

  const seen = new Set<string>();

  const dedupedItems = items
    .filter((item) => {
      const key = favoriteItemKey(item);
      if (key.endsWith(":") || seen.has(key)) return false;
      seen.add(key);
      return true;
    })
    .map((item, index) => ({
      ...item,
      position: index
    }));

  await SetLocalPlaylistItems(canonical.id, dedupedItems);

  await Promise.all(
    candidates
      .filter((pl) => pl.id !== canonical.id)
      .map((pl) => DeleteLocalPlaylist(pl.id))
  );

  const refreshed = ((await GetLocalPlaylists()) ??
    []) as LocalPlaylistSummary[];
  return { normalized: refreshed, canonicalID: canonical.id };
}

// rebuild the set of favorite track ids from the local favorites playlist
async function refreshFavoriteTrackIDsFromLocal(
  localPlaylistID: string | null
) {
  if (!localPlaylistID) {
    favoriteTrackIDs.set(new Set());
    return;
  }

  try {
    const items = ((await GetLocalPlaylistItems(localPlaylistID)) ?? []) as
      | LocalPlaylistItem[]
      | null;
    favoriteTrackIDs.set(favoriteTrackIDsFromItems(items ?? [], true));
  } catch {
    favoriteTrackIDs.set(new Set());
  }
}

// load playlists from the database and set the initial state
async function hydrateLocalPlaylistsState() {
  const initial = ((await GetLocalPlaylists()) ?? []) as LocalPlaylistSummary[];
  const preferredRemoteID = get(favoritesRemotePlaylistId);
  const { normalized } = await normalizeLocalFavorites(
    initial,
    preferredRemoteID
  );
  playlists.set(normalized);

  const favorites = findFavoritesPlaylist(normalized);
  favoritesPlaylistId.set(favorites?.id ?? null);
  favoritesRemotePlaylistId.set(favorites?.remote_playlist_id || null);

  await refreshFavoriteTrackIDsFromLocal(favorites?.id ?? null);

  return normalized;
}

// ensures that there is a remote favorites playlist.
// if there is, it returns the id of the canonical favorites playlist
// if there is not, it creates one and returns the id of the new favorites playlist
// if it fails to create a favorites playlist, it returns null
async function ensureRemoteFavoritesPlaylist(): Promise<string | null> {
  if (!get(connected)) return null;

  const listRes = await serverFetch("/api/playlists");
  if (!listRes.ok) return null;

  const listData = await listRes.json();
  const remotePlaylists = (
    Array.isArray(listData) ? listData : (listData.playlists ?? [])
  ) as RemotePlaylistSummary[];
  const candidates = remotePlaylists.filter((pl) =>
    isFavoritesPlaylistMeta(pl.name, pl.description)
  );

  // if there are multiple favorites playlists for some reason, merge them
  if (candidates.length > 0) {
    const canonical = pickCanonicalFavoritesPlaylist(candidates);

    if (candidates.length > 1) {
      const merged = await Promise.all(
        candidates.map((pl) => fetchRemoteFavoritesItems(pl.id))
      ).then((items) => items.flat());

      const mergedItems = merged.map((item, index) => ({
        source: item.source || "remote",
        track_id: item.track_id,
        position: index,
        added_at: item.added_at ?? "",
        ref_title: item.ref_title,
        ref_album: item.ref_album,
        ref_album_artist: item.ref_album_artist,
        ref_duration_ms: item.ref_duration_ms,
        missing: false
      }));
      const deduped = dedupeFavoriteTrackItems(mergedItems);

      await serverFetch(`/api/playlists/${canonical.id}/items`, {
        method: "PUT",
        body: JSON.stringify(deduped)
      });

      for (const pl of candidates) {
        if (pl.id === canonical.id) continue;
        await serverFetch(`/api/playlists/${pl.id}`, { method: "DELETE" });
      }
    }

    // if the canonical favorites playlist is not named
    // "Favorites" or has a description, rename it
    if (
      canonical.name !== favoritesPlaylistName ||
      (canonical.description ?? "") !== ""
    ) {
      await serverFetch(`/api/playlists/${canonical.id}`, {
        method: "PUT",
        body: JSON.stringify({
          name: favoritesPlaylistName,
          description: ""
        })
      });
    }

    favoritesRemotePlaylistId.set(canonical.id);
    return canonical.id;
  }

  // else, create a new favorites playlist and set the remote favorites playlist id
  const createRes = await serverFetch("/api/playlists", {
    method: "POST",
    body: JSON.stringify({
      name: favoritesPlaylistName,
      description: ""
    })
  });
  if (!createRes.ok) return null;

  const created = await createRes.json();
  const createdID = (created?.id as string | undefined) ?? null;

  favoritesRemotePlaylistId.set(createdID);
  return createdID;
}

async function fetchRemoteFavoritesItems(
  remotePlaylistID: string
): Promise<RemoteFavoriteItem[]> {
  const res = await serverFetch(`/api/playlists/${remotePlaylistID}/items`);
  if (!res.ok) return [];

  const data = await res.json();
  const items = (
    Array.isArray(data) ? data : (data.items ?? [])
  ) as RemoteFavoriteItem[];
  return items.filter((item) => Boolean(item.track_id));
}

function remoteToLocalFavoriteItems(
  remoteItems: RemoteFavoriteItem[]
): LocalPlaylistItem[] {
  return remoteItems.map((item, index) => ({
    position: index,
    source: "remote",
    track_id: item.track_id,
    local_path: "",
    ref_title: item.ref_title,
    ref_album: item.ref_album,
    ref_album_artist: item.ref_album_artist,
    ref_duration_ms: item.ref_duration_ms,
    added_at: item.added_at ?? "",
    resolved: true,
    missing: false
  }));
}

export function isFavoritesPlaylist(
  playlist: LocalPlaylistSummary | null | undefined
) {
  return isFavoritesPlaylistShared(playlist);
}

export function isTrackFavorited(trackID: string): boolean {
  return get(favoriteTrackIDs).has(trackID);
}

export function visiblePlaylistsForAddMenu(
  list: LocalPlaylistSummary[]
): LocalPlaylistSummary[] {
  return visiblePlaylistsForAddMenuShared(list);
}

export async function setFavoritesSyncEnabled(enabled: boolean) {
  const wasEnabled = get(favoritesSyncEnabled);
  favoritesSyncEnabled.set(enabled);

  if (!wasEnabled && enabled && get(connected)) {
    await syncFavoritesFromServer();
    await loadPlaylists();
  }
}

function shouldSyncFavoritesToServer() {
  return get(favoritesSyncEnabled) && get(connected);
}

// ensures that there is a favorites playlist, either remote or local
// if there is, it returns the id of the canonical favorites playlist
// if there is not, it creates one and returns the id of the new favorites playlist
// if it fails to create a favorites playlist, it returns null
export async function ensureFavoritesPlaylist(): Promise<string | null> {
  const remoteID = shouldSyncFavoritesToServer()
    ? await ensureRemoteFavoritesPlaylist()
    : null;

  if (remoteID) {
    favoritesRemotePlaylistId.set(remoteID);
  }

  const hydrated = await hydrateLocalPlaylistsState();
  const existing = findFavoritesPlaylist(hydrated);

  if (existing?.id) {
    return existing.id;
  }

  const created = await CreateLocalPlaylist(favoritesPlaylistName, "").catch(
    () => null
  );
  if (!created) return null;

  await hydrateLocalPlaylistsState();

  if (remoteID) {
    await syncFavoritesFromServer();
  }

  return get(favoritesPlaylistId);
}

export async function syncFavoritesFromServer() {
  if (!shouldSyncFavoritesToServer()) return;

  if (favoritesSyncPromise) {
    await favoritesSyncPromise;
    return;
  }

  // run the sync in the background
  favoritesSyncPromise = (async () => {
    const localID = await ensureFavoritesPlaylist();
    if (!localID) return;

    const remoteID = get(favoritesRemotePlaylistId);
    if (!remoteID) return;

    const remoteItems = await fetchRemoteFavoritesItems(remoteID);
    const currentItems = ((await GetLocalPlaylistItems(localID)) ??
      []) as LocalPlaylistItem[];

    const mergedRemoteAndLocal = mergeRemoteAndLocalFavoriteItems(
      remoteToLocalFavoriteItems(remoteItems),
      currentItems
    );

    if (!hasSameFavoriteKeyOrder(currentItems, mergedRemoteAndLocal)) {
      await SetLocalPlaylistItems(localID, mergedRemoteAndLocal);
    }

    favoritesRemotePlaylistId.set(remoteID);
    await refreshFavoriteTrackIDsFromLocal(localID);

    if (get(selectedPlaylistId) === localID) {
      const items = (await ResolvePlaylistItems(localID)) as
        | LocalPlaylistItem[]
        | null;
      selectedPlaylistItems.set(items ?? []);
    }
  })();

  try {
    await favoritesSyncPromise;
  } finally {
    favoritesSyncPromise = null;
  }
}

export async function toggleFavoriteTrack(track: Track | null) {
  if (!track?.id) return;

  const localID = await ensureFavoritesPlaylist();
  if (!localID) {
    addToast("Failed to open Favorites playlist", "error");
    return;
  }

  const currentItems = ((await GetLocalPlaylistItems(localID)) ??
    []) as LocalPlaylistItem[];
  const isLocalTrack = isLocalID(track.id);
  const localExisting = currentItems.find(
    (item) =>
      (item.source === "remote" && item.track_id === track.id) ||
      (item.source === "local_ref" && item.local_path === track.id)
  );

  const localAlreadyFavorite = Boolean(localExisting);

  const canSyncRemote = shouldSyncFavoritesToServer() && !isLocalTrack;
  let wasRemoved = localAlreadyFavorite;

  // if the track is a local track, we don't want to sync it to the remote playlist
  // if the track is a remote track, we want to sync it to the remote playlist
  if (canSyncRemote) {
    const remoteID =
      get(favoritesRemotePlaylistId) ?? (await ensureRemoteFavoritesPlaylist());
    if (!remoteID) {
      console.error("Failed to update favorites: no remote favorites playlist");
      addToast("Failed to update Favorites", "error");
      return;
    }

    const read = await serverFetch(`/api/playlists/${remoteID}/items`);
    if (!read.ok) {
      console.error(
        "Failed to update favorites: failed to read remote favorites playlist"
      );
      addToast("Failed to update Favorites", "error");
      return;
    }

    const readData = await read.json();
    const existingItems = (
      Array.isArray(readData) ? readData : (readData.items ?? [])
    ) as RemoteFavoriteItem[];
    const alreadyFavorite = existingItems.some(
      (item) => item.track_id === track.id
    );
    wasRemoved = alreadyFavorite;

    const nextPayload = alreadyFavorite
      ? existingItems
          .filter((item) => item.track_id !== track.id)
          .map(toFavoritesWriteItem)
      : [
          ...existingItems.map(toFavoritesWriteItem),
          toFavoritesWriteItemFromTrack(track)
        ];

    const write = await serverFetch(`/api/playlists/${remoteID}/items`, {
      method: "PUT",
      body: JSON.stringify(nextPayload)
    });
    if (!write.ok) {
      console.error(
        "Failed to update favorites: failed to write remote favorites playlist"
      );
      addToast("Failed to update Favorites", "error");
      return;
    }

    await syncFavoritesFromServer();
  } else {
    const nextItems = localAlreadyFavorite
      ? currentItems.filter(
          (item) =>
            !(
              (item.source === "remote" && item.track_id === track.id) ||
              (item.source === "local_ref" && item.local_path === track.id)
            )
        )
      : [
          ...currentItems,
          {
            position: currentItems.length,
            source: isLocalTrack ? "local_ref" : "remote",
            track_id: isLocalTrack ? "" : track.id,
            local_path: isLocalTrack ? track.id : "",
            ref_title: track.title,
            ref_album: track.album_name,
            ref_album_artist: track.album_artist,
            ref_duration_ms: track.duration_ms,
            added_at: "",
            resolved: false,
            missing: false
          }
        ];

    await SetLocalPlaylistItems(
      localID,
      nextItems.map((item, index) => ({
        ...item,
        position: index
      }))
    );
  }

  await loadPlaylists();
  if (get(selectedPlaylistId) === localID) {
    await selectPlaylist(localID);
  }

  addToast(
    wasRemoved
      ? `Removed "${track.title}" from Favorites`
      : `Added "${track.title}" to Favorites`,
    "success"
  );
}

/** Load all local playlists from the desktop DB. */
export async function loadPlaylists() {
  try {
    await hydrateLocalPlaylistsState();
  } catch (e: any) {
    console.error("loadPlaylists:", e);
  }
}

export async function createPlaylist(name: string, description = "") {
  if (isFavoritesPlaylistMeta(name, description)) {
    return await ensureFavoritesPlaylist();
  }

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
    const pl = get(playlists).find((p) => p.id === id);
    if (isFavoritesPlaylist(pl)) {
      addToast("Favorites playlist cannot be deleted", "warning");
      return;
    }

    await DeleteLocalPlaylist(id);

    // remove from recently played
    removeRecentPlaylist(id);

    // remove from remote server if applicable
    if (pl?.remote_playlist_id) {
      serverFetch(`/api/playlists/${pl.remote_playlist_id}`, {
        method: "DELETE"
      }).catch((e) => console.warn("Failed to delete remote playlist:", e));
    }

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
    const target = get(playlists).find((p) => p.id === id);
    if (isFavoritesPlaylist(target)) {
      addToast("Favorites playlist cannot be edited", "warning");
      return;
    }

    await UpdateLocalPlaylist(id, name, description, artworkPath);
    await loadPlaylists();

    // Refresh selected if it's the one we updated.
    if (get(selectedPlaylistId) === id) {
      await selectPlaylist(id);
    }
  } catch (e: any) {
    console.error("Failed to update playlist:", e);
    addToast(`Failed to update playlist`, "error");
  }
}

// select a playlist and load its items
export async function selectPlaylist(id: string) {
  selectedPlaylistId.set(id);
  playlistsLoading.set(true);

  try {
    const list = get(playlists);
    const summary = list.find((p) => p.id === id) ?? null;
    selectedPlaylist.set(summary);

    const items = (await ResolvePlaylistItems(id)) as
      | LocalPlaylistItem[]
      | null;
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
  track: PlaylistTrackInput,
  isLocal: boolean
) {
  const pl = get(playlists).find((p) => p.id === playlistId);
  const playlistName = pl?.name ?? "playlist";

  // use cached items if this is the selected playlist, else fetch them.
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
    addToast(`Added \"${track.title}\" to \"${playlistName}\"`, "success");

    // Sync queue if this playlist is currently playing.
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

  let added = 0;
  let skipped = 0;
  for (const track of tracks) {
    const isDuplicate = currentItems.some((item) =>
      isLocal
        ? item.local_path && item.local_path === track.path
        : item.track_id && item.track_id === track.id
    );

    if (isDuplicate) {
      skipped++;
      continue;
    }

    try {
      const item = buildLocalPlaylistItem(track, isLocal, 0);
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
  items: LocalPlaylistItem[]
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
  } catch (e: any) {
    addToast(`Failed to reorder playlist: ${e}`, "error");
  }
}

export async function removePlaylistItem(playlistId: string, position: number) {
  const current = get(selectedPlaylistItems);
  const item = current.find((i) => i.position === position) ?? null;
  const playlist = get(playlists).find((pl) => pl.id === playlistId) ?? null;

  // if its a remote favorite track, remove it from the remote favorites playlist
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
  } catch (e: any) {
    addToast(`Failed to upload playlist: ${e}`, "error");
    return null;
  }
}

/** Play a playlist from a given index (builds queue from playlist items). */
export async function playPlaylist(
  items: LocalPlaylistItem[],
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

  // Notify the server about the new track and queue so the server session
  // stays in sync with the client. Without this, the server retains stale
  // state from the previous play, and a subsequent seek causes the server
  // to echo back the wrong track_id which switches playback to a different
  // song. Only send for remote tracks (local paths are unknown to the server).
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

/** Pick and set custom artwork for a playlist (opens file dialog). */
export async function pickPlaylistArtwork(playlistId: string) {
  try {
    const target = get(playlists).find((pl) => pl.id === playlistId);
    if (isFavoritesPlaylist(target)) {
      addToast("Favorites artwork is disabled", "info");
      return;
    }

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

/**
 * Generate a random playlist targeting a specific duration.
 * Falls back to local tracks if not connected to a server.
 */
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
  } catch (e: any) {
    addToast(`Failed to generate playlist: ${e}`, "error");
  }
  return null;
}

export async function initPlaylists() {
  await loadPlaylists();
  if (shouldSyncFavoritesToServer()) {
    await syncFavoritesFromServer();
    await loadPlaylists();
  }
}
