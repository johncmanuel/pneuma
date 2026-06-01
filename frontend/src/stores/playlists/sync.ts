import { get } from "svelte/store";
import {
  CreateLocalPlaylist,
  GetLocalPlaylistItems,
  LinkLocalPlaylistToRemote,
  SetLocalPlaylistItems,
  UpdateLocalPlaylist
} from "../../../wailsjs/go/desktop/App";
import {
  favoritesPlaylistMarker,
  type LocalPlaylistItem,
  type LocalPlaylistSummary
} from "@pneuma/shared";
import { playlists, selectPlaylist, selectedPlaylistId } from "./state";
import { loadPlaylists } from "./favorites";
import { connected, serverFetch } from "../../utils/api";
import { RefreshPlaylistArtByRemoteID } from "../../../wailsjs/go/desktop/App";
import type { RemotePlaylistSummary, RemoteFavoriteItem } from "./helpers";

let syncAllPromise: Promise<void> | null = null;

/**
 * Fetches all remote playlists from the server and ensures each one has a
 * corresponding local playlist linked by remote_playlist_id. Items and
 * metadata are pulled down for any new or updated playlists. Ignores
 * favorites.
 */
export async function syncAllPlaylistsFromServer() {
  if (!get(connected)) return;

  if (syncAllPromise) {
    await syncAllPromise;
    return;
  }

  syncAllPromise = doSyncAll();

  try {
    await syncAllPromise;
  } finally {
    syncAllPromise = null;
  }
}

async function doSyncAll() {
  const remotePlaylists = await fetchRemotePlaylists();
  if (!remotePlaylists) return;

  const nonFavorites = remotePlaylists.filter(
    (pl) => pl.description !== favoritesPlaylistMarker
  );

  if (nonFavorites.length === 0) return;

  const localList = get(playlists);

  for (const remote of nonFavorites) {
    const existing = localList.find(
      (lp) => lp.remote_playlist_id === remote.id
    );

    if (existing) {
      await syncExistingPlaylist(existing, remote);
    } else {
      await createLocalFromRemote(remote);
    }
  }

  await loadPlaylists();

  const selId = get(selectedPlaylistId);
  if (selId) {
    await selectPlaylist(selId);
  }
}

/**
 * Syncs a single remote playlist by its server-side ID. Creates a local
 * playlist if none exists, or updates the existing one.
 */
export async function syncPlaylistFromServer(remoteID: string) {
  if (!get(connected) || !remoteID) return;

  const res = await serverFetch(`/api/playlists/${remoteID}`);
  if (!res.ok) return;

  const remote = (await res.json()) as RemotePlaylistSummary;
  if (remote.description === favoritesPlaylistMarker) return;

  const localList = get(playlists);
  const existing = localList.find((lp) => lp.remote_playlist_id === remoteID);

  if (existing) {
    await syncExistingPlaylist(existing, remote);
  } else {
    await createLocalFromRemote(remote);
  }

  await loadPlaylists();

  const selId = get(selectedPlaylistId);
  if (selId) {
    await selectPlaylist(selId);
  }
}

async function fetchRemotePlaylists(): Promise<RemotePlaylistSummary[] | null> {
  try {
    const res = await serverFetch("/api/playlists");
    if (!res.ok) return null;

    const data = await res.json();
    return (
      Array.isArray(data) ? data : (data.playlists ?? [])
    ) as RemotePlaylistSummary[];
  } catch (e) {
    console.warn("Failed to fetch remote playlists for sync:", e);
    return null;
  }
}

async function fetchRemotePlaylistItems(
  remotePlaylistID: string
): Promise<RemoteFavoriteItem[]> {
  try {
    const res = await serverFetch(
      `/api/playlists/${remotePlaylistID}/items?view=full`
    );
    if (!res.ok) return [];

    const data = await res.json();
    return (
      Array.isArray(data) ? data : (data.items ?? [])
    ) as RemoteFavoriteItem[];
  } catch (e) {
    console.warn("Failed to fetch remote playlist items:", e);
    return [];
  }
}

function remoteItemToLocal(
  item: RemoteFavoriteItem,
  index: number
): LocalPlaylistItem {
  return {
    position: index,
    source: item.source || "remote",
    track_id: item.track_id,
    local_path: "",
    ref_title: item.ref_title,
    ref_album: item.ref_album,
    ref_album_artist: item.ref_album_artist,
    ref_duration_ms: item.ref_duration_ms,
    added_at: item.added_at ?? "",
    resolved: false,
    missing: false
  };
}

async function createLocalFromRemote(remote: RemotePlaylistSummary) {
  try {
    const created = await CreateLocalPlaylist(
      remote.name,
      remote.description ?? ""
    );
    if (!created) return;

    await LinkLocalPlaylistToRemote(created.id, remote.id);

    const remoteItems = await fetchRemotePlaylistItems(remote.id);
    if (remoteItems.length > 0) {
      const localItems = remoteItems.map(remoteItemToLocal);
      await SetLocalPlaylistItems(created.id, localItems);
    }

    RefreshPlaylistArtByRemoteID(remote.id).catch((e) =>
      console.warn("Failed to fetch artwork for new synced playlist:", e)
    );
  } catch (e) {
    console.error("Failed to create local playlist from remote:", e);
  }
}

async function syncExistingPlaylist(
  local: LocalPlaylistSummary,
  remote: RemotePlaylistSummary
) {
  try {
    const metadataChanged =
      local.name !== remote.name ||
      (local.description ?? "") !== (remote.description ?? "");

    if (metadataChanged) {
      await UpdateLocalPlaylist(
        local.id,
        remote.name,
        remote.description ?? "",
        local.artwork_path ?? ""
      );
    }

    const remoteItems = await fetchRemotePlaylistItems(remote.id);
    const localItems = ((await GetLocalPlaylistItems(local.id)) ??
      []) as LocalPlaylistItem[];

    if (!itemsMatch(localItems, remoteItems)) {
      const newLocalItems = remoteItems.map(remoteItemToLocal);
      await SetLocalPlaylistItems(local.id, newLocalItems);
    }

    RefreshPlaylistArtByRemoteID(remote.id).catch((e) =>
      console.warn("Failed to refresh artwork for synced playlist:", e)
    );
  } catch (e) {
    console.error("Failed to sync existing playlist from remote:", e);
  }
}

/**
 * Quick check to see if local items already match remote items by comparing
 * track IDs in order. Avoids unnecessary writes.
 */
function itemsMatch(
  localItems: LocalPlaylistItem[],
  remoteItems: RemoteFavoriteItem[]
): boolean {
  if (localItems.length !== remoteItems.length) return false;

  return localItems.every((local, i) => {
    const remote = remoteItems[i];
    return (
      (local.track_id ?? "") === (remote.track_id ?? "") &&
      (local.source ?? "remote") === (remote.source ?? "remote")
    );
  });
}
