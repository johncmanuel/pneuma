import { writable, get } from "svelte/store"
import {
  IsConnected,
  GetServerURL,
  GetToken,
  GetLocalPort,
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

/* ── Initialisation (call once at startup) ──────────────────────── */

export async function initApi() {
  try {
    const port = await GetLocalPort()
    localPort.set(port)
  } catch {
    // Running outside Wails (e.g. browser dev) — keep 0
  }
  await refreshConnection()
}

/** Re-read connection state from the Go backend. */
export async function refreshConnection() {
  try {
    const ok = await IsConnected()
    connected.set(ok)
    if (ok) {
      serverURL.set(await GetServerURL())
      authToken.set(await GetToken())
    } else {
      serverURL.set("")
      authToken.set("")
    }
  } catch {
    connected.set(false)
    serverURL.set("")
    authToken.set("")
  }
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

/**
 * Returns the stream URL for a track.
 * When connected, uses the server with a short-lived stream token query param.
 * When local-only, uses the local http server (path-based).
 */
export function streamUrl(trackId: string, localPath?: string): string {
  const base = get(serverURL)
  const token = get(authToken)
  if (base && token) {
    return `${base}/api/library/tracks/${trackId}/stream?token=${encodeURIComponent(token)}`
  }
  // Local file streaming through the Wails local server
  const p = get(localPort)
  if (p && localPath) {
    return `http://127.0.0.1:${p}/local/stream?path=${encodeURIComponent(localPath)}`
  }
  return ""
}

/** Returns the artwork URL for a track (server only). */
export function artworkUrl(trackId: string): string {
  const base = get(serverURL)
  const token = get(authToken)
  if (base && token) {
    return `${base}/api/library/tracks/${trackId}/art?token=${encodeURIComponent(token)}`
  }
  return ""
}
