import { writable } from "svelte/store";
import type { Track } from "../types";
import { isLocalId } from "./library";

export type RepeatMode = 0 | 1 | 2;

export interface PlayerState {
  trackId: string;
  track: Track | null;
  queue: string[];
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
  queueIndex: 0,
  positionMs: 0,
  paused: true,
  repeat: 0,
  shuffle: false
};

export const playerState = writable<PlayerState>(initial);

function isLocalPayload(payload: any): boolean {
  const id: string | undefined = payload?.track_id;
  if (!id) return false;
  return isLocalId(id);
}

export function handlePlaybackChanged(payload: any): boolean {
  if (!payload || typeof payload !== "object") return false;

  if (isLocalPayload(payload)) {
    return true;
  }

  const incomingTrackId: string | undefined = payload.track_id;
  const incomingPlaying: boolean | undefined = payload.playing;

  playerState.update((s) => {
    const trackChanged =
      incomingTrackId != null && incomingTrackId !== s.trackId;
    const isPaused = incomingPlaying != null ? !incomingPlaying : s.paused;

    // Only accept the server's position when:
    // 1. the track actually changed (new song), or
    // 2. playback is paused (no local audio advancement).
    //
    // During active playback, the local audio element is the authoritative
    // source of position via onTimeUpdate; the server's echoed position is
    // stale by at least one network round-trip.
    const acceptServerPosition = trackChanged || isPaused;

    return {
      ...s,
      trackId: incomingTrackId ?? s.trackId,
      paused: isPaused,
      positionMs: acceptServerPosition
        ? (payload.position_ms ?? s.positionMs)
        : s.positionMs,
      queue: payload.queue ?? s.queue,
      queueIndex: payload.queue_index ?? s.queueIndex,
      repeat: payload.repeat ?? s.repeat,
      shuffle: payload.shuffle ?? s.shuffle
    };
  });

  return false;
}
