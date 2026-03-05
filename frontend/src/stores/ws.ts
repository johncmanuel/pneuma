import { playerState } from "./player"
import type { Track } from "./player"
import { loadTracks, tracks } from "./library"
import { wsBase, authToken, connected } from "../lib/api"
import { addToast } from "./toasts"
import { get } from "svelte/store"

let socket: WebSocket | null = null
let reconnectTimer: ReturnType<typeof setTimeout> | null = null

export function connectWS() {
  if (!get(connected)) return
  const base = wsBase()
  if (!base) return

  const token = get(authToken)
  const url = token
    ? `${base}/ws?token=${encodeURIComponent(token)}`
    : `${base}/ws`

  socket = new WebSocket(url)

  socket.onmessage = (e) => {
    const msg = JSON.parse(e.data)
    switch (msg.type) {
      case "track.added":
      case "track.updated":
      case "track.removed":
        loadTracks()
        break
      case "library.deduped": {
        const n: number = msg.payload?.removed ?? 0
        addToast(
          `Removed ${n} duplicate song${n !== 1 ? "s" : ""} from your library.`,
          "warning"
        )
        loadTracks()
        break
      }
      case "playback.changed": {
        // Resolve full track object from local store
        let currentTracks: Track[] = []
        tracks.subscribe(v => { currentTracks = v })()
        const trackObj = currentTracks.find(t => t.id === msg.payload.track_id) ?? null
        playerState.update(s => ({
          ...s,
          trackId: msg.payload.track_id ?? s.trackId,
          track: trackObj ?? s.track,
          queue: msg.payload.queue ?? s.queue,
          queueIndex: msg.payload.queue_index ?? s.queueIndex,
          positionMs: msg.payload.position_ms ?? s.positionMs,
          paused: msg.payload.playing != null ? !msg.payload.playing : s.paused,
          shuffle: msg.payload.shuffle ?? s.shuffle,
          repeat: msg.payload.repeat ?? s.repeat,
        }))
        break
      }
    }
  }

  socket.onclose = () => {
    if (get(connected)) {
      reconnectTimer = setTimeout(() => connectWS(), 3000)
    }
  }

  socket.onerror = () => socket?.close()
}

export function disconnectWS() {
  if (reconnectTimer) clearTimeout(reconnectTimer)
  socket?.close()
}
