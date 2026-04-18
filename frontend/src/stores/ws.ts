import { writable } from "svelte/store";
import { playerState } from "./player";
import { type Track, addToast } from "@pneuma/shared";
import {
  invalidateCachedTrack,
  loadRemoteAlbumGroupsPage,
  tracks
} from "./library";
import {
  favoritesRemotePlaylistId,
  favoritesSyncEnabled,
  loadPlaylists,
  selectPlaylist,
  selectedPlaylistId,
  syncFavoritesFromServer
} from "./playlists";
import {
  wsBase,
  authToken,
  connected,
  serverFetch,
  autoReconnect,
  deviceId
} from "../utils/api";
import { get } from "svelte/store";
import { RefreshPlaylistArtByRemoteID } from "../../wailsjs/go/desktop/App";

export const serverDisconnected = writable(false);

let socket: WebSocket | null = null;
const reconnectTimer: ReturnType<typeof setTimeout> | null = null;
let intentionalClose = false;

export function connectWS() {
  if (!get(connected)) return;

  const base = wsBase();
  if (!base) return;

  intentionalClose = false;
  const token = get(authToken);
  const url = token
    ? `${base}/ws?token=${encodeURIComponent(token)}&device_id=${encodeURIComponent(deviceId)}`
    : `${base}/ws?device_id=${encodeURIComponent(deviceId)}`;

  const maskedUrl = token
    ? `${base}/ws?token=***&device_id=${encodeURIComponent(deviceId)}`
    : url;
  console.info(`[WS] Connecting to ${maskedUrl}`);

  socket = new WebSocket(url);

  socket.onopen = () => {
    console.info("[WS] Connection established");
    if (get(serverDisconnected)) {
      serverDisconnected.set(false);
      addToast("Reconnected to server.", "success");
    }

    if (get(favoritesSyncEnabled)) {
      syncFavoritesFromServer()
        .then(() => loadPlaylists())
        .catch((e) =>
          console.warn("Failed to sync favorites on reconnect:", e)
        );
    }
  };

  socket.onmessage = (e) => {
    try {
      const msg = JSON.parse(e.data);
      console.debug("[WS] Received:", msg.type, msg.payload);
      switch (msg.type) {
        case "track.added":
        case "track.updated":
        case "track.removed":
          if (msg.payload?.id) {
            invalidateCachedTrack(String(msg.payload.id));
          }
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
        case "playlist.updated": {
          const remoteID: string = msg.payload?.id ?? "";
          if (remoteID) {
            const shouldRefreshArt =
              get(favoritesRemotePlaylistId) !== remoteID;

            const refreshPromise = shouldRefreshArt
              ? RefreshPlaylistArtByRemoteID(remoteID)
              : Promise.resolve();

            refreshPromise
              .then(() => loadPlaylists())
              // if the favorites playlist was updated, refresh the view
              .then(async () => {
                if (!get(favoritesSyncEnabled)) return;

                const favoritesRemoteID = get(favoritesRemotePlaylistId);
                if (!favoritesRemoteID || favoritesRemoteID === remoteID) {
                  await syncFavoritesFromServer();
                }
              })
              .then(() => {
                const selId = get(selectedPlaylistId);
                if (selId) return selectPlaylist(selId);
              })
              .catch((e) =>
                console.warn("Failed to refresh playlist art from server:", e)
              );
          }
          break;
        }
        case "playlist.created":
        case "playlist.deleted": {
          loadPlaylists()
            .then(async () => {
              if (get(favoritesSyncEnabled)) {
                await syncFavoritesFromServer();
              }
            })
            .then(() => {
              const selId = get(selectedPlaylistId);
              if (selId) return selectPlaylist(selId);
            })
            .catch((e) =>
              console.warn("Failed to refresh playlists from server event:", e)
            );
          break;
        }
        case "playback.changed": {
          let currentTracks: Track[] = [];

          tracks.subscribe((v) => {
            currentTracks = v;
          })();

          const trackObj =
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
          // sources. client's queue is the authoritative source of truth since the server's queue is always stale after shuffle or local-only queue modifications,
          // only accept the server's queue if the client doesn't have one yet, usually during initial connect / first play
          playerState.update((s) => {
            const effectiveQueue =
              s.queue.length > 0 ? s.queue : (msg.payload.queue ?? s.queue);

            // Resolve queue index from the track_id against the client's
            // queue. The server's index is based on the server's stale
            // queue and would point to the wrong position after shuffle.
            let resolvedIndex = s.queueIndex;
            const serverTrackId: string = msg.payload.track_id ?? s.trackId;
            if (serverTrackId && effectiveQueue.length > 0) {
              const computed = effectiveQueue.indexOf(serverTrackId);
              resolvedIndex = computed >= 0 ? computed : s.queueIndex;
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
              // The desktop app manages shuffle entirely client-side, so the server's value
              // is always stale. Preserve the client's shuffle/repeat on
              // every playback.changed echo; they only change when the user
              // explicitly toggles them via the desktop UI.
              shuffle: s.shuffle,
              repeat: s.repeat
            };
          });
          break;
        }
      }
    } catch (err) {
      console.error("[WS] Failed to parse message:", err, e.data);
    }
  };

  socket.onclose = (e) => {
    console.info(
      `[WS] Connection closed (code=${e.code}, reason=${e.reason || "none"}, intentional=${intentionalClose})`
    );
    if (intentionalClose) return;

    connected.set(false);
    serverDisconnected.set(true);

    // attempt to reconnect back
    autoReconnect(() => connectWS());
  };

  socket.onerror = (err) => {
    console.error("[WS] Connection error:", err);
    socket?.close();
  };
}

export function disconnectWS() {
  console.info("[WS] Intentional disconnect");
  intentionalClose = true;
  if (reconnectTimer) clearTimeout(reconnectTimer);
  serverDisconnected.set(false);
  socket?.close();
}

/** Send a message to the server over the open WebSocket. */
export function wsSend(type: string, payload: object) {
  if (socket && socket.readyState === WebSocket.OPEN) {
    console.debug("[WS] Sending:", type, payload);
    socket.send(JSON.stringify({ type, payload }));
  } else {
    console.warn("[WS] Cannot send, socket not open:", type);
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
