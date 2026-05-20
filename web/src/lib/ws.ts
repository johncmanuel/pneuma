import { writable, get } from "svelte/store";
import { loggedIn, wsBase, deviceId } from "./api";
import { handlePlaybackChanged } from "./stores/playback";
import {
  clearMissingPlaylistArtID,
  clearMissingTrackArtID,
  resetMissingTrackArtIDs
} from "./stores/missing-art";
import {
  applyPlaylistDelta,
  favoritesPlaylistId,
  loadPlaylists,
  selectedPlaylist,
  selectPlaylist
} from "./stores/playlists";
import { getTrackResolver } from "./track-resolver";
import {
  createWSClient,
  type WSEventPayloads,
  type WSEventType
} from "@pneuma/shared";

const libraryVersion = writable(0);
const scanRunning = writable(false);
const scanResult = writable<WSEventPayloads["scan.completed"]>(null);

const wsClient = createWSClient({
  buildUrl: () => {
    const base = wsBase();
    if (!base) return "";
    return `${base}/ws?device_id=${encodeURIComponent(deviceId)}`;
  },
  shouldReconnect: () => get(loggedIn),
  onMessage: (msg) => handleMessage(msg),
  onParseError: () => {
    console.warn("Failed to parse WebSocket message");
  }
});

export function connectWS() {
  wsClient.connect();
}

export function disconnectWS() {
  wsClient.disconnect();
}

export function wsSend<Type extends WSEventType>(
  type: Type,
  payload: WSEventPayloads[Type]
) {
  wsClient.send(type, payload);
}

function handleMessage(msg: {
  type: WSEventType;
  payload: WSEventPayloads[WSEventType];
}) {
  switch (msg.type) {
    case "playback.changed":
      if (handlePlaybackChanged(msg.payload)) {
        wsSend("playback.next", {});
      }
      break;
    case "track.added":
    case "track.updated":
    case "track.removed":
      // if the track was updated or removed, clear any missing artwork ID for it
      if (msg.payload?.id) {
        const trackID = String(msg.payload.id);
        clearMissingTrackArtID(trackID);
        getTrackResolver().invalidate(trackID);
      }
      libraryVersion.update((n) => n + 1);
      break;
    case "library.deduped":
      libraryVersion.update((n) => n + 1);
      break;
    case "scan.started":
      scanRunning.set(true);
      scanResult.set(null);
      break;
    case "scan.completed":
      scanRunning.set(false);
      resetMissingTrackArtIDs();
      if (msg.payload && typeof msg.payload === "object") {
        scanResult.set(msg.payload);
      }
      libraryVersion.update((n) => n + 1);
      break;
    case "playlist.created":
    case "playlist.updated":
    case "playlist.deleted":
      if (msg.payload?.id) {
        const playlistID = String(msg.payload.id);

        if (msg.type === "playlist.deleted") {
          clearMissingPlaylistArtID(playlistID);
        }

        if (
          msg.type === "playlist.updated" &&
          typeof msg.payload?.artwork_path === "string" &&
          msg.payload.artwork_path.trim() !== ""
        ) {
          clearMissingPlaylistArtID(playlistID);
        }

        libraryVersion.update((n) => n + 1);
        void applyPlaylistDelta(msg.payload);
      } else {
        libraryVersion.update((n) => n + 1);
        void loadPlaylists().then(() => {
          const favoritesID = get(favoritesPlaylistId);
          const selected = get(selectedPlaylist);
          if (favoritesID && selected?.id === favoritesID) {
            void selectPlaylist(favoritesID);
          }
        });
      }
      break;
  }
}
