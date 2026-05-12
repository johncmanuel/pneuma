import {
  isFavoritesPlaylist as isFavoritesPlaylistShared,
  visiblePlaylistsForAddMenu as visiblePlaylistsForAddMenuShared,
  pickCanonicalFavoritesPlaylist,
  type LocalPlaylistSummary,
  type LocalPlaylistItem,
  type Track
} from "@pneuma/shared";

interface RemotePlaylistSummary {
  id: string;
  name: string;
  description: string;
  item_count: number;
  total_duration_ms?: number;
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

interface RemotePlaylistItemWrite {
  source: string;
  track_id: string;
  ref_title: string;
  ref_album: string;
  ref_album_artist: string;
  ref_duration_ms: number;
}

type RemotePlaylistDelta = {
  id?: string;
  remote_playlist_id?: string;
  name?: string;
  description?: string;
  item_count?: number;
  total_duration_ms?: number;
  duration_ms?: number;
  total_dur_ms?: number;
  artwork_path?: string;
  created_at?: string;
  updated_at?: string;
  deleted?: boolean;
  items_changed?: boolean;
  metadata_changed?: boolean;
};

type RemotePlaylistDeltaResult = {
  applied: boolean;
  localPlaylistID: string | null;
  wasFavorites: boolean;
  itemsChanged: boolean;
};

type PlaylistTrackInput =
  | Track
  | {
      path: string;
      title: string;
      album: string;
      album_artist: string;
      duration_ms: number;
    };

function trackAlbum(track: PlaylistTrackInput) {
  return "album_name" in track ? track.album_name : track.album;
}

function trackID(track: PlaylistTrackInput): string {
  return "id" in track ? track.id : "";
}

function errorMessage(e: unknown): string {
  return e instanceof Error ? e.message : String(e);
}

function isFiniteNumber(value: unknown): value is number {
  return typeof value === "number" && Number.isFinite(value);
}

function resolveRemoteDurationMS(
  delta: RemotePlaylistDelta,
  fallbackDurationMS: number
) {
  if (isFiniteNumber(delta.total_duration_ms)) return delta.total_duration_ms;
  if (isFiniteNumber(delta.duration_ms)) return delta.duration_ms;
  if (isFiniteNumber(delta.total_dur_ms)) return delta.total_dur_ms;
  return fallbackDurationMS;
}

function resolveLocalArtworkPath(
  delta: RemotePlaylistDelta,
  fallbackArtworkPath: string
) {
  if (typeof delta.artwork_path !== "string") return fallbackArtworkPath;
  return delta.artwork_path.trim() === "" ? "" : fallbackArtworkPath;
}

function mergeLocalPlaylistFromRemoteDelta(
  fallback: LocalPlaylistSummary,
  delta: RemotePlaylistDelta,
  remoteID: string
): LocalPlaylistSummary {
  return {
    ...fallback,
    name: typeof delta.name === "string" ? delta.name : fallback.name,
    description:
      typeof delta.description === "string"
        ? delta.description
        : fallback.description,
    item_count: isFiniteNumber(delta.item_count)
      ? delta.item_count
      : fallback.item_count,
    total_duration_ms: resolveRemoteDurationMS(
      delta,
      fallback.total_duration_ms
    ),
    artwork_path: resolveLocalArtworkPath(delta, fallback.artwork_path),
    remote_playlist_id: remoteID,
    updated_at:
      typeof delta.updated_at === "string"
        ? delta.updated_at
        : fallback.updated_at,
    created_at:
      typeof delta.created_at === "string"
        ? delta.created_at
        : fallback.created_at
  };
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

function isFavoritesPlaylist(
  playlist: LocalPlaylistSummary | null | undefined
) {
  return isFavoritesPlaylistShared(playlist);
}

function visiblePlaylistsForAddMenu(
  list: LocalPlaylistSummary[]
): LocalPlaylistSummary[] {
  return visiblePlaylistsForAddMenuShared(list);
}

export type {
  RemotePlaylistSummary,
  RemoteFavoriteItem,
  RemotePlaylistItemWrite,
  RemotePlaylistDelta,
  RemotePlaylistDeltaResult,
  PlaylistTrackInput
};

export {
  trackAlbum,
  trackID,
  errorMessage,
  isFiniteNumber,
  resolveRemoteDurationMS,
  resolveLocalArtworkPath,
  mergeLocalPlaylistFromRemoteDelta,
  buildLocalPlaylistItem,
  pickCanonicalFavoritesLocal,
  isFavoritesPlaylist,
  visiblePlaylistsForAddMenu,
  remoteToLocalFavoriteItems
};
