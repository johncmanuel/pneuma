import { writable, derived } from "svelte/store";

export type RepeatMode = 0 | 1 | 2; // Off | Queue | One

export interface Track {
  id: string;
  path: string;
  title: string;
  artist_id: string;
  album_id: string;
  artist_name?: string; // track-level artist (preferred for display)
  album_artist: string; // album artist tag (fallback)
  album_name: string;
  genre: string;
  year: number;
  track_number: number;
  disc_number: number;
  duration_ms: number;
  bitrate_kbps: number;
  replay_gain_track: number;
  artwork_id: string;
}

export interface PlayerState {
  trackId: string;
  track: Track | null;
  queue: string[];
  /** The original album/context order — restored when the queue wraps. Never
   *  contains tracks inserted via "Add to queue". */
  baseQueue: string[];
  queueIndex: number;
  positionMs: number;
  paused: boolean;
  repeat: RepeatMode;
  shuffle: boolean;
}

const initial: PlayerState = {
  trackId: "",
  track: null,
  queue: [],
  baseQueue: [],
  queueIndex: 0,
  positionMs: 0,
  paused: true,
  repeat: 0,
  shuffle: false
};

export const playerState = writable<PlayerState>(initial);

export const isPlaying = derived(
  playerState,
  ($s) => !$s.paused && $s.trackId !== ""
);
