import { writable } from "svelte/store";
import { playerState } from "./player";
import type { Track } from "./player";
import { loadRemoteAlbumGroupsPage, tracks } from "./library";
import {
  wsBase,
  authToken,
  connected,
  serverFetch,
  autoReconnect
} from "../utils/api";
import { addToast } from "./toasts";
import { isLocalId } from "./localLibrary";
import { get } from "svelte/store";

export const serverDisconnected = writable(false);

let socket: WebSocket | null = null;
let reconnectTimer: ReturnType<typeof setTimeout> | null = null;
let intentionalClose = false;

export function connectWS() {
  if (!get(connected)) return;

  const base = wsBase();
  if (!base) return;

  intentionalClose = false;
  const token = get(authToken);
  const url = token
    ? `${base}/ws?token=${encodeURIComponent(token)}`
    : `${base}/ws`;

  socket = new WebSocket(url);

  socket.onopen = () => {
    if (get(serverDisconnected)) {
      serverDisconnected.set(false);
      addToast("Reconnected to server.", "success");
    }
  };

  socket.onmessage = (e) => {
    const msg = JSON.parse(e.data);
    switch (msg.type) {
      case "track.added":
      case "track.updated":
      case "track.removed":
        loadRemoteAlbumGroupsPage(0);
        break;
      case "library.deduped": {
        const n: number = msg.payload?.removed ?? 0;
        addToast(
          `Removed ${n} duplicate song${n !== 1 ? "s" : ""} from your library.`,
          "warning"
        );
        loadRemoteAlbumGroupsPage(0);
        break;
      }
      case "playback.changed": {
        let currentTracks: Track[] = [];

        tracks.subscribe((v) => {
          currentTracks = v;
        })();

        let trackObj =
          currentTracks.find((t) => t.id === msg.payload.track_id) ?? null;

        // If track not in local store, fetch metadata from server
        if (!trackObj && msg.payload.track_id) {
          fetchRemoteTrack(msg.payload.track_id).then((remote) => {
            if (remote) {
              playerState.update((s) =>
                s.trackId === remote.id ? { ...s, track: remote } : s
              );
            }
          });
        }

        // sync state accordingly since queue deals with mixed tracks, or tracks from both local and remote
        // sources
        playerState.update((s) => {
          // If the client queue contains local file paths the server can't know
          // about, don't let the server overwrite it. The client is the authority
          // for mixed local+remote queues!
          const queueHasLocalTracks = s.queue.some(isLocalId);

          // If the server sent a new queue state and the desktop client currently doesn't
          // have any local tracks in its queue, let the client accept the server's queue.
          const effectiveQueue =
            msg.payload.queue != null && !queueHasLocalTracks
              ? (msg.payload.queue as string[])
              : s.queue;

          // Validate the server's queue_index against the track_id it claims is
          // playing. Seek/pause broadcasts can carry a stale index (e.g. 0) even
          // after the client has already advanced via skipNext. Recompute from
          // the queue when the index doesn't match.
          let resolvedIndex = s.queueIndex;
          if (!queueHasLocalTracks && msg.payload.queue_index != null) {
            const serverIdx: number = msg.payload.queue_index;
            const serverTrackId: string = msg.payload.track_id ?? s.trackId;

            // If the server's index is consistent with the track_id it claims
            // is playing, trust it and update the queue index.
            // Otherwise, the server's index is stale and requires recalculation
            // from the track_id.
            if (effectiveQueue[serverIdx] === serverTrackId) {
              resolvedIndex = serverIdx;
            } else {
              const computed = effectiveQueue.indexOf(serverTrackId);
              resolvedIndex = computed >= 0 ? computed : s.queueIndex;
            }
          }

          // update state accordingly which will update the UI with its new state
          return {
            ...s,
            trackId: msg.payload.track_id ?? s.trackId,
            track: trackObj ?? s.track,
            queue: effectiveQueue,
            queueIndex: resolvedIndex,
            positionMs: msg.payload.position_ms ?? s.positionMs,
            paused:
              msg.payload.playing != null ? !msg.payload.playing : s.paused,
            shuffle: msg.payload.shuffle ?? s.shuffle,
            repeat: msg.payload.repeat ?? s.repeat
          };
        });
        break;
      }
    }
  };

  socket.onclose = () => {
    if (intentionalClose) return;

    connected.set(false);
    serverDisconnected.set(true);

    // attempt to reconnect back
    autoReconnect(() => connectWS());
  };

  socket.onerror = () => socket?.close();
}

export function disconnectWS() {
  intentionalClose = true;
  if (reconnectTimer) clearTimeout(reconnectTimer);
  serverDisconnected.set(false);
  socket?.close();
}

/** Send a message to the server over the open WebSocket. */
export function wsSend(type: string, payload: object) {
  if (socket && socket.readyState === WebSocket.OPEN) {
    socket.send(JSON.stringify({ type, payload }));
  }
}

async function fetchRemoteTrack(trackId: string): Promise<Track | null> {
  try {
    const res = await serverFetch(`/api/library/tracks/${trackId}`);
    if (!res.ok) return null;
    return (await res.json()) as Track;
  } catch {
    return null;
  }
}
