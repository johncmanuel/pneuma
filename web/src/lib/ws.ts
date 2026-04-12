import { writable, get } from "svelte/store";
import { authToken, wsBase, deviceId } from "./api";
import { handlePlaybackChanged } from "./stores/playback";
import {
  favoritesPlaylistId,
  loadPlaylists,
  selectedPlaylist,
  selectPlaylist
} from "./stores/playlists";

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
  const token = get(authToken);
  if (!token) return;

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
  const url = `${base}/ws?token=${encodeURIComponent(token)}&device_id=${encodeURIComponent(deviceId)}`;

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
    if (get(authToken)) {
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
    case "library.deduped":
      libraryVersion.update((n) => n + 1);
      break;
    case "scan.started":
      scanRunning.set(true);
      scanResult.set(null);
      break;
    case "scan.completed":
      scanRunning.set(false);
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
      libraryVersion.update((n) => n + 1);
      loadPlaylists().then(() => {
        // if the favorites playlist was updated, refresh the view
        const favoritesID = get(favoritesPlaylistId);
        const selected = get(selectedPlaylist);
        if (favoritesID && selected?.id === favoritesID) {
          void selectPlaylist(favoritesID);
        }
      });

      // refresh the playlist view if selected
      if (
        msg.type === "playlist.updated" &&
        msg.payload &&
        typeof msg.payload === "object" &&
        "id" in msg.payload &&
        typeof msg.payload.id === "string"
      ) {
        const sel = get(selectedPlaylist);
        if (sel?.id === msg.payload.id) {
          selectedPlaylist.set(msg.payload as typeof sel);
        }
      }
      break;
  }
}
