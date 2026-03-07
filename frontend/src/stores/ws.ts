import { writable } from "svelte/store"
import { playerState } from "./player"
import type { Track } from "./player"
import { loadTracks, loadRemoteAlbumGroupsPage, tracks } from "./library"
import { wsBase, authToken, connected, serverFetch, autoReconnect } from "../utils/api"
import { addToast } from "./toasts"
import { get } from "svelte/store"

/** True when the WS connection dropped unexpectedly (drives the banner). */
export const serverDisconnected = writable(false)

let socket: WebSocket | null = null
let reconnectTimer: ReturnType<typeof setTimeout> | null = null
let intentionalClose = false

export function connectWS() {
  if (!get(connected)) return
  const base = wsBase()
  if (!base) return

  intentionalClose = false
  const token = get(authToken)
  const url = token
    ? `${base}/ws?token=${encodeURIComponent(token)}`
    : `${base}/ws`

  socket = new WebSocket(url)

  socket.onopen = () => {
    // We (re)connected — hide the disconnect banner and show a brief toast
    if (get(serverDisconnected)) {
      serverDisconnected.set(false)
      addToast("Reconnected to server.", "success")
    }
  }

  socket.onmessage = (e) => {
    const msg = JSON.parse(e.data)
    switch (msg.type) {
      case "track.added":
      case "track.updated":
      case "track.removed":
        loadTracks()
        loadRemoteAlbumGroupsPage(0)
        break
      case "library.deduped": {
        const n: number = msg.payload?.removed ?? 0
        addToast(
          `Removed ${n} duplicate song${n !== 1 ? "s" : ""} from your library.`,
          "warning"
        )
        loadTracks()
        loadRemoteAlbumGroupsPage(0)
        break
      }
      case "playback.changed": {
        // Resolve full track object from local store
        let currentTracks: Track[] = []
        tracks.subscribe(v => { currentTracks = v })()
        let trackObj = currentTracks.find(t => t.id === msg.payload.track_id) ?? null

        // If track not in local store (remote-only), fetch metadata from server
        if (!trackObj && msg.payload.track_id) {
          fetchRemoteTrack(msg.payload.track_id).then(remote => {
            if (remote) {
              playerState.update(s =>
                s.trackId === remote.id ? { ...s, track: remote } : s
              )
            }
          })
        }

        playerState.update(s => {
          // If the client queue contains local file paths the server can't know
          // about, don't let the server overwrite it — the desktop is the
          // authority for mixed local+remote queues.
          const isLocalId = (id: string) => id.startsWith('/') || /^[a-zA-Z]:[/\\]/.test(id)
          const queueHasLocalTracks = s.queue.some(isLocalId)

          return {
            ...s,
            trackId: msg.payload.track_id ?? s.trackId,
            track: trackObj ?? s.track,
            queue: (msg.payload.queue != null && !queueHasLocalTracks) ? msg.payload.queue : s.queue,
            queueIndex: (msg.payload.queue_index != null && !queueHasLocalTracks) ? msg.payload.queue_index : s.queueIndex,
            positionMs: msg.payload.position_ms ?? s.positionMs,
            paused: msg.payload.playing != null ? !msg.payload.playing : s.paused,
            shuffle: msg.payload.shuffle ?? s.shuffle,
            repeat: msg.payload.repeat ?? s.repeat,
          }
        })
        break
      }
    }
  }

  socket.onclose = () => {
    if (intentionalClose) return

    // Unexpected disconnect — show banner but let already-buffered audio
    // continue playing so the user isn't interrupted.

    // Show disconnect banner
    serverDisconnected.set(true)

    // Attempt WS reconnect if still logically connected
    if (get(connected)) {
      reconnectTimer = setTimeout(() => connectWS(), 3000)
    } else {
      // Backend says disconnected — try full re-auth reconnect, then reconnect WS
      autoReconnect(() => connectWS())
    }
  }

  socket.onerror = () => socket?.close()
}

export function disconnectWS() {
  intentionalClose = true
  if (reconnectTimer) clearTimeout(reconnectTimer)
  // Hide disconnect banner on intentional disconnect
  serverDisconnected.set(false)
  socket?.close()
}

/** Fire-and-forget a message to the server over the open WebSocket. */
export function wsSend(type: string, payload: object) {
  if (socket && socket.readyState === WebSocket.OPEN) {
    socket.send(JSON.stringify({ type, payload }))
  }
}

/* ── Helpers ── */

async function fetchRemoteTrack(trackId: string): Promise<Track | null> {
  try {
    const res = await serverFetch(`/api/library/tracks/${trackId}`)
    if (!res.ok) return null
    return (await res.json()) as Track
  } catch {
    return null
  }
}
