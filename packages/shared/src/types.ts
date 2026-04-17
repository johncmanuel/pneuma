export const RepeatModeEnum = {
  Off: 0,
  All: 1,
  One: 2
} as const;

export const StreamQualityValues = [
  "auto",
  "low",
  "medium",
  "high",
  "original"
] as const;

export type StreamQuality = (typeof StreamQualityValues)[number];

export const RepeatLabels = ["Off", "All", "One"];

export type RepeatModeEnum =
  (typeof RepeatModeEnum)[keyof typeof RepeatModeEnum];

export interface Track {
  id: string;
  path: string;
  title: string;
  artist_id: string;
  album_id: string;
  artist_name?: string;
  album_artist: string;
  album_name: string;
  genre: string;
  year: number;
  track_number: number;
  disc_number: number;
  duration_ms: number;
  bitrate_kbps: number;
  artwork_id: string;
}

export interface AlbumGroup {
  key: string;
  name: string;
  artist: string;
  track_count: number;
  first_track_id: string;
}

export interface AlbumGroupsResponse {
  groups: AlbumGroup[];
  total: number;
  offset: number;
  limit: number;
}

export interface TracksResponse {
  tracks: Track[];
  total: number;
  offset: number;
  limit: number;
}

export interface PlaylistSummary {
  id: string;
  name: string;
  description: string;
  item_count: number;
  total_duration_ms: number;
  duration_ms?: number;
  artwork_path: string;
  remote_playlist_id?: string;
  track_count?: number;
  total_dur_ms?: number;
  created_at?: string;
  updated_at: string;
}

export interface LocalPlaylistSummary {
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

export interface PlaylistMenuItem {
  id: string;
  name: string;
}

export interface PlaylistItem {
  track_id: string;
  position: number;
  added_at: string;
  ref_title: string;
  ref_album: string;
  ref_album_artist: string;
  ref_duration_ms: number;
  source: string;
  missing: boolean;
}

export interface LocalPlaylistItem extends PlaylistItem {
  source: "remote" | "local_ref";
  local_path: string;
  resolved: boolean;
}

export interface PlaybackState {
  playing: boolean;
  track_id: string;
  position_ms: number;
  queue: string[];
  queue_index: number;
  repeat: RepeatModeEnum;
  shuffle: boolean;
}

export interface PlayerState {
  trackId: string;
  track: Track | null;
  queue: string[];
  baseQueue: string[];
  queueIndex: number;
  positionMs: number;
  paused: boolean;
  repeat: RepeatModeEnum;
  shuffle: boolean;
}

export interface SearchResult {
  tracks: Track[];
  albums: AlbumGroup[];
}
