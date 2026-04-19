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
import { invalidateCachedTrack } from "./stores/library";

const libraryVersion = writable(0);
const scanRunning = writable(false);
const scanResult = writable<{
  added: number;
  updated: number;
  removed: number;
} | null>(null);

let socket: WebSocket | null = null;
let reconnectTimer: ReturnType<typeof setTimeout> | null = null;

export function connectWS() {
  if (!get(loggedIn)) return;

  // Prevent double-connect
  if (
    socket &&
    (socket.readyState === WebSocket.OPEN ||
      socket.readyState === WebSocket.CONNECTING)
  ) {
    return;
  }

  if (socket) {
    try {
      socket.close();
    } catch {
      console.warn("Failed to close existing WebSocket");
    }
    socket = null;
  }

  const base = wsBase();
  const url = `${base}/ws?device_id=${encodeURIComponent(deviceId)}`;

  const ws = new WebSocket(url);
  socket = ws;

  ws.onmessage = (e) => {
    try {
      const msg = JSON.parse(e.data);
      handleMessage(msg);
    } catch {
      console.warn("Failed to parse WebSocket message");
    }
  };

  ws.onclose = () => {
    if (socket !== ws) return;
    socket = null;
    if (get(loggedIn)) {
      reconnectTimer = setTimeout(connectWS, 3000);
    }
  };

  ws.onerror = () => ws.close();
}

export function disconnectWS() {
  if (reconnectTimer) clearTimeout(reconnectTimer);
  reconnectTimer = null;
  socket?.close();
  socket = null;
}

export function wsSend(type: string, payload: object) {
  if (socket && socket.readyState === WebSocket.OPEN) {
    socket.send(JSON.stringify({ type, payload }));
  }
}

// eslint-disable-next-line @typescript-eslint/no-explicit-any
function handleMessage(msg: { type: string; payload: any }) {
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
        invalidateCachedTrack(trackID);
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
        scanResult.set(
          msg.payload as { added: number; updated: number; removed: number }
        );
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
