import { writable, get } from "svelte/store";
import { authToken, wsBase } from "./api";

/**
 * Incremented every time the server reports a library mutation
 * (track.added / track.updated / track.removed / library.deduped).
 * Components can `$: if ($libraryVersion !== prev) reload()` to react.
 */
export const libraryVersion = writable(0);

/** True while the server is running a library scan. */
export const scanRunning = writable(false);

/** Result payload from the last completed scan, or null. */
export const scanResult = writable<{
  added: number;
  updated: number;
  removed: number;
} | null>(null);

let socket: WebSocket | null = null;
let reconnectTimer: ReturnType<typeof setTimeout> | null = null;

export function connectWS() {
  const token = get(authToken);
  if (!token) return;

  // Prevent double-connect: if a socket is already open or connecting, bail out.
  if (
    socket &&
    (socket.readyState === WebSocket.OPEN ||
      socket.readyState === WebSocket.CONNECTING)
  ) {
    return;
  }

  // Close any lingering previous socket before creating a new one.
  if (socket) {
    try {
      socket.close();
    } catch {
      /* ignore */
    }
    socket = null;
  }

  const base = wsBase();
  const url = `${base}/ws?token=${encodeURIComponent(token)}`;

  const ws = new WebSocket(url);
  socket = ws;

  ws.onmessage = (e) => {
    try {
      const msg = JSON.parse(e.data);
      handleMessage(msg);
    } catch {
      /* ignore malformed */
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

/** Fire-and-forget a message to the server over the open WebSocket. */
export function wsSend(type: string, payload: object) {
  if (socket && socket.readyState === WebSocket.OPEN) {
    socket.send(JSON.stringify({ type, payload }));
  }
}

/** No-op stub retained for future player integration. */
// eslint-disable-next-line @typescript-eslint/no-unused-vars
export function setTrackList(_tracks: unknown[]) {}

function handleMessage(msg: { type: string; payload: any }) {
  switch (msg.type) {
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
  }
}
