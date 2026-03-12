import { writable, derived } from "svelte/store";
import type { Track } from "./TrackRow.svelte";

export interface WebPlayerState {
  trackId: string;
  track: Track | null;
  queue: string[];
  queueIndex: number;
  positionMs: number;
  paused: boolean;
}

const initial: WebPlayerState = {
  trackId: "",
  track: null,
  queue: [],
  queueIndex: 0,
  positionMs: 0,
  paused: true
};

export const webPlayerState = writable<WebPlayerState>(initial);

export const isPlaying = derived(
  webPlayerState,
  ($s) => !$s.paused && $s.trackId !== ""
);
