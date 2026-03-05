import { writable, get } from "svelte/store"
import {
  IsConnected,
  GetServerURL,
  GetToken,
  GetLocalPort,
  ConnectToServer,
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

/* ── Credential persistence ─────────────────────────────────────── */

const CRED_KEY = "pneuma_server_creds"

interface SavedCreds {
  url: string
  username: string
  password: string
}

export function saveCredentials(url: string, username: string, password: string) {
  localStorage.setItem(CRED_KEY, JSON.stringify({ url, username, password }))
}

export function clearCredentials() {
  localStorage.removeItem(CRED_KEY)
}

export function loadCredentials(): SavedCreds | null {
  const raw = localStorage.getItem(CRED_KEY)
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
      // statements (connectWS, loadTracks) see valid values immediately.
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

export function autoReconnect(onSuccess?: () => void) {
  const creds = loadCredentials()
  if (!creds) return
  if (reconnectInterval) return // already running

  isReconnecting.set(true)

  reconnectInterval = setInterval(async () => {
    if (get(connected)) {
      stopAutoReconnect()
      onSuccess?.()
      return
    }
    try {
      await ConnectToServer(creds.url, creds.username, creds.password)
      await refreshConnection()
      if (get(connected)) {
        stopAutoReconnect()
        onSuccess?.()
      }
    } catch {
      // will retry next interval
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
