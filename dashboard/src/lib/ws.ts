import { writable, get } from "svelte/store";
import { loggedIn, wsBase } from "./api";
import {
  createWSClient,
  type WSEventPayloads,
  type WSEventType
} from "@pneuma/shared";

type LibraryDelta = {
  seq: number;
  type: "track.added" | "track.updated" | "track.removed" | "library.deduped";
  id: string | null;
};

export const libraryDelta = writable<LibraryDelta | null>(null);

export const scanRunning = writable(false);

export const scanResult = writable<WSEventPayloads["scan.completed"]>(null);

const wsClient = createWSClient({
  buildUrl: () => {
    const base = wsBase();
    return base ? `${base}/ws` : "";
  },
  shouldReconnect: () => get(loggedIn),
  onMessage: (msg) => handleMessage(msg),
  onParseError: () => {
    console.warn("Failed to parse WebSocket message");
  }
});
let deltaSeq = 0;

export function connectWS() {
  wsClient.connect();
}

export function disconnectWS() {
  wsClient.disconnect();
}

function handleMessage(msg: {
  type: WSEventType;
  payload: WSEventPayloads[WSEventType];
}) {
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
        scanResult.set(msg.payload);
      }
      break;
  }
}
