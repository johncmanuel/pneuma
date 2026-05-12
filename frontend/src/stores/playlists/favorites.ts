import { writable, get } from "svelte/store";
import {
  CreateLocalPlaylist,
  DeleteLocalPlaylist,
  GetLocalPlaylistItems,
  GetLocalPlaylists,
  LinkLocalPlaylistToRemote,
  ResolvePlaylistItems,
  SetLocalPlaylistItems,
  UpdateLocalPlaylist
} from "../../../wailsjs/go/desktop/App";
import {
  addToast,
  dedupeFavoriteTrackItems,
  favoriteTrackIDsFromItems,
  findFavoritesPlaylist,
  favoritesPlaylistMarker,
  favoritesPlaylistName,
  hasSameFavoriteKeyOrder,
  isLocalID,
  mergeRemoteAndLocalFavoriteItems,
  pickCanonicalFavoritesPlaylist,
  storageKeys,
  toFavoritesWriteItemFromTrack,
  type Track,
  type LocalPlaylistSummary,
  type LocalPlaylistItem
} from "@pneuma/shared";
import {
  playlists,
  selectedPlaylistId,
  selectedPlaylistItems,
  selectedPlaylist,
  favoriteTrackIDs as favoriteTrackIDsStore,
  favoritesPlaylistId,
  favoritesRemotePlaylistId,
  selectPlaylist
} from "./state";
import {
  type RemotePlaylistSummary,
  type RemoteFavoriteItem,
  type RemotePlaylistItemWrite,
  type RemotePlaylistDelta,
  type RemotePlaylistDeltaResult,
  pickCanonicalFavoritesLocal,
  mergeLocalPlaylistFromRemoteDelta,
  remoteToLocalFavoriteItems
} from "./helpers";
import { connected, serverFetch } from "../../utils/api";

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

let favoritesSyncPromise: Promise<void> | null = null;

async function normalizeLocalFavorites(
  list: LocalPlaylistSummary[],
  preferredRemoteID: string | null
): Promise<{
  normalized: LocalPlaylistSummary[];
  canonicalID: string | null;
}> {
  const candidates = list.filter(
    (pl) => pl.description === favoritesPlaylistMarker
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
      const key =
        item.source === "local_ref"
          ? `local:${item.local_path ?? ""}`
          : `remote:${item.track_id ?? ""}`;

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

async function refreshFavoriteTrackIDsFromLocal(
  localPlaylistID: string | null
) {
  if (!localPlaylistID) {
    favoriteTrackIDsStore.set(new Set());
    return;
  }

  try {
    const items = ((await GetLocalPlaylistItems(localPlaylistID)) ?? []) as
      | LocalPlaylistItem[]
      | null;
    favoriteTrackIDsStore.set(favoriteTrackIDsFromItems(items ?? [], true));
  } catch {
    favoriteTrackIDsStore.set(new Set());
  }
}

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

async function ensureRemoteFavoritesPlaylist(): Promise<string | null> {
  if (!get(connected)) return null;

  const listRes = await serverFetch("/api/playlists");
  if (!listRes.ok) return null;

  const listData = await listRes.json();
  const remotePlaylists = (
    Array.isArray(listData) ? listData : (listData.playlists ?? [])
  ) as RemotePlaylistSummary[];
  const candidates = remotePlaylists.filter(
    (pl) => pl.description === favoritesPlaylistMarker
  );

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

      await serverFetch(`/api/playlists/${canonical.id}/items?view=full`, {
        method: "PUT",
        body: JSON.stringify(deduped)
      });

      for (const pl of candidates) {
        if (pl.id === canonical.id) continue;
        await serverFetch(`/api/playlists/${pl.id}`, { method: "DELETE" });
      }
    }

    if (
      canonical.name !== favoritesPlaylistName ||
      (canonical.description ?? "") !== favoritesPlaylistMarker
    ) {
      await serverFetch(`/api/playlists/${canonical.id}`, {
        method: "PUT",
        body: JSON.stringify({
          name: favoritesPlaylistName,
          description: favoritesPlaylistMarker
        })
      });
    }

    favoritesRemotePlaylistId.set(canonical.id);
    return canonical.id;
  }

  const createRes = await serverFetch("/api/playlists", {
    method: "POST",
    body: JSON.stringify({
      name: favoritesPlaylistName,
      description: favoritesPlaylistMarker
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
  const res = await serverFetch(
    `/api/playlists/${remotePlaylistID}/items?view=full`
  );
  if (!res.ok) return [];

  const data = await res.json();
  const items = (
    Array.isArray(data) ? data : (data.items ?? [])
  ) as RemoteFavoriteItem[];
  return items.filter((item) => Boolean(item.track_id));
}

async function appendRemotePlaylistItems(
  remotePlaylistID: string,
  items: RemotePlaylistItemWrite[]
) {
  if (items.length === 0) return true;

  const res = await serverFetch(
    `/api/playlists/${remotePlaylistID}/items/append`,
    {
      method: "POST",
      body: JSON.stringify({ items })
    }
  );

  return res.ok;
}

async function fetchRemotePlaylistSummary(
  remotePlaylistID: string
): Promise<RemotePlaylistDelta | null> {
  if (!remotePlaylistID) return null;

  const res = await serverFetch(`/api/playlists/${remotePlaylistID}`);
  if (!res.ok) return null;

  const payload = (await res.json()) as RemotePlaylistSummary & {
    artwork_path?: string;
    remote_playlist_id?: string;
    created_at?: string;
    updated_at?: string;
  };

  return {
    id: payload.id,
    name: payload.name,
    description: payload.description,
    item_count: payload.item_count,
    total_duration_ms: payload.total_duration_ms,
    artwork_path: payload.artwork_path,
    remote_playlist_id: payload.remote_playlist_id,
    created_at: payload.created_at,
    updated_at: payload.updated_at,
    metadata_changed: true
  };
}

async function removeRemotePlaylistItemByPosition(
  remotePlaylistID: string,
  position: number
) {
  const res = await serverFetch(
    `/api/playlists/${remotePlaylistID}/items/${position}`,
    {
      method: "DELETE"
    }
  );

  return res.ok;
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
    if (remoteID && existing.remote_playlist_id !== remoteID) {
      await LinkLocalPlaylistToRemote(existing.id, remoteID);
      await hydrateLocalPlaylistsState();
    }
    return existing.id;
  }

  const created = await CreateLocalPlaylist(
    favoritesPlaylistName,
    favoritesPlaylistMarker
  ).catch(() => console.error("Failed to create local favorites playlist"));
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

export async function applyRemotePlaylistDelta(
  delta: RemotePlaylistDelta
): Promise<RemotePlaylistDeltaResult> {
  const remoteID = (delta.id ?? delta.remote_playlist_id ?? "").trim();
  if (!remoteID) {
    return {
      applied: false,
      localPlaylistID: null,
      wasFavorites: false,
      itemsChanged: Boolean(delta.items_changed)
    };
  }

  const local = get(playlists).find((pl) => pl.remote_playlist_id === remoteID);
  if (!local) {
    return {
      applied: false,
      localPlaylistID: null,
      wasFavorites: false,
      itemsChanged: Boolean(delta.items_changed)
    };
  }

  const wasFavorites = get(favoritesPlaylistId) === local.id;

  if (delta.deleted) {
    const unlinkTime =
      typeof delta.updated_at === "string"
        ? delta.updated_at
        : new Date().toISOString();

    playlists.update((list) =>
      list.map((playlist) =>
        playlist.id === local.id
          ? {
              ...playlist,
              remote_playlist_id: "",
              updated_at: unlinkTime
            }
          : playlist
      )
    );

    selectedPlaylist.update((selected) =>
      selected?.id === local.id
        ? {
            ...selected,
            remote_playlist_id: "",
            updated_at: unlinkTime
          }
        : selected
    );

    if (wasFavorites && get(favoritesRemotePlaylistId) === remoteID) {
      favoritesRemotePlaylistId.set(null);
    }

    await LinkLocalPlaylistToRemote(local.id, "").catch((e) =>
      console.warn("Failed to unlink local playlist from deleted remote", e)
    );

    return {
      applied: true,
      localPlaylistID: local.id,
      wasFavorites,
      itemsChanged: Boolean(delta.items_changed)
    };
  }

  const merged = mergeLocalPlaylistFromRemoteDelta(local, delta, remoteID);

  playlists.update((list) =>
    list.map((playlist) => (playlist.id === local.id ? merged : playlist))
  );

  selectedPlaylist.update((selected) =>
    selected?.id === local.id ? merged : selected
  );

  if (wasFavorites && get(favoritesRemotePlaylistId) !== remoteID) {
    favoritesRemotePlaylistId.set(remoteID);
  }

  const metadataChanged =
    merged.name !== local.name ||
    merged.description !== local.description ||
    merged.artwork_path !== local.artwork_path;

  if (metadataChanged) {
    await UpdateLocalPlaylist(
      local.id,
      merged.name,
      merged.description,
      merged.artwork_path ?? ""
    ).catch((e) =>
      console.warn("Failed to persist remote playlist metadata delta", e)
    );
  }

  return {
    applied: true,
    localPlaylistID: local.id,
    wasFavorites,
    itemsChanged: Boolean(delta.items_changed)
  };
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
  const prevIDs = get(favoriteTrackIDsStore);
  const optimisticIDs = new Set(prevIDs);

  if (localAlreadyFavorite) {
    optimisticIDs.delete(track.id);
  } else {
    optimisticIDs.add(track.id);
  }

  favoriteTrackIDsStore.set(optimisticIDs);

  const canSyncRemote = shouldSyncFavoritesToServer() && !isLocalTrack;
  let wasRemoved = localAlreadyFavorite;

  if (canSyncRemote) {
    const remoteID =
      get(favoritesRemotePlaylistId) ?? (await ensureRemoteFavoritesPlaylist());

    if (!remoteID) {
      console.error("Failed to update favorites: no remote favorites playlist");
      favoriteTrackIDsStore.set(prevIDs);
      addToast("Failed to update Favorites", "error");
      return;
    }

    const read = await serverFetch(
      `/api/playlists/${remoteID}/items?view=full`
    );
    if (!read.ok) {
      console.error(
        "Failed to update favorites: failed to read remote favorites playlist"
      );
      favoriteTrackIDsStore.set(prevIDs);
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

    const position = existingItems.findIndex(
      (item) => item.track_id === track.id
    );

    const ok = alreadyFavorite
      ? position >= 0
        ? await removeRemotePlaylistItemByPosition(remoteID, position)
        : true
      : await appendRemotePlaylistItems(remoteID, [
          toFavoritesWriteItemFromTrack(track)
        ]);

    if (!ok) {
      console.error(
        "Failed to update favorites: failed to write remote favorites playlist"
      );
      favoriteTrackIDsStore.set(prevIDs);
      addToast("Failed to update Favorites", "error");
      return;
    }

    await syncFavoritesFromServer();

    const remoteSummary = await fetchRemotePlaylistSummary(remoteID);
    if (remoteSummary) {
      await applyRemotePlaylistDelta({
        ...remoteSummary,
        items_changed: true,
        updated_at: new Date().toISOString()
      });
    }
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

  if (!canSyncRemote) {
    await loadPlaylists();
  }

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

export async function loadPlaylists() {
  try {
    const list = await hydrateLocalPlaylistsState();
    if (!findFavoritesPlaylist(list)) {
      await CreateLocalPlaylist(
        favoritesPlaylistName,
        favoritesPlaylistMarker
      ).catch((e) =>
        console.error("Failed to create local favorites playlist:", e)
      );
      await hydrateLocalPlaylistsState();
    }
  } catch (e) {
    console.error("loadPlaylists:", e);
  }
}

async function migrateNameOnlyFavoritesPlaylists(list: LocalPlaylistSummary[]) {
  const nameOnly = list.filter(
    (pl) =>
      pl.name.trim().toLowerCase() === favoritesPlaylistName.toLowerCase() &&
      pl.description !== favoritesPlaylistMarker
  );
  await Promise.all(
    nameOnly.map((pl) =>
      UpdateLocalPlaylist(
        pl.id,
        pl.name,
        favoritesPlaylistMarker,
        pl.artwork_path ?? ""
      )
    )
  );
}

export async function initPlaylists() {
  const initial = ((await GetLocalPlaylists()) ?? []) as LocalPlaylistSummary[];

  await migrateNameOnlyFavoritesPlaylists(initial);
  await loadPlaylists();

  if (shouldSyncFavoritesToServer()) {
    await syncFavoritesFromServer();
    await loadPlaylists();
  }
}
