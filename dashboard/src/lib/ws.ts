import { writable, get } from "svelte/store";
import { loggedIn, wsBase } from "./api";

/**
 * Incremented every time the server reports a library mutation
 * (track.added / track.updated / track.removed / library.deduped).
 * Components can `$: if ($libraryVersion !== prev) reload()` to react.
 */
export const libraryVersion = writable(0);

export const scanRunning = writable(false);

export const scanResult = writable<{
  added: number;
  updated: number;
  removed: number;
} | null>(null);

let socket: WebSocket | null = null;
let reconnectTimer: ReturnType<typeof setTimeout> | null = null;

export function connectWS() {
  if (!get(loggedIn)) return;

  // Prevent double-connect: if a socket is already open or connecting, stop the
  // connection
  if (
    socket &&
    (socket.readyState === WebSocket.OPEN ||
      socket.readyState === WebSocket.CONNECTING)
  ) {
    return;
  }

  // Close any old previous sockets before creating a new one.
  if (socket) {
    try {
      socket.close();
    } catch {
      console.warn("Failed to close existing WebSocket");
    }
    socket = null;
  }

  const base = wsBase();
  const url = `${base}/ws`;

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

// eslint-disable-next-line @typescript-eslint/no-explicit-any
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
