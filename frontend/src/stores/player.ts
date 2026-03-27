import { writable, derived } from "svelte/store";

// TODO: enum would be better here i believe
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
  artwork_id: string;
}

export function isRemoteTrack(id: string): boolean {
  return !id.startsWith("/") && !/^[a-zA-Z]:[/\\]/.test(id);
}

// TODO: want to migrate to svelte 5 syntax to make use of runes over stores
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
