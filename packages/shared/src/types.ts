export type RepeatMode = 0 | 1 | 2; // Off | Queue | One

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
  artwork_path: string;
  owner_id: string;
  updated_at: string;
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

export interface PlaybackState {
  playing: boolean;
  track_id: string;
  position_ms: number;
  queue: string[];
  queue_index: number;
  repeat: RepeatMode;
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
  repeat: RepeatMode;
  shuffle: boolean;
}

export interface SearchResult {
  tracks: Track[];
  albums: AlbumGroup[];
}
