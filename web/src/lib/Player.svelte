<script lang="ts">
  import { webPlayerState, isPlaying } from "./playerStore"
  import { streamUrl, artworkUrl, apiFetch } from "./api"
  import { wsSend, setTrackList } from "./ws"
  import { formatDuration } from "./TrackRow.svelte"
  import type { Track } from "./TrackRow.svelte"

  let audio: HTMLAudioElement
  let deviceId = "web"
  let volume = 1
  let audioDurationMs = 0
  let seeking = false
  let seekSyncTimer: ReturnType<typeof setTimeout> | null = null

  // A resolved list of all tracks for next/prev lookup
  let allTracks: Track[] = []

  // Load tracks list once for queue resolution
  ;(async () => {
    try {
      const r = await apiFetch("/api/library/tracks")
      if (r.ok) {
        allTracks = await r.json()
        setTrackList(allTracks)
      }
    } catch { /* ignore */ }
  })()

  $: track = $webPlayerState.track
  $: hasTrack = !!$webPlayerState.trackId
  $: durationMs = audioDurationMs > 0 ? audioDurationMs : (track?.duration_ms ?? 0)

  function findTrackById(id: string) {
    return allTracks.find((t) => t.id === id) ?? null
  }

  function skipNext() {
    if (!hasTrack) return
    const q = $webPlayerState.queue
    let nextIdx = $webPlayerState.queueIndex + 1
    if (nextIdx >= q.length) nextIdx = 0
    const nextId = q[nextIdx]
    if (!nextId) return
    const nextTrack = findTrackById(nextId)
    audioDurationMs = 0
    webPlayerState.update((s) => ({
      ...s,
      trackId: nextId,
      track: nextTrack,
      queueIndex: nextIdx,
      positionMs: 0,
      paused: false,
    }))
    wsSend("playback.play", { device_id: deviceId, track_id: nextId, position_ms: 0 })
  }

  function skipPrev() {
    if (!hasTrack) return
    const q = $webPlayerState.queue
    let prevIdx = $webPlayerState.queueIndex - 1
    if (prevIdx < 0) prevIdx = q.length - 1
    const prevId = q[prevIdx]
    if (!prevId) return
    const prevTrack = findTrackById(prevId)
    audioDurationMs = 0
    webPlayerState.update((s) => ({
      ...s,
      trackId: prevId,
      track: prevTrack,
      queueIndex: prevIdx,
      positionMs: 0,
      paused: false,
    }))
    wsSend("playback.play", { device_id: deviceId, track_id: prevId, position_ms: 0 })
  }

  function togglePause() {
    if (!hasTrack) return
    const newPaused = !$webPlayerState.paused
    webPlayerState.update((s) => ({ ...s, paused: newPaused }))
    wsSend("playback.pause", { device_id: deviceId, paused: newPaused })
  }

  function onSeekInput(e: Event) {
    seeking = true
    const ms = Number((e.target as HTMLInputElement).value)
    webPlayerState.update((s) => ({ ...s, positionMs: ms }))
  }

  function onSeekChange(e: Event) {
    seeking = false
    const ms = Number((e.target as HTMLInputElement).value)
    if (audio) audio.currentTime = ms / 1000
    webPlayerState.update((s) => ({ ...s, positionMs: ms }))
    wsSend("playback.seek", { device_id: deviceId, position_ms: ms })
  }

  function setVolume(e: Event) {
    volume = Number((e.target as HTMLInputElement).value)
    if (audio) audio.volume = volume
  }

  // Sync audio element when state changes
  $: if (audio && $webPlayerState.trackId) {
    const url = streamUrl($webPlayerState.trackId)
    if (audio.src !== url && url) {
      audio.src = url
      audio.currentTime = $webPlayerState.positionMs / 1000
    }
    if (!$webPlayerState.paused) audio.play().catch(() => {})
    else audio.pause()
  }

  function onEnded() { skipNext() }
  function onTimeUpdate() {
    if (!seeking) {
      webPlayerState.update((s) => ({ ...s, positionMs: audio.currentTime * 1000 }))
    }
    // Debounced position sync to server (every 5 s)
    if (!seekSyncTimer) {
      seekSyncTimer = setTimeout(() => {
        seekSyncTimer = null
        wsSend("playback.seek", { device_id: deviceId, position_ms: audio.currentTime * 1000 })
      }, 5000)
    }
  }
  function onLoadedMetadata() {
    if (audio && isFinite(audio.duration)) audioDurationMs = audio.duration * 1000
  }
  function onDurationChange() {
    if (audio && isFinite(audio.duration)) audioDurationMs = audio.duration * 1000
  }
</script>

<div class="player">
  <audio
    bind:this={audio}
    on:timeupdate={onTimeUpdate}
    on:ended={onEnded}
    on:loadedmetadata={onLoadedMetadata}
    on:durationchange={onDurationChange}
    preload="metadata"
  />

  <!-- Left: album art + track info -->
  <div class="now-playing">
    <div class="art">
      {#if track}
        <img
          src={artworkUrl(track.id)}
          alt=""
          on:error={(e) => { e.currentTarget.style.display = "none" }}
        />
        <div class="art-placeholder" style="position:absolute">♫</div>
      {:else}
        <div class="art-placeholder">♫</div>
      {/if}
    </div>
    <div class="info">
      {#if track}
        <span class="title truncate">{track.title}</span>
        <span class="artist truncate text-2">{track.album_artist || "Unknown Artist"}</span>
      {:else}
        <span class="text-3">No track selected</span>
      {/if}
    </div>
  </div>

  <!-- Center: controls + seekbar -->
  <div class="center">
    <div class="controls">
      <button class="ctrl-btn" on:click={skipPrev} title="Previous" disabled={!hasTrack}>⏮</button>
      <button class="play-btn" on:click={togglePause} title={$webPlayerState.paused ? "Play" : "Pause"} disabled={!hasTrack}>
        {$webPlayerState.paused ? "▶" : "⏸"}
      </button>
      <button class="ctrl-btn" on:click={skipNext} title="Next" disabled={!hasTrack}>⏭</button>
    </div>
    <div class="seek-row">
      <span class="ts text-3">{formatDuration($webPlayerState.positionMs)}</span>
      <input
        type="range"
        class="seek-bar"
        min="0"
        max={durationMs}
        value={$webPlayerState.positionMs}
        on:input={onSeekInput}
        on:change={onSeekChange}
      />
      <span class="ts text-3">{formatDuration(durationMs)}</span>
    </div>
  </div>

  <!-- Right: volume -->
  <div class="right-controls">
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
  .artist { font-size: 12px; }

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
</style>
