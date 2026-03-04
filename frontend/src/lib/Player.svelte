<script lang="ts">
  import { playerState, isPlaying, type Track } from "../stores/player"
  import { tracks } from "../stores/library"
  import { activePanel, togglePanel, toggleQueuePanel } from "../stores/ui"
  import { formatDuration } from "./TrackRow.svelte"
  import { apiBase } from "./api"

  let audio: HTMLAudioElement
  let deviceId = "desktop"
  let volume = 1
  let audioDurationMs = 0  // actual duration from <audio> element
  let seeking = false       // true while user is dragging seekbar

  $: track = $playerState.track
  $: hasTrack = !!$playerState.trackId
  // Use the audio element's actual duration as primary source, fall back to metadata
  $: durationMs = audioDurationMs > 0 ? audioDurationMs : (track?.duration_ms ?? 0)

  function api(method: string, path: string, body?: object) {
    return fetch(`${apiBase()}/api/playback/${deviceId}/${path}`, {
      method, headers: { "Content-Type": "application/json" },
      body: body ? JSON.stringify(body) : undefined,
    })
  }

  function findTrackById(id: string): Track | null {
    const all = $tracks as Track[]
    return all?.find(t => t.id === id) ?? null
  }

  async function togglePause() {
    if (!hasTrack) return
    const newPaused = !$playerState.paused
    await api("POST", "pause", { paused: newPaused })
    playerState.update(s => ({ ...s, paused: newPaused }))
  }

  async function skipNext() {
    if (!hasTrack) return
    const res = await api("POST", "next")
    if (!res.ok) return
    const data = await res.json()
    const nextTrack = findTrackById(data.track_id)
    if (data.track_id && data.track_id !== $playerState.trackId) {
      audioDurationMs = 0
      playerState.update(s => ({
        ...s,
        trackId: data.track_id,
        track: nextTrack,
        positionMs: 0,
        paused: false,
        queueIndex: data.queue_index ?? s.queueIndex,
      }))
    } else if (data.track_id === $playerState.trackId) {
      // Same track (repeat-one or end of queue) — restart
      if (audio) { audio.currentTime = 0 }
      playerState.update(s => ({
        ...s,
        positionMs: 0,
        queueIndex: data.queue_index ?? s.queueIndex,
      }))
    }
  }

  async function skipPrev() {
    if (!hasTrack) return
    const res = await api("POST", "prev")
    if (!res.ok) return
    const data = await res.json()
    const prevTrack = findTrackById(data.track_id)
    if (data.track_id && data.track_id !== $playerState.trackId) {
      audioDurationMs = 0
      playerState.update(s => ({
        ...s,
        trackId: data.track_id,
        track: prevTrack,
        positionMs: 0,
        paused: false,
        queueIndex: data.queue_index ?? s.queueIndex,
      }))
    } else if (data.track_id) {
      // Same track — restart from beginning
      if (audio) { audio.currentTime = 0 }
      playerState.update(s => ({
        ...s,
        positionMs: 0,
        queueIndex: data.queue_index ?? s.queueIndex,
      }))
    }
  }

  async function toggleShuffle() {
    const enabled = !$playerState.shuffle
    await api("POST", "shuffle", { enabled })
    // The backend shuffles the queue and publishes via WS — don't manually update queue here
    playerState.update(s => ({ ...s, shuffle: enabled }))
  }

  async function toggleRepeat() {
    const nextMode = (($playerState.repeat + 1) % 3) as 0 | 1 | 2
    await api("POST", "repeat", { mode: nextMode })
    playerState.update(s => ({ ...s, repeat: nextMode }))
  }

  const repeatLabels = ["Off", "All", "One"] as const
  $: repeatLabel = repeatLabels[$playerState.repeat] ?? "Off"

  function onSeekInput(e: Event) {
    seeking = true
    const ms = Number((e.target as HTMLInputElement).value)
    playerState.update(s => ({ ...s, positionMs: ms }))
  }

  async function onSeekChange(e: Event) {
    seeking = false
    const ms = Number((e.target as HTMLInputElement).value)
    if (audio) { audio.currentTime = ms / 1000 }
    await api("POST", "seek", { position_ms: ms })
    playerState.update(s => ({ ...s, positionMs: ms }))
  }

  function setVolume(e: Event) {
    const target = e.target as HTMLInputElement
    volume = Number(target.value)
    if (audio) audio.volume = volume
  }

  // Sync HTML audio element when track changes
  $: if (audio && $playerState.trackId) {
    const url = `${apiBase()}/api/library/tracks/${$playerState.trackId}/stream`
    if (audio.src !== url) {
      audio.src = url
      audio.currentTime = $playerState.positionMs / 1000
    }
    if (!$playerState.paused) audio.play().catch(() => {})
    else audio.pause()
  }

  function onEnded() {
    skipNext()
  }

  function onTimeUpdate() {
    if (!seeking) {
      playerState.update(s => ({ ...s, positionMs: audio.currentTime * 1000 }))
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
          src="{apiBase()}/api/library/tracks/{track.id}/art"
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
        <span class="title truncate">{track.title}</span>
        <span class="artist truncate text-2">{track.album_artist || "Unknown Artist"}</span>
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
