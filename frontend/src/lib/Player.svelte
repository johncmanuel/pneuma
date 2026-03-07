<script lang="ts">
  import { playerState, type Track } from "../stores/player"
  import { fetchTracksByIDs } from "../stores/library"
  import { resolveLocalTracksByPaths } from "../stores/localLibrary"
  import { activePanel, togglePanel, toggleQueuePanel, currentView, pushNav } from "../stores/ui"
  import { formatDuration } from "./TrackRow.svelte"
  import { streamUrl, artworkUrl } from "../utils/api"
  import { wsSend } from "../stores/ws"

  let audio: HTMLAudioElement
  let deviceId = "desktop"
  let volume = 1
  let prevVolume = 1  // last non-zero volume, restored on unmute
  let audioDurationMs = 0  // actual duration from <audio> element
  let seeking = false       // true while user is dragging seekbar
  let seekSyncTimer: ReturnType<typeof setTimeout> | null = null
  // Track the URL we set on audio.src ourselves — do NOT compare against
  // audio.src directly because the browser normalizes percent-encoding
  // (e.g. %27 → ') so the comparison never matches for paths with special
  // characters, causing a continuous src reset that prevents playback.
  let currentAudioSrc = ""

  $: track = $playerState.track
  $: hasTrack = !!$playerState.trackId
  // Use the audio element's actual duration as primary source, fall back to metadata
  $: durationMs = audioDurationMs > 0 ? audioDurationMs : (track?.duration_ms ?? 0)
  // Local tracks use their filesystem path as the ID — don't send WS events for them.
  $: isLocal = !!($playerState.trackId?.startsWith('/') || /^[a-zA-Z]:[/\\]/.test($playerState.trackId ?? ''))

  /** Cache of resolved tracks — avoids re-fetching on every skip. */
  const trackCache = new Map<string, Track>()
  const isLocalPath = (id: string) => id.startsWith('/') || /^[a-zA-Z]:[/\\]/.test(id)

  /** Resolve a track ID to a Track object (server library OR local files). */
  async function findTrackById(id: string): Promise<Track | null> {
    if (trackCache.has(id)) return trackCache.get(id)!

    try {
      if (isLocalPath(id)) {
        const locals = await resolveLocalTracksByPaths([id])
        if (locals.length > 0) {
          const lt = locals[0]
          const t: Track = {
            id: lt.path,
            path: lt.path,
            title: lt.title,
            artist_id: "",
            album_id: "",
            artist_name: lt.artist,
            album_artist: lt.album_artist,
            album_name: lt.album,
            genre: lt.genre,
            year: lt.year,
            track_number: lt.track_number,
            disc_number: lt.disc_number,
            duration_ms: lt.duration_ms,
            bitrate_kbps: 0,
            replay_gain_track: 0,
            artwork_id: "",
          } as Track
          trackCache.set(id, t)
          return t
        }
      } else {
        const remotes = await fetchTracksByIDs([id])
        if (remotes.length > 0) {
          trackCache.set(id, remotes[0])
          return remotes[0]
        }
      }
    } catch {
      // ignore — return null
    }
    return null
  }

  function jumpToAlbum() {
    if (!track) return
    const UNORGANIZED_KEY = "__unorganized__"
    const albumName = track.album_name?.trim() ?? ""
    const albumArtist = track.album_artist?.trim() ?? ""
    const hasAlbum = albumName !== ""
    const albumKey = hasAlbum ? `${albumName}|||${albumArtist}` : UNORGANIZED_KEY
    pushNav({
      view: "library",
      tab: isLocal ? "local" : "library",
      subTab: "albums",
      albumKey,
    })
  }

  function togglePause() {
    if (!hasTrack) return
    const newPaused = !$playerState.paused
    playerState.update(s => ({ ...s, paused: newPaused }))
    if (!isLocal) wsSend("playback.pause", {
      device_id: deviceId,
      paused: newPaused,
      // Include current playhead so the server stores the accurate position
      // and echoes it back in playback.changed (prevents seek-point regression).
      position_ms: audio ? Math.round(audio.currentTime * 1000) : $playerState.positionMs,
    })
  }

  async function skipNext() {
    if (!hasTrack) return
    const q = $playerState.queue
    if (q.length === 0) return

    if ($playerState.repeat === 2) {
      // Repeat-one: restart the current track in-place
      const id = q[$playerState.queueIndex]
      const nextTrack = await findTrackById(id)
      audioDurationMs = 0
      playerState.update(s => ({ ...s, track: nextTrack, positionMs: 0, paused: false }))
      if (!isLocalPath(id)) wsSend("playback.play", { device_id: deviceId, track_id: id, position_ms: 0 })
      return
    }

    // For both repeat-off and repeat-queue: advance to next track. When the
    // queue is exhausted, restore the base queue (original album order) so
    // manually-inserted tracks don't become part of the permanent loop.
    let nextIdx = $playerState.queueIndex + 1
    let nextQueue = q
    if (nextIdx >= q.length) {
      // End of queue — restart from the base queue
      const base = $playerState.baseQueue.length > 0 ? $playerState.baseQueue : q
      nextQueue = base
      nextIdx = 0
    }
    const nextId = nextQueue[nextIdx]
    const nextTrack = await findTrackById(nextId)
    audioDurationMs = 0
    playerState.update(s => ({
      ...s,
      trackId: nextId,
      track: nextTrack,
      queue: nextQueue,
      queueIndex: nextIdx,
      positionMs: 0,
      paused: false,
    }))
    if (!isLocalPath(nextId)) wsSend("playback.play", { device_id: deviceId, track_id: nextId, position_ms: 0 })
  }

  async function skipPrev() {
    if (!hasTrack) return
    const q = $playerState.queue
    if (q.length === 0) return
    let prevIdx = $playerState.queueIndex - 1
    if (prevIdx < 0) prevIdx = q.length - 1
    const prevId = q[prevIdx]
    const prevTrack = await findTrackById(prevId)
    const prevIsLocal = isLocalPath(prevId)
    audioDurationMs = 0
    playerState.update(s => ({
      ...s,
      trackId: prevId,
      track: prevTrack,
      queueIndex: prevIdx,
      positionMs: 0,
      paused: false,
    }))
    if (!prevIsLocal) wsSend("playback.play", { device_id: deviceId, track_id: prevId, position_ms: 0 })
  }

  function toggleShuffle() {
    const enabled = !$playerState.shuffle
    if (isLocal) {
      // Local tracks: shuffle/unshuffle queue client-side (no server involved)
      playerState.update(s => {
        if (enabled && s.queue.length > 1) {
          // Pin current track at index 0, Fisher-Yates shuffle the rest
          const current = s.queue[s.queueIndex]
          const rest = s.queue.filter((_, i) => i !== s.queueIndex)
          for (let i = rest.length - 1; i > 0; i--) {
            const j = Math.floor(Math.random() * (i + 1));
            [rest[i], rest[j]] = [rest[j], rest[i]]
          }
          return { ...s, shuffle: true, queue: [current, ...rest], queueIndex: 0 }
        }
        return { ...s, shuffle: enabled }
      })
      return
    }
    playerState.update(s => ({ ...s, shuffle: enabled }))
    wsSend("playback.shuffle", { device_id: deviceId, enabled })
  }

  function toggleRepeat() {
    const nextMode = (($playerState.repeat + 1) % 3) as 0 | 1 | 2
    playerState.update(s => ({ ...s, repeat: nextMode }))
    if (!isLocal) wsSend("playback.repeat", { device_id: deviceId, mode: nextMode })
  }

  const repeatLabels = ["Off", "All", "One"] as const
  $: repeatLabel = repeatLabels[$playerState.repeat] ?? "Off"

  function onSeekInput(e: Event) {
    seeking = true
    const ms = Number((e.target as HTMLInputElement).value)
    playerState.update(s => ({ ...s, positionMs: ms }))
  }

  function onSeekChange(e: Event) {
    seeking = false
    const ms = Number((e.target as HTMLInputElement).value)
    if (audio) { audio.currentTime = ms / 1000 }
    playerState.update(s => ({ ...s, positionMs: ms }))
    if (!isLocal) wsSend("playback.seek", { device_id: deviceId, position_ms: ms })
  }

  function setVolume(e: Event) {
    const target = e.target as HTMLInputElement
    volume = Number(target.value)
    if (audio) audio.volume = volume
    if (volume > 0) prevVolume = volume
  }

  function toggleMute() {
    if (!audio) return
    if (audio.volume > 0) {
      prevVolume = audio.volume
      audio.volume = 0
      volume = 0
    } else {
      volume = prevVolume || 1
      audio.volume = volume
    }
  }

  // ── Keyboard shortcuts ──────────────────────────────────────────────────────
  // Guards: don't intercept when typing in an input / textarea / contenteditable.
  function handleKeyDown(e: KeyboardEvent) {
    const target = e.target as HTMLElement
    if (
      target.tagName === "INPUT" ||
      target.tagName === "TEXTAREA" ||
      (target as HTMLElement).isContentEditable
    ) return

    const ctrl = e.ctrlKey || e.metaKey // Ctrl on Linux/Win, Cmd on macOS

    // Space → play/pause
    if (e.code === "Space" && !ctrl && !e.altKey && !e.shiftKey) {
      e.preventDefault()
      togglePause()
      return
    }
    // Ctrl/Cmd+S → shuffle
    if (ctrl && e.key === "s" && !e.altKey && !e.shiftKey) {
      e.preventDefault()
      toggleShuffle()
      return
    }
    // Ctrl/Cmd+R → repeat
    if (ctrl && e.key === "r" && !e.altKey && !e.shiftKey) {
      e.preventDefault()
      toggleRepeat()
      return
    }
    // Ctrl/Cmd+, → open settings
    if (ctrl && e.key === "," && !e.altKey && !e.shiftKey) {
      e.preventDefault()
      currentView.set("settings")
      return
    }
    // Alt+Shift+Q → toggle queue panel
    if (e.altKey && e.shiftKey && (e.key === "Q" || e.key === "q")) {
      e.preventDefault()
      toggleQueuePanel()
      return
    }
    // Left arrow → previous track
    if (e.code === "ArrowLeft" && !ctrl && !e.altKey && !e.shiftKey) {
      e.preventDefault()
      skipPrev()
      return
    }
    // Right arrow → next track
    if (e.code === "ArrowRight" && !ctrl && !e.altKey && !e.shiftKey) {
      e.preventDefault()
      skipNext()
      return
    }
    // M → mute/unmute
    if ((e.key === "m" || e.key === "M") && !ctrl && !e.altKey && !e.shiftKey) {
      e.preventDefault()
      toggleMute()
      return
    }
  }

  // Sync HTML audio element when track changes
  $: if (audio && $playerState.trackId) {
    const url = streamUrl($playerState.trackId, $playerState.track?.path)
    if (currentAudioSrc !== url && url) {
      currentAudioSrc = url
      audio.src = url
      audio.currentTime = $playerState.positionMs / 1000
    }
    if (!$playerState.paused && !audio.seeking) audio.play().catch(() => {})
    else if ($playerState.paused) audio.pause()
  }

  function onEnded() {
    skipNext()
  }

  function onTimeUpdate() {
    if (!seeking) {
      playerState.update(s => ({ ...s, positionMs: audio.currentTime * 1000 }))
    }
    // Debounced position sync to server (every 5 s) — local tracks don't need this.
    if (!isLocal && !seekSyncTimer) {
      seekSyncTimer = setTimeout(() => {
        seekSyncTimer = null
        wsSend("playback.seek", { device_id: deviceId, position_ms: audio.currentTime * 1000 })
      }, 5000)
    }
  }

  function onLoadedMetadata() {
    if (audio && isFinite(audio.duration)) {
      audioDurationMs = audio.duration * 1000
    }
  }

  function onDurationChange() {
    if (audio && isFinite(audio.duration)) {
      audioDurationMs = audio.duration * 1000
    }
  }
</script>

<svelte:window on:keydown={handleKeyDown} />

<div class="player">
  <audio
    bind:this={audio}
    on:timeupdate={onTimeUpdate}
    on:ended={onEnded}
    on:loadedmetadata={onLoadedMetadata}
    on:durationchange={onDurationChange}
    preload="metadata"
 ></audio>
  <!-- Left: album art + track info -->
  <div class="now-playing">
    <div class="art">
      {#if track}
        <img
          src="{artworkUrl(track.id)}"
          alt=""
          on:error={(e) => { e.currentTarget.style.display = 'none' }}
        />
        <div class="art-placeholder" style="position:absolute">♫</div>
      {:else}
        <div class="art-placeholder">♫</div>
      {/if}
    </div>
    <div class="info">
      {#if track}
        <button class="title truncate title-link" on:click={jumpToAlbum} title="Go to album">{track.title}</button>
        <span class="artist truncate text-2">{track.artist_name || track.album_artist || "Unknown Artist"}</span>
      {:else}
        <span class="text-3">No track selected</span>
      {/if}
    </div>
  </div>

  <!-- Center: controls + seekbar stacked -->
  <div class="center">
    <div class="controls">
      <button
        class="ctrl-btn"
        class:active-toggle={$playerState.shuffle}
        on:click={toggleShuffle}
        title="Shuffle"
      >⇄</button>
      <button class="ctrl-btn" on:click={skipPrev} title="Previous" disabled={!hasTrack}>⏮</button>
      <button class="play-btn" on:click={togglePause} title={$playerState.paused ? "Play" : "Pause"} disabled={!hasTrack}>
        {$playerState.paused ? "▶" : "⏸"}
      </button>
      <button class="ctrl-btn" on:click={skipNext} title="Next" disabled={!hasTrack}>⏭</button>
      <button
        class="ctrl-btn repeat-btn"
        class:active-toggle={$playerState.repeat !== 0}
        on:click={toggleRepeat}
        title="Repeat: {repeatLabel}"
      >
        🔁{#if $playerState.repeat === 2}<span class="repeat-badge">1</span>{/if}
      </button>
    </div>
    <div class="seek-row">
      <span class="ts text-3">{formatDuration($playerState.positionMs)}</span>
      <input
        type="range"
        class="seek-bar"
        min="0"
        max={durationMs}
        value={$playerState.positionMs}
        on:input={onSeekInput}
        on:change={onSeekChange}
      />
      <span class="ts text-3">{formatDuration(durationMs)}</span>
    </div>
  </div>

  <!-- Right: volume + queue toggle -->
  <div class="right-controls">
    <button
      class="ctrl-btn devices-toggle"
      class:active-toggle={$activePanel === 'devices'}
      on:click={() => togglePanel('devices')}
      title="Devices"
    >
      <svg viewBox="0 0 24 24" width="18" height="18" fill="none" stroke="currentColor" stroke-width="2">
        <rect x="2" y="3" width="20" height="14" rx="2"/>
        <line x1="8" y1="21" x2="16" y2="21"/>
        <line x1="12" y1="17" x2="12" y2="21"/>
      </svg>
    </button>
    <button
      class="ctrl-btn queue-toggle"
      class:active-toggle={$activePanel === 'queue'}
      on:click={toggleQueuePanel}
      title="Queue"
    >
      <svg viewBox="0 0 24 24" width="18" height="18" fill="none" stroke="currentColor" stroke-width="2">
        <line x1="3" y1="6" x2="17" y2="6"/>
        <line x1="3" y1="12" x2="13" y2="12"/>
        <line x1="3" y1="18" x2="9" y2="18"/>
        <polyline points="17 14 21 18 17 22"/>
      </svg>
    </button>
    <span class="vol-icon">{volume === 0 ? "🔇" : volume < 0.4 ? "🔈" : volume < 0.8 ? "🔉" : "🔊"}</span>
    <input
      type="range"
      class="vol-bar"
      min="0"
      max="1"
      step="0.01"
      value={volume}
      on:input={setVolume}
    />
  </div>
</div>

<style>
  .player {
    height: var(--player-h);
    background: var(--surface);
    border-top: 1px solid var(--border);
    display: grid;
    grid-template-columns: minmax(180px, 1fr) 2fr minmax(120px, 1fr);
    align-items: center;
    gap: 0 16px;
    padding: 0 16px;
  }

  /* Left: now playing */
  .now-playing {
    display: flex;
    align-items: center;
    gap: 12px;
    min-width: 0;
  }
  .art {
    width: 56px;
    height: 56px;
    border-radius: 4px;
    overflow: hidden;
    flex-shrink: 0;
    background: var(--surface-2);
    display: flex;
    align-items: center;
    justify-content: center;
    position: relative;
  }
  .art img {
    width: 100%;
    height: 100%;
    object-fit: cover;
    position: relative;
    z-index: 1;
  }
  .art-placeholder { font-size: 24px; color: var(--text-3); }

  .info {
    display: flex;
    flex-direction: column;
    min-width: 0;
    gap: 2px;
  }
  .title { font-size: 13px; font-weight: 600; }
  .title-link {
    cursor: pointer;
    text-align: left;
    padding: 0;
    background: none;
    border: none;
    color: inherit;
    font: inherit;
    font-weight: 600;
  }
  .title-link:hover { text-decoration: underline; }
  .artist { font-size: 12px; }

  /* Center: controls + seek */
  .center {
    display: flex;
    flex-direction: column;
    align-items: center;
    gap: 2px;
  }
  .controls {
    display: flex;
    align-items: center;
    gap: 12px;
  }
  .ctrl-btn {
    font-size: 14px;
    padding: 4px;
    color: var(--text-2);
    transition: color 0.15s;
  }
  .ctrl-btn:hover { color: var(--text-1); }
  .active-toggle { color: var(--accent) !important; }

  .play-btn {
    background: var(--accent);
    color: #000;
    border-radius: 50%;
    width: 34px;
    height: 34px;
    font-size: 14px;
    display: flex;
    align-items: center;
    justify-content: center;
    flex-shrink: 0;
  }
  .play-btn:hover:not(:disabled) { transform: scale(1.06); }
  .play-btn:disabled { opacity: 0.4; cursor: not-allowed; }

  .repeat-btn {
    position: relative;
    font-size: 14px;
  }
  .repeat-badge {
    position: absolute;
    top: -2px;
    right: -4px;
    font-size: 9px;
    font-weight: 700;
    color: var(--accent);
    line-height: 1;
  }

  .devices-toggle {
    margin-right: 2px;
    color: var(--text-2);
  }
  .devices-toggle:hover { color: var(--text-1); }

  .seek-row {
    display: flex;
    align-items: center;
    gap: 6px;
    width: 100%;
    max-width: 600px;
  }
  .ts { font-size: 11px; min-width: 34px; text-align: center; }

  .seek-bar {
    flex: 1;
    accent-color: var(--accent);
    height: 4px;
    padding: 0;
  }

  /* Right: volume */
  .right-controls {
    display: flex;
    align-items: center;
    gap: 6px;
    justify-content: flex-end;
  }
  .vol-icon { font-size: 14px; flex-shrink: 0; }
  .vol-bar {
    width: 100px;
    accent-color: var(--accent);
    height: 4px;
    padding: 0;
  }
  .queue-toggle {
    margin-right: 8px;
    color: var(--text-2);
  }
  .queue-toggle:hover { color: var(--text-1); }
</style>
