import { writable, derived } from "svelte/store";
import type { PlayerState } from "@pneuma/shared";

export function isRemoteTrack(id: string): boolean {
  return !id.startsWith("/") && !/^[a-zA-Z]:[/\\]/.test(id);
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
