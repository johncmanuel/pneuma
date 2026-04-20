import { writable } from "svelte/store";
import {
  type PlayerState,
  type Track,
  isLocalID,
  addToast
} from "@pneuma/shared";
import { apiFetch } from "../api";
import { wsSend } from "../ws";

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

function isTrackPayload(value: unknown): value is Track {
  if (!value || typeof value !== "object") return false;

  const candidate = value as Partial<Track>;
  return (
    typeof candidate.id === "string" &&
    typeof candidate.title === "string" &&
    typeof candidate.album_name === "string" &&
    typeof candidate.album_artist === "string"
  );
}

/**
 * Fetch the current playback state from the server and hydrate the store.
 * Called on auth success so the player restores where the user left off.
 */
export async function loadPlaybackState() {
  try {
    const res = await apiFetch("/api/playback");
    if (!res.ok) {
      playerState.set(initial);
      return;
    }
    const s = await res.json();
    playerState.set({
      trackId: s.track_id ?? "",
      track: isTrackPayload(s.track) ? s.track : null,

      queue: s.queue ?? [],
      baseQueue: s.queue ?? [],
      queueIndex: s.queue_index ?? 0,
      positionMs: s.position_ms ?? 0,

      // start paused to avoid playing immediately
      paused: true,

      repeat: s.repeat ?? 0,
      shuffle: s.shuffle ?? false
    });
  } catch (e) {
    console.warn("Failed to load playback state:", e);
  }
}

// eslint-disable-next-line @typescript-eslint/no-explicit-any
function isLocalPayload(payload: any): boolean {
  const id: string | undefined = payload?.track_id;
  if (!id) return false;
  return isLocalID(id);
}

// eslint-disable-next-line @typescript-eslint/no-explicit-any
export function handlePlaybackChanged(payload: any): boolean {
  if (!payload || typeof payload !== "object") return false;

  if (isLocalPayload(payload)) {
    return true;
  }

  const incomingTrackId: string | undefined = payload.track_id;
  const incomingPlaying: boolean | undefined = payload.playing;
  const incomingTrack = isTrackPayload(payload.track) ? payload.track : null;

  playerState.update((s) => {
    const trackChanged =
      incomingTrackId != null && incomingTrackId !== s.trackId;
    const isPaused = incomingPlaying != null ? !incomingPlaying : s.paused;
    const nextTrack = incomingTrack ?? (trackChanged ? null : s.track);

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
      track: nextTrack,
      paused: isPaused,
      positionMs: acceptServerPosition
        ? (payload.position_ms ?? s.positionMs)
        : s.positionMs,
      queue:
        payload.queue != null && payload.queue.length > 0
          ? payload.queue
          : s.queue,
      queueIndex: payload.queue_index ?? s.queueIndex,
      repeat: payload.repeat ?? s.repeat,
      shuffle: payload.shuffle ?? s.shuffle
    };
  });

  return false;
}

export function appendTrackToQueue(track: Track) {
  playerState.update((s) => {
    const insertAt = s.queueIndex + 1;
    const newQueue = [
      ...s.queue.slice(0, insertAt),
      track.id,
      ...s.queue.slice(insertAt)
    ];

    wsSend("playback.queue", {
      track_ids: newQueue,
      start_index: s.queueIndex
    });

    return { ...s, queue: newQueue };
  });
  addToast(`Added "${track.title}" to queue.`, "success");
}
