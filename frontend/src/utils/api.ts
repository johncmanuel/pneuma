import { writable, get } from "svelte/store"
import { initLocalLibrary } from "../stores/localLibrary"
import { initRecentAlbums } from "../stores/recentAlbums"
import {
  IsConnected,
  GetServerURL,
  GetToken,
  GetLocalPort,
  RestoreSession,
} from "../../wailsjs/go/main/App"

/* ── Reactive state ─────────────────────────────────────────────── */

/** Whether the desktop is connected to a remote pneuma server. */
export const connected = writable(false)

/** The remote server URL (e.g. "http://192.168.1.10:8989"). Empty when disconnected. */
export const serverURL = writable("")

/** JWT token for the remote server. Empty when disconnected. */
export const authToken = writable("")

/** The local streaming server port (always running). */
export const localPort = writable(0)

/** Whether the app is currently trying to auto-reconnect. */
export const isReconnecting = writable(false)

/* ── Session persistence ─────────────────────────────────────────── */

// Only the server URL and JWT token are persisted — never the password.
// The token is short-lived (24 h), rotated automatically, and can be
// revoked server-side, making leakage far less damaging than a password.
const SESSION_KEY = "pneuma_session"

interface SavedSession {
  url: string
  token: string
}

export function saveSession(url: string, token: string) {
  sessionStorage.setItem(SESSION_KEY, JSON.stringify({ url, token }))
  localStorage.setItem(SESSION_KEY, JSON.stringify({ url, token }))
}

export function clearSession() {
  sessionStorage.removeItem(SESSION_KEY)
  localStorage.removeItem(SESSION_KEY)
}

export function loadSession(): SavedSession | null {
  // One-time migration: remove any plaintext credentials left by the old storage scheme.
  localStorage.removeItem("pneuma_server_creds")

  // sessionStorage takes priority (same window); fall back to localStorage
  // so it survives an app restart.
  const raw = sessionStorage.getItem(SESSION_KEY) ?? localStorage.getItem(SESSION_KEY)
  if (!raw) return null
  try {
    return JSON.parse(raw)
  } catch {
    return null
  }
}

/* ── Initialisation (call once at startup) ──────────────────────── */

export async function initApi() {
  try {
    const port = await GetLocalPort()
    localPort.set(port)
  } catch {
    // Running outside Wails (e.g. browser dev) — keep 0
  }
  // Load persisted local state from SQLite before any reactive subscribers write.
  await Promise.all([initLocalLibrary(), initRecentAlbums()])
  await refreshConnection()
  // If not connected, try auto-reconnect with saved credentials
  if (!get(connected)) {
    autoReconnect()
  }
}

/** Re-read connection state from the Go backend. */
export async function refreshConnection() {
  try {
    const ok = await IsConnected()
    if (ok) {
      // Populate URL & token BEFORE setting connected so that reactive
      // statements (connectWS, loadRemoteAlbumGroupsPage) see valid values immediately.
      const url = await GetServerURL()
      const token = await GetToken()
      serverURL.set(url)
      authToken.set(token)
      connected.set(true)
    } else {
      serverURL.set("")
      authToken.set("")
      connected.set(false)
    }
  } catch {
    serverURL.set("")
    authToken.set("")
    connected.set(false)
  }
}

/* ── Auto-reconnect ─────────────────────────────────────────────── */

let reconnectInterval: ReturnType<typeof setInterval> | null = null

export async function autoReconnect(onSuccess?: () => void) {
  const session = loadSession()
  if (!session) return
  if (reconnectInterval) return // already running

  isReconnecting.set(true)

  // Try an immediate restore before starting the polling loop.
  try {
    await RestoreSession(session.url, session.token)
    await refreshConnection()
    if (get(connected)) {
      // Token was refreshed by RestoreSession — persist the new one.
      saveSession(session.url, get(authToken))
      isReconnecting.set(false)
      onSuccess?.()
      return
    }
  } catch {
    // Token expired or server unreachable — fall through to polling.
  }

  reconnectInterval = setInterval(async () => {
    if (get(connected)) {
      stopAutoReconnect()
      onSuccess?.()
      return
    }
    const s = loadSession()
    if (!s) {
      stopAutoReconnect()
      return
    }
    try {
      await RestoreSession(s.url, s.token)
      await refreshConnection()
      if (get(connected)) {
        saveSession(s.url, get(authToken))
        stopAutoReconnect()
        onSuccess?.()
      }
    } catch (e: any) {
      // 401/403 means the token is permanently invalid — stop retrying
      // and clear the stale session so the user sees the login form.
      if (typeof e?.message === "string" && e.message.includes("session expired")) {
        clearSession()
        stopAutoReconnect()
      }
      // Otherwise (network error) keep retrying.
    }
  }, 5000)
}

export function stopAutoReconnect() {
  if (reconnectInterval) {
    clearInterval(reconnectInterval)
    reconnectInterval = null
  }
  isReconnecting.set(false)
}

/* ── URL helpers ────────────────────────────────────────────────── */

/** Base HTTP URL for the remote server, or empty string. */
export function apiBase(): string {
  return get(serverURL)
}

/** Base URL for the local streaming server. */
export function localBase(): string {
  const p = get(localPort)
  return p ? `http://127.0.0.1:${p}` : ""
}

/** Base WebSocket URL for the remote server (ws:// or wss://). */
export function wsBase(): string {
  const url = get(serverURL)
  if (!url) return ""
  return url.replace(/^http/, "ws")
}

/* ── Auth-aware fetch wrapper ───────────────────────────────────── */

/** Fetch from the remote server with Authorization header. */
export async function serverFetch(
  path: string,
  init: RequestInit = {},
): Promise<Response> {
  const base = get(serverURL)
  if (!base) throw new Error("Not connected to server")
  const token = get(authToken)
  const headers = new Headers(init.headers)
  if (token) headers.set("Authorization", `Bearer ${token}`)
  if (!headers.has("Content-Type") && init.body) {
    headers.set("Content-Type", "application/json")
  }
  return fetch(`${base}${path}`, { ...init, headers })
}

/* ── Stream / artwork URL helpers ───────────────────────────────── */

/** Detect if a track ID is a local filesystem path rather than a server UUID. */
function isLocalPath(id: string): boolean {
  // Unix absolute path or Windows drive letter (e.g. C:\)
  return id.startsWith("/") || /^[a-zA-Z]:[/\\]/.test(id)
}

/**
 * Returns the stream URL for a track.
 * Local files (identified by path-style IDs) always stream through the local
 * HTTP server, even when connected to a remote server.
 * Remote tracks stream from the server with a JWT query param.
 */
export function streamUrl(trackId: string, localPath?: string): string {
  const p = get(localPort)

  // If the track ID looks like a filesystem path, always use local server
  if (isLocalPath(trackId) && p) {
    return `http://127.0.0.1:${p}/local/stream?path=${encodeURIComponent(trackId)}`
  }

  // If an explicit local path is provided and the port is available, prefer local
  if (localPath && p) {
    return `http://127.0.0.1:${p}/local/stream?path=${encodeURIComponent(localPath)}`
  }

  // Remote server stream
  const base = get(serverURL)
  const token = get(authToken)
  if (base && token) {
    return `${base}/api/library/tracks/${trackId}/stream?token=${encodeURIComponent(token)}`
  }

  return ""
}

/** Returns the artwork URL for a track. Local tracks route to the local art server. */
export function artworkUrl(trackId: string): string {
  const p = get(localPort)
  if (isLocalPath(trackId) && p) {
    return `http://127.0.0.1:${p}/local/art?path=${encodeURIComponent(trackId)}`
  }
  const base = get(serverURL)
  const token = get(authToken)
  if (base && token) {
    return `${base}/api/library/tracks/${trackId}/art?token=${encodeURIComponent(token)}`
  }
  return ""
}
