import { writable } from "svelte/store";
import type { PlayerState } from "@pneuma/shared";

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
