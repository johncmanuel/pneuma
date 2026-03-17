import { writable, get } from "svelte/store";
import { initLocalLibrary, isLocalId } from "../stores/localLibrary";
import { initRecentAlbums } from "../stores/recentAlbums";
import {
  IsConnected,
  GetServerURL,
  GetToken,
  GetLocalPort,
  RestoreSession
} from "../../wailsjs/go/desktop/App";

export const connected = writable(false);

/** The remote server URL (e.g. "http://192.168.1.10:8989"). Empty when disconnected. */
export const serverURL = writable("");

/** JWT token for the remote server. Empty when disconnected. */
export const authToken = writable("");

export const localPort = writable(0);

export const isReconnecting = writable(false);

// Only the server URL and JWT token are persisted.
// The token is short-lived (24 h), rotated automatically, and can be
// revoked server-side
const SESSION_KEY = "pneuma_session";

interface SavedSession {
  url: string;
  token: string;
}

export function saveSession(url: string, token: string) {
  sessionStorage.setItem(SESSION_KEY, JSON.stringify({ url, token }));
  localStorage.setItem(SESSION_KEY, JSON.stringify({ url, token }));
}

export function clearSession() {
  sessionStorage.removeItem(SESSION_KEY);
  localStorage.removeItem(SESSION_KEY);
}

export function loadSession(): SavedSession | null {
  const raw =
    sessionStorage.getItem(SESSION_KEY) ?? localStorage.getItem(SESSION_KEY);

  if (!raw) return null;

  try {
    return JSON.parse(raw);
  } catch {
    return null;
  }
}

export async function initApi() {
  try {
    const port = await GetLocalPort();
    localPort.set(port);
  } catch {
    console.error("Failed to get local port from backend");
  }

  // Load persisted local state from SQLite before any reactive subscribers write.
  await Promise.all([initLocalLibrary(), initRecentAlbums()]);
  await refreshConnection();

  if (!get(connected)) {
    autoReconnect();
  }
}

/** Re-read connection state from the Go backend. */
export async function refreshConnection() {
  try {
    const ok = await IsConnected();
    if (ok) {
      // Populate URL & token BEFORE setting connected so that reactive
      // statements (connectWS, loadRemoteAlbumGroupsPage) see valid values immediately.
      const url = await GetServerURL();
      const token = await GetToken();

      serverURL.set(url);
      authToken.set(token);
      connected.set(true);
    } else {
      serverURL.set("");
      authToken.set("");
      connected.set(false);
    }
  } catch {
    serverURL.set("");
    authToken.set("");
    connected.set(false);
  }
}

let reconnectInterval: ReturnType<typeof setInterval> | null = null;

export async function autoReconnect(onSuccess?: () => void) {
  const session = loadSession();
  if (!session) return;
  if (reconnectInterval) return;

  isReconnecting.set(true);

  // Try an immediate restore before starting the polling loop.
  try {
    await RestoreSession(session.url, session.token);
    await refreshConnection();

    if (get(connected)) {
      // Token was refreshed via RestoreSession, so persist the new one.
      saveSession(session.url, get(authToken));
      isReconnecting.set(false);
      onSuccess?.();
      return;
    }
  } catch {
    // start polling loop below
    console.warn("Auto-restore failed, starting reconnect loop");
  }

  // poll every 5 seconds
  // TODO: might make this configurable or instead have this stop at a certain point
  // but seems good for now
  const pollLoopTimeMs = 5000;

  reconnectInterval = setInterval(async () => {
    if (get(connected)) {
      stopAutoReconnect();
      onSuccess?.();
      return;
    }

    const s = loadSession();

    if (!s) {
      stopAutoReconnect();
      return;
    }

    try {
      await RestoreSession(s.url, s.token);
      await refreshConnection();

      if (get(connected)) {
        saveSession(s.url, get(authToken));
        stopAutoReconnect();
        onSuccess?.();
      }
    } catch (e: any) {
      if (
        typeof e?.message === "string" &&
        e.message.includes("session expired")
      ) {
        clearSession();
        stopAutoReconnect();
      }
    }
  }, pollLoopTimeMs);
}

export function stopAutoReconnect() {
  if (reconnectInterval) {
    clearInterval(reconnectInterval);
    reconnectInterval = null;
  }
  isReconnecting.set(false);
}

/** Base HTTP URL for the remote server, or empty string. */
export function apiBase(): string {
  return get(serverURL);
}

/** Base URL for the local streaming server. */
export function localBase(): string {
  const p = get(localPort);
  return p ? `http://127.0.0.1:${p}` : "";
}

/** Base WebSocket URL for the remote server (ws:// or wss://). */
export function wsBase(): string {
  const url = get(serverURL);
  if (!url) return "";
  return url.replace(/^http/, "ws");
}

/** Fetch from the remote server with Authorization header. */
export async function serverFetch(
  path: string,
  init: RequestInit = {}
): Promise<Response> {
  const base = get(serverURL);

  if (!base) throw new Error("Not connected to server");

  const token = get(authToken);
  const headers = new Headers(init.headers);

  if (token) headers.set("Authorization", `Bearer ${token}`);

  if (!headers.has("Content-Type") && init.body) {
    headers.set("Content-Type", "application/json");
  }
  return fetch(`${base}${path}`, { ...init, headers });
}

/**
 * Returns the stream URL for a track.
 * Local files (identified by path-style IDs) always stream through the local
 * HTTP server, even when connected to a remote server.
 * Remote tracks stream from the server with a JWT query param.
 */
export function streamUrl(trackId: string, localPath?: string): string {
  const p = get(localPort);

  // If the track ID looks like a filesystem path, use the local server
  if (isLocalId(trackId) && p) {
    return `http://127.0.0.1:${p}/local/stream?path=${encodeURIComponent(trackId)}`;
  }

  // If an explicit local path is provided and the port is available, use the local server
  if (localPath && p) {
    return `http://127.0.0.1:${p}/local/stream?path=${encodeURIComponent(localPath)}`;
  }

  // remote track
  const base = get(serverURL);
  const token = get(authToken);
  if (base && token) {
    return `${base}/api/library/tracks/${trackId}/stream?token=${encodeURIComponent(token)}`;
  }

  return "";
}

/** Returns the artwork URL for a track. Local tracks route to the local art server. */
export function artworkUrl(trackId: string): string {
  const p = get(localPort);

  if (isLocalId(trackId) && p) {
    return `http://127.0.0.1:${p}/local/art?path=${encodeURIComponent(trackId)}`;
  }

  const base = get(serverURL);
  const token = get(authToken);

  if (base && token) {
    return `${base}/api/library/tracks/${trackId}/art?token=${encodeURIComponent(token)}`;
  }
  return "";
}

/** URL for a locally stored playlist artwork file. */
export function playlistArtUrl(artworkPath: string): string {
  if (!artworkPath) return "";

  const p = get(localPort);
  if (!p) return "";

  return `http://127.0.0.1:${p}/local/playlist-art?file=${encodeURIComponent(artworkPath)}`;
}
