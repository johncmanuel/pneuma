import { writable } from "svelte/store";
import { playerState } from "./player";
import {
  addToast,
  createWSClient,
  type WSEventPayloads,
  type WSEventType,
  type Track
} from "@pneuma/shared";
import { loadRemoteAlbumGroupsPage, tracks } from "./library";
import { getTrackResolver } from "./trackResolver";
import {
  applyRemotePlaylistDelta,
  favoritesRemotePlaylistId,
  favoritesSyncEnabled,
  loadPlaylists,
  selectPlaylist,
  selectedPlaylistId,
  syncAllPlaylistsFromServer,
  syncFavoritesFromServer,
  syncPlaylistFromServer
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

let intentionalClose = false;
let wsClient: ReturnType<typeof createWSClient> | null = null;

export function connectWS() {
  if (!get(connected)) return;

  const base = wsBase();
  if (!base) return;

  intentionalClose = false;
  const token = get(authToken);
  const url = token
    ? `${base}/ws?token=${encodeURIComponent(token)}&device_id=${encodeURIComponent(deviceId)}`
    : `${base}/ws?device_id=${encodeURIComponent(deviceId)}`;

  console.info(`[WS] Connecting to ${base}/ws`);

  wsClient?.close();

  wsClient = createWSClient({
    buildUrl: () => url,
    shouldReconnect: () => get(connected) && !intentionalClose,
    onOpen: () => {
      console.info("[WS] Connection established");
      if (get(serverDisconnected)) {
        serverDisconnected.set(false);
        addToast("Reconnected to server.", "success");
      }

      syncAllPlaylistsFromServer().catch((e) =>
        console.warn("Failed to sync playlists on reconnect:", e)
      );

      if (get(favoritesSyncEnabled)) {
        syncFavoritesFromServer()
          .then(() => loadPlaylists())
          .catch((e) =>
            console.warn("Failed to sync favorites on reconnect:", e)
          );
      }
    },
    onMessage: (msg) => {
      console.debug("[WS] Received:", msg.type, msg.payload);
      handleMessage(msg);
    },
    onParseError: (err, raw) => {
      console.error("[WS] Failed to parse message:", err, raw);
    },
    onClose: (e) => {
      console.info(
        `[WS] Connection closed (code=${e.code}, reason=${e.reason || "none"}, intentional=${intentionalClose})`
      );
      if (intentionalClose) return;

      connected.set(false);
      serverDisconnected.set(true);

      // attempt to reconnect back
      autoReconnect(() => connectWS());
    },
    onError: (err) => {
      console.error("[WS] Connection error:", err);
    }
  });

  wsClient.connect();
}

export function disconnectWS() {
  console.info("[WS] Intentional disconnect");
  intentionalClose = true;
  serverDisconnected.set(false);
  wsClient?.close();
  wsClient = null;
}

/** Send a message to the server over the open WebSocket. */
export function wsSend<Type extends WSEventType>(
  type: Type,
  payload: WSEventPayloads[Type]
) {
  if (wsClient?.getSocket()?.readyState === WebSocket.OPEN) {
    console.debug("[WS] Sending:", type, payload);
    wsClient.send(type, payload);
  } else {
    console.warn("[WS] Cannot send, socket not open:", type);
  }
}

function handleMessage(msg: {
  type: WSEventType;
  payload: WSEventPayloads[WSEventType];
}) {
  switch (msg.type) {
    case "track.added":
    case "track.updated":
    case "track.removed":
      if (msg.payload?.id) {
        getTrackResolver().invalidate(String(msg.payload.id));
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

      if (!remoteID) {
        loadPlaylists()
          .then(async () => {
            if (!get(favoritesSyncEnabled)) return;
            await syncFavoritesFromServer();
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

      const shouldRefreshArt = get(favoritesRemotePlaylistId) !== remoteID;
      const refreshPromise = shouldRefreshArt
        ? RefreshPlaylistArtByRemoteID(remoteID)
        : Promise.resolve();

      refreshPromise
        .then(async () => {
          const deltaResult = await applyRemotePlaylistDelta(msg.payload);

          if (!deltaResult.applied) {
            await syncPlaylistFromServer(remoteID);
            return;
          }

          const shouldSyncFavorites =
            get(favoritesSyncEnabled) &&
            deltaResult.wasFavorites &&
            deltaResult.itemsChanged;

          if (shouldSyncFavorites) {
            await syncFavoritesFromServer();
          }

          const selectedID = get(selectedPlaylistId);
          if (
            deltaResult.itemsChanged &&
            deltaResult.localPlaylistID &&
            selectedID === deltaResult.localPlaylistID
          ) {
            await selectPlaylist(deltaResult.localPlaylistID);
          }
        })
        .catch((e) =>
          console.warn("Failed to apply remote playlist delta:", e)
        );
      break;
    }
    case "playlist.created":
    case "playlist.deleted": {
      const createdRemoteID: string = msg.payload?.id ?? "";

      applyRemotePlaylistDelta(msg.payload)
        .then(async (deltaResult) => {
          if (!deltaResult.applied) {
            // if playlist was created on the server and not yet synced with desktop,
            // sync it
            if (msg.type === "playlist.created" && createdRemoteID) {
              await syncPlaylistFromServer(createdRemoteID);
              return;
            }

            await loadPlaylists();

            if (get(favoritesSyncEnabled)) {
              await syncFavoritesFromServer();
            }

            const selId = get(selectedPlaylistId);
            if (selId) {
              await selectPlaylist(selId);
            }

            return;
          }

          const shouldSyncFavorites =
            get(favoritesSyncEnabled) &&
            (deltaResult.wasFavorites || msg.type === "playlist.created");

          if (shouldSyncFavorites) {
            await syncFavoritesFromServer();
          }

          const selectedID = get(selectedPlaylistId);
          if (selectedID && deltaResult.localPlaylistID === selectedID) {
            if (msg.type === "playlist.deleted") {
              await loadPlaylists();
              return;
            }

            if (deltaResult.itemsChanged) {
              await selectPlaylist(selectedID);
            }
          }
        })
        .catch((e) =>
          console.warn("Failed to apply remote playlist event delta:", e)
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

      if (!trackObj && msg.payload.track_id) {
        fetchRemoteTrack(msg.payload.track_id).then((remote) => {
          if (remote) {
            playerState.update((s) =>
              s.trackId === remote.id ? { ...s, track: remote } : s
            );
          }
        });
      }

      playerState.update((s) => {
        const effectiveQueue =
          s.queue.length > 0 ? s.queue : (msg.payload.queue ?? s.queue);

        let resolvedIndex = s.queueIndex;
        const serverTrackId: string = msg.payload.track_id ?? s.trackId;
        if (serverTrackId && effectiveQueue.length > 0) {
          const computed = effectiveQueue.indexOf(serverTrackId);
          resolvedIndex = computed >= 0 ? computed : s.queueIndex;
        }

        return {
          ...s,
          trackId: msg.payload.track_id ?? s.trackId,
          track: trackObj ?? s.track,
          queue: effectiveQueue,
          queueIndex: resolvedIndex,
          positionMs: msg.payload.position_ms ?? s.positionMs,
          paused: msg.payload.playing != null ? !msg.payload.playing : s.paused,
          shuffle: s.shuffle,
          repeat: s.repeat
        };
      });
      break;
    }
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
