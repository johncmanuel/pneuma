import type { LocalPlaylistItem, PlaylistSummary, Track } from "./types";

export const favoritesPlaylistName = "Favorites";
export const favoritesPlaylistMarker = "__pneuma_favorites__";

export interface FavoritesWriteItem {
  source: string;
  track_id: string;
  ref_title: string;
  ref_album: string;
  ref_album_artist: string;
  ref_duration_ms: number;
}

type FavoriteSourceItem = {
  source?: string;
  track_id: string;
  ref_title: string;
  ref_album: string;
  ref_album_artist: string;
  ref_duration_ms: number;
};

export function isFavoritesPlaylistMeta(
  name: string | null | undefined,
  description: string | null | undefined
): boolean {
  const normalizedName = (name ?? "").trim().toLowerCase();
  const normalizedDescription = (description ?? "").trim();

  return (
    normalizedDescription === favoritesPlaylistMarker ||
    normalizedName === favoritesPlaylistName.toLowerCase()
  );
}

export function isFavoritesPlaylist(
  playlist: Pick<PlaylistSummary, "name" | "description"> | null | undefined
): boolean {
  if (!playlist) return false;
  return isFavoritesPlaylistMeta(playlist.name, playlist.description);
}

export function findFavoritesPlaylist<
  T extends { name: string; description: string }
>(playlists: T[]): T | null {
  return (
    playlists.find((pl) => isFavoritesPlaylistMeta(pl.name, pl.description)) ??
    null
  );
}

export function visiblePlaylistsForAddMenu<
  T extends { name: string; description: string }
>(list: T[]): T[] {
  return list.filter((pl) => !isFavoritesPlaylistMeta(pl.name, pl.description));
}

export function pickCanonicalFavoritesPlaylist<
  T extends { description?: string; item_count?: number; updated_at?: string }
>(candidates: T[]): T {
  return [...candidates].sort((a, b) => {
    const byCount = (b.item_count ?? 0) - (a.item_count ?? 0);
    if (byCount !== 0) return byCount;

    const byMarker =
      Number((b.description ?? "") === favoritesPlaylistMarker) -
      Number((a.description ?? "") === favoritesPlaylistMarker);
    if (byMarker !== 0) return byMarker;

    return Date.parse(b.updated_at || "") - Date.parse(a.updated_at || "");
  })[0];
}

export function toFavoritesWriteItem(
  item: FavoriteSourceItem
): FavoritesWriteItem {
  return {
    source: item.source || "remote",
    track_id: item.track_id,
    ref_title: item.ref_title,
    ref_album: item.ref_album,
    ref_album_artist: item.ref_album_artist,
    ref_duration_ms: item.ref_duration_ms
  };
}

export function toFavoritesWriteItemFromTrack(
  track: Track
): FavoritesWriteItem {
  return {
    source: "remote",
    track_id: track.id,
    ref_title: track.title,
    ref_album: track.album_name,
    ref_album_artist: track.album_artist,
    ref_duration_ms: track.duration_ms
  };
}

export function dedupeFavoriteTrackItems(
  items: FavoriteSourceItem[]
): FavoritesWriteItem[] {
  const seen = new Set<string>();
  const out: FavoritesWriteItem[] = [];

  for (const item of items) {
    if (!item.track_id || seen.has(item.track_id)) continue;
    seen.add(item.track_id);
    out.push(toFavoritesWriteItem(item));
  }

  return out;
}

export function favoriteTrackIDsFromItems(
  items: Array<{ source: string; track_id?: string; local_path?: string }>,
  includeLocalRef = false
): Set<string> {
  if (includeLocalRef) {
    return new Set(
      items
        .map((item) =>
          item.source === "remote" ? item.track_id : item.local_path
        )
        .filter((id): id is string => !!id)
    );
  }

  return new Set(
    items
      .filter((item) => item.source === "remote" && !!item.track_id)
      .map((item) => item.track_id as string)
  );
}

export function favoriteItemKey(item: {
  source: string;
  track_id?: string;
  local_path?: string;
}): string {
  return item.source === "local_ref"
    ? `local:${item.local_path ?? ""}`
    : `remote:${item.track_id ?? ""}`;
}

export function hasSameFavoriteKeyOrder(
  a: Array<{ source: string; track_id?: string; local_path?: string }>,
  b: Array<{ source: string; track_id?: string; local_path?: string }>
): boolean {
  const aKeys = a.map(favoriteItemKey).filter((key) => !key.endsWith(":"));
  const bKeys = b.map(favoriteItemKey).filter((key) => !key.endsWith(":"));

  return (
    aKeys.length === bKeys.length && aKeys.every((key, i) => key === bKeys[i])
  );
}

export function mergeRemoteAndLocalFavoriteItems(
  remoteItems: LocalPlaylistItem[],
  localItems: LocalPlaylistItem[]
): LocalPlaylistItem[] {
  const localOnly = localItems.filter(
    (item) => item.source === "local_ref" && !!item.local_path
  );

  return [...remoteItems, ...localOnly].map((item, index) => ({
    ...item,
    position: index
  }));
}
