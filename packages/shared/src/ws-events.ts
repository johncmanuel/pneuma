import type { RepeatModeEnum } from "./types";

export const WS_EVENT_TYPES = [
  "track.added",
  "track.updated",
  "track.removed",
  "library.deduped",
  "scan.started",
  "scan.completed",
  "playback.changed",
  "playback.next",
  "playback.prev",
  "playback.play",
  "playback.pause",
  "playback.seek",
  "playback.queue",
  "playback.repeat",
  "playback.shuffle",
  "playlist.created",
  "playlist.updated",
  "playlist.deleted"
] as const;

export type WSEventType = (typeof WS_EVENT_TYPES)[number];

export type PlaybackTrack = {
  id: string;
  title: string;
  album_artist: string;
  album_name: string;
  duration_ms: number;
};

type EmptyPayload = Record<string, never>;

type TrackEventPayload = {
  id?: string;
};

type LibraryDedupedPayload = {
  removed?: number;
};

type ScanCompletedPayload = {
  added: number;
  updated: number;
  removed: number;
};

type PlaybackChangedPayload = {
  track_id?: string;
  track?: PlaybackTrack | null;
  playing?: boolean;
  position_ms?: number;
  queue?: string[];
  queue_index?: number;
  repeat?: RepeatModeEnum;
  shuffle?: boolean;
};

type PlaybackPlayPayload = {
  track_id: string;
  position_ms: number;
};

type PlaybackPausePayload = {
  paused: boolean;
  position_ms?: number;
};

type PlaybackSeekPayload = {
  position_ms: number;
};

type PlaybackQueuePayload = {
  track_ids: string[];
  start_index: number;
};

type PlaybackRepeatPayload = {
  mode: RepeatModeEnum;
};

type PlaybackShufflePayload = {
  enabled: boolean;
};

type PlaylistDeltaPayload = {
  id?: string;
  remote_playlist_id?: string;
  name?: string;
  description?: string;
  item_count?: number;
  total_duration_ms?: number;
  duration_ms?: number;
  total_dur_ms?: number;
  artwork_path?: string;
  track_count?: number;
  created_at?: string;
  updated_at?: string;
  deleted?: boolean;
  items_changed?: boolean;
  metadata_changed?: boolean;
};

export type WSEventPayloads = {
  "track.added": TrackEventPayload;
  "track.updated": TrackEventPayload;
  "track.removed": TrackEventPayload;
  "library.deduped": LibraryDedupedPayload;
  "scan.started": null;
  "scan.completed": ScanCompletedPayload | null;
  "playback.changed": PlaybackChangedPayload;
  "playback.next": EmptyPayload;
  "playback.prev": EmptyPayload;
  "playback.play": PlaybackPlayPayload;
  "playback.pause": PlaybackPausePayload;
  "playback.seek": PlaybackSeekPayload;
  "playback.queue": PlaybackQueuePayload;
  "playback.repeat": PlaybackRepeatPayload;
  "playback.shuffle": PlaybackShufflePayload;
  "playlist.created": PlaylistDeltaPayload;
  "playlist.updated": PlaylistDeltaPayload;
  "playlist.deleted": PlaylistDeltaPayload;
};
