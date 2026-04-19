import { writable, get } from "svelte/store";
import { loggedIn, wsBase } from "./api";

type LibraryDelta = {
  seq: number;
  type: "track.added" | "track.updated" | "track.removed" | "library.deduped";
  id: string | null;
};

export const libraryDelta = writable<LibraryDelta | null>(null);

export const scanRunning = writable(false);

export const scanResult = writable<{
  added: number;
  updated: number;
  removed: number;
} | null>(null);

let socket: WebSocket | null = null;
let reconnectTimer: ReturnType<typeof setTimeout> | null = null;
let deltaSeq = 0;

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
      libraryDelta.set({
        seq: ++deltaSeq,
        type: msg.type,
        id: msg.payload?.id ? String(msg.payload.id) : null
      });
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
      break;
  }
}
