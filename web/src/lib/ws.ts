import { writable, get } from "svelte/store"
import { authToken, wsBase } from "./api"
import { webPlayerState } from "./playerStore"
import type { Track } from "./TrackRow.svelte"

/**
 * Incremented every time the server reports a library mutation
 * (track.added / track.updated / track.removed / library.deduped).
 * Components can `$: if ($libraryVersion !== prev) reload()` to react.
 */
export const libraryVersion = writable(0)

let socket: WebSocket | null = null
let reconnectTimer: ReturnType<typeof setTimeout> | null = null
let allTracks: Track[] = []

/** Provide the tracks list so the WS handler can resolve track objects. */
export function setTrackList(tracks: Track[]) {
  allTracks = tracks
}

export function connectWS() {
  const token = get(authToken)
  if (!token) return

  // Prevent double-connect: if a socket is already open or connecting, bail out.
  if (socket && (socket.readyState === WebSocket.OPEN || socket.readyState === WebSocket.CONNECTING)) {
    return
  }

  // Close any lingering previous socket before creating a new one.
  if (socket) {
    try { socket.close() } catch { /* ignore */ }
    socket = null
  }

  const base = wsBase()
  const url = `${base}/ws?token=${encodeURIComponent(token)}`

  const ws = new WebSocket(url)
  socket = ws

  ws.onmessage = (e) => {
    try {
      const msg = JSON.parse(e.data)
      handleMessage(msg)
    } catch { /* ignore malformed */ }
  }

  ws.onclose = () => {
    // Only act if this is still the active socket (prevents stale closures)
    if (socket !== ws) return
    socket = null
    if (get(authToken)) {
      reconnectTimer = setTimeout(connectWS, 3000)
    }
  }

  ws.onerror = () => ws.close()
}

export function disconnectWS() {
  if (reconnectTimer) clearTimeout(reconnectTimer)
  reconnectTimer = null
  socket?.close()
  socket = null
}

/** Fire-and-forget a message to the server over the open WebSocket. */
export function wsSend(type: string, payload: object) {
  if (socket && socket.readyState === WebSocket.OPEN) {
    socket.send(JSON.stringify({ type, payload }))
  }
}

function handleMessage(msg: { type: string; payload: any }) {
  switch (msg.type) {
    case "track.added":
    case "track.updated":
    case "track.removed":
    case "library.deduped":
      libraryVersion.update((n) => n + 1)
      break
    case "playback.changed": {
      const p = msg.payload
      const trackObj = allTracks.find((t) => t.id === p.track_id) ?? null
      webPlayerState.update((s) => ({
        ...s,
        trackId: p.track_id ?? s.trackId,
        track: trackObj ?? s.track,
        queue: p.queue ?? s.queue,
        queueIndex: p.queue_index ?? s.queueIndex,
        positionMs: p.position_ms ?? s.positionMs,
        paused: p.playing != null ? !p.playing : s.paused,
      }))
      break
    }
    // Library mutations are handled by the page that cares (Library.svelte)
    // via its own polling — no action needed here.
  }
}
