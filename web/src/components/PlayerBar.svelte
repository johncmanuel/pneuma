<script lang="ts">
  import { onMount, onDestroy } from "svelte";
  import { playerState } from "../lib/stores/playback";
  import { apiFetch, artworkUrl, getStreamToken, streamUrl } from "../lib/api";
  import { wsSend } from "../lib/ws";
  import {
    formatDuration,
    setupMediaSessionActions,
    setMediaSessionPlaybackState,
    setMediaSessionTrack,
    storageKeys,
    updateMediaSessionMetadata,
    isLocalID,
    RepeatLabels
  } from "@pneuma/shared";
  import {
    Play,
    Pause,
    SkipBack,
    SkipForward,
    Shuffle,
    Repeat,
    VolumeX,
    Volume1,
    Volume2,
    Music,
    List
  } from "@lucide/svelte";
  import { activePanel, toggleQueuePanel } from "../lib/stores/ui";

  let audio: HTMLAudioElement;
  let volume = 1;
  let prevVolume = 1;

  const VOLUME_KEY = storageKeys.volume;

  onMount(() => {
    const saved = parseFloat(localStorage.getItem(VOLUME_KEY) ?? "1");
    volume = isNaN(saved) ? 1 : Math.max(0, Math.min(1, saved));
    prevVolume = volume > 0 ? volume : 1;
    if (audio) audio.volume = volume;

    setupMediaSessionActions({
      onPlay: () => {
        if ($playerState.paused) togglePause();
      },
      onPause: () => {
        if (!$playerState.paused) togglePause();
      },
      onPrev: () => skipPrev(),
      onNext: () => skipNext()
    });
  });

  onDestroy(() => {
    stopPositionLoop();

    if (streamTokenTimer) clearInterval(streamTokenTimer);
    if (seekSyncTimer) clearTimeout(seekSyncTimer);

    setMediaSessionTrack(null);
    setMediaSessionPlaybackState(null);
  });

  let audioDurationMs = 0;

  let seeking = false;
  let seekSyncTimer: ReturnType<typeof setTimeout> | null = null;
  let currentAudioSrc = "";

  let streamToken = "";
  let streamTokenTimer: ReturnType<typeof setTimeout> | null = null;
  let streamRetryCount = 0;

  const MAX_STREAM_RETRIES = 2;

  let lastTrackId = "";
  let lastPaused = true;
  let rafId = 0;
  let lastMediaMetadataKey = "";

  // Position driven directly from audio.currentTime via requestAnimationFrame.
  // Decoupled from the Svelte store to avoid store-update-induced jitter:
  // onTimeUpdate fires every ~250ms which is too coarse for smooth seek bar.
  let displayPosition = 0;

  function startPositionLoop() {
    cancelAnimationFrame(rafId);
    function tick() {
      if (audio && !seeking) {
        displayPosition = audio.currentTime * 1000;
      }
      rafId = requestAnimationFrame(tick);
    }
    rafId = requestAnimationFrame(tick);
  }

  function stopPositionLoop() {
    cancelAnimationFrame(rafId);
  }

  $: track = $playerState.track;
  $: hasTrack = !!$playerState.trackId;

  // define a key from the track metadata to determine when to update Media Session metadata.
  $: mediaMetadataKey = track
    ? `${track.id}|${track.title}|${track.artist_name}|${track.album_artist}|${track.album_name}`
    : "";

  $: if (
    mediaMetadataKey &&
    mediaMetadataKey !== lastMediaMetadataKey &&
    track
  ) {
    lastMediaMetadataKey = mediaMetadataKey;
    setMediaSessionTrack(track);
    updateMediaSessionMetadata(track, artworkUrl);
  }

  $: setMediaSessionPlaybackState(hasTrack ? $playerState.paused : null);

  $: durationMs =
    audioDurationMs > 0 ? audioDurationMs : (track?.duration_ms ?? 0);

  async function refreshToken() {
    streamToken = await getStreamToken();
  }

  const TOKEN_REFRESH_TIMER_MS = 50_000;

  function startTokenRefresh() {
    if (streamTokenTimer) clearInterval(streamTokenTimer);
    streamTokenTimer = setInterval(refreshToken, TOKEN_REFRESH_TIMER_MS);
  }

  // Sync the audio element to the store's playback state.
  // Local/missing tracks are blocked by handlePlaybackChanged in playback.ts
  // before they reach the store, so this only handles remote tracks.
  $: if (audio && $playerState.trackId) {
    const trackChanged = $playerState.trackId !== lastTrackId;
    const pausedChanged = $playerState.paused !== lastPaused;

    if (trackChanged) {
      lastTrackId = $playerState.trackId;
      lastPaused = $playerState.paused;
      streamRetryCount = 0;

      if (seekSyncTimer) {
        clearTimeout(seekSyncTimer);
        seekSyncTimer = null;
      }

      (async () => {
        // Fetch track metadata if the store's track object is stale.
        // Happens when playback.changed arrives from the server (skipNext,
        // auto-advance, other client) which only carries track_id, not the
        // full Track object.
        if ($playerState.track?.id !== $playerState.trackId) {
          try {
            const res = await apiFetch(
              `/api/library/tracks/${$playerState.trackId}`
            );
            if (res.ok) {
              const t = await res.json();
              playerState.update((s) =>
                s.trackId === $playerState.trackId ? { ...s, track: t } : s
              );
            }
          } catch {
            console.error(
              "Failed to fetch track metadata for",
              $playerState.trackId
            );
          }
        }

        if (!streamToken) {
          await refreshToken();
          startTokenRefresh();
        }
        const url = streamUrl($playerState.trackId, streamToken);

        if (url) {
          currentAudioSrc = url;
          audio.src = url;
          audio.currentTime = $playerState.positionMs / 1000;
          displayPosition = $playerState.positionMs;
          if (track) setMediaSessionTrack(track);
        }

        if (!$playerState.paused) {
          audio.play().catch((e) => {
            if (e.name !== "AbortError") {
              console.warn("Audio play failed", e);
            }
          });
          startPositionLoop();
        }
      })();
    } else if (pausedChanged) {
      lastPaused = $playerState.paused;

      if ($playerState.paused && !audio.paused) {
        audio.pause();
        stopPositionLoop();
        displayPosition = audio.currentTime * 1000;
      } else if (!$playerState.paused && audio.paused) {
        audio.play().catch((e) => {
          if (e.name !== "AbortError") {
            console.warn("Audio play failed", e);
          }
        });
        startPositionLoop();
      }
    }
  }

  // When the track is cleared
  $: if (audio && !$playerState.trackId && currentAudioSrc) {
    audio.pause();
    audio.src = "";

    currentAudioSrc = "";
    lastTrackId = "";
    lastPaused = true;
    displayPosition = 0;

    stopPositionLoop();

    if (streamTokenTimer) {
      clearInterval(streamTokenTimer);
      streamTokenTimer = null;
    }

    streamToken = "";
    lastMediaMetadataKey = "";
    setMediaSessionTrack(null);
    setMediaSessionPlaybackState(null);
  }

  function togglePause() {
    if (!hasTrack) return;
    const newPaused = !$playerState.paused;

    playerState.update((s) => ({ ...s, paused: newPaused }));

    wsSend("playback.pause", {
      paused: newPaused,
      position_ms: audio
        ? Math.round(audio.currentTime * 1000)
        : displayPosition
    });
  }

  /** Filter the queue to remote-only tracks for navigation. */
  function remoteQueue(): string[] {
    return $playerState.queue.filter((id) => !isLocalID(id));
  }

  /** Find the current track's position in the filtered queue. */
  function currentRemoteIndex(): number {
    const rq = remoteQueue();
    return rq.indexOf($playerState.trackId);
  }

  function restartCurrentTrack() {
    if (!audio) return;
    audio.currentTime = 0;
    displayPosition = 0;
    audio.play().catch((e) => {
      if (e.name !== "AbortError") {
        console.warn("Audio play failed", e);
      }
    });
    startPositionLoop();
  }

  async function skipNext() {
    if (!hasTrack) return;
    const rq = remoteQueue();
    if (rq.length === 0) return;

    if ($playerState.repeat === 2) {
      restartCurrentTrack();
      playerState.update((s) => ({ ...s, positionMs: 0 }));
      wsSend("playback.play", {
        track_id: $playerState.trackId,
        position_ms: 0
      });
      return;
    }

    const idx = currentRemoteIndex();
    if (idx < 0) return;

    let nextIdx = idx + 1;
    if (nextIdx >= rq.length) {
      if ($playerState.repeat === 1) {
        nextIdx = 0;
      } else {
        playerState.update((s) => ({ ...s, paused: true }));
        return;
      }
    }

    playerState.update((s) => ({
      ...s,
      trackId: rq[nextIdx],
      track: null,
      positionMs: 0,
      paused: false
    }));

    wsSend("playback.play", {
      track_id: rq[nextIdx],
      position_ms: 0
    });
  }

  async function skipPrev() {
    if (!hasTrack) return;
    const rq = remoteQueue();
    if (rq.length === 0) return;

    const idx = currentRemoteIndex();
    if (idx < 0) return;

    const currentTime = audio ? audio.currentTime * 1000 : displayPosition;
    if (currentTime > 3000) {
      restartCurrentTrack();
      playerState.update((s) => ({ ...s, positionMs: 0 }));
      wsSend("playback.play", {
        track_id: $playerState.trackId,
        position_ms: 0
      });
      return;
    }

    let prevIdx = idx - 1;
    if (prevIdx < 0) {
      if ($playerState.repeat === 1) {
        prevIdx = rq.length - 1;
      } else {
        playerState.update((s) => ({ ...s, positionMs: 0 }));
        wsSend("playback.play", {
          track_id: $playerState.trackId,
          position_ms: 0
        });
        return;
      }
    }

    playerState.update((s) => ({
      ...s,
      trackId: rq[prevIdx],
      track: null,
      positionMs: 0,
      paused: false
    }));

    wsSend("playback.play", {
      track_id: rq[prevIdx],
      position_ms: 0
    });
  }

  function toggleShuffle() {
    const newState = !$playerState.shuffle;
    playerState.update((s) => ({ ...s, shuffle: newState }));
    wsSend("playback.shuffle", { enabled: newState });
  }

  function toggleRepeat() {
    const nextMode = (($playerState.repeat + 1) % 3) as 0 | 1 | 2;
    playerState.update((s) => ({ ...s, repeat: nextMode }));
    wsSend("playback.repeat", { mode: nextMode });
  }

  $: repeatLabel = RepeatLabels[$playerState.repeat] ?? "Off";

  function onSeekInput(e: Event) {
    seeking = true;
    const ms = Number((e.target as HTMLInputElement).value);
    displayPosition = ms;
  }

  function onSeekChange(e: Event) {
    seeking = false;
    const ms = Number((e.target as HTMLInputElement).value);
    if (audio) audio.currentTime = ms / 1000;
    displayPosition = ms;
    wsSend("playback.seek", { position_ms: ms });
  }

  function setVolume(e: Event) {
    const target = e.target as HTMLInputElement;
    volume = Number(target.value);
    if (audio) audio.volume = volume;
    if (volume > 0) prevVolume = volume;
    localStorage.setItem(VOLUME_KEY, String(volume));
  }

  function toggleMute() {
    if (!audio) return;
    if (audio.volume > 0) {
      prevVolume = audio.volume;
      audio.volume = 0;
      volume = 0;
    } else {
      volume = prevVolume || 1;
      audio.volume = volume;
    }
  }

  function handleKeyDown(e: KeyboardEvent) {
    const target = e.target as HTMLElement;
    if (
      target.tagName === "INPUT" ||
      target.tagName === "TEXTAREA" ||
      (target as HTMLElement).isContentEditable
    )
      return;

    const ctrl = e.ctrlKey || e.metaKey;
    const isModifierFree = !ctrl && !e.altKey && !e.shiftKey;
    const isOnlyCtrl = ctrl && !e.altKey && !e.shiftKey;

    // Space -> play/pause
    if (e.code === "Space" && isModifierFree) {
      e.preventDefault();
      togglePause();
      return;
    }
    // Ctrl/Cmd+S -> shuffle
    if (isOnlyCtrl && e.key === "s") {
      e.preventDefault();
      toggleShuffle();
      return;
    }
    // Ctrl/Cmd+R -> repeat
    if (isOnlyCtrl && e.key === "r") {
      e.preventDefault();
      toggleRepeat();
      return;
    }
    // Alt+Shift+Q -> toggle queue panel
    if (e.altKey && e.shiftKey && (e.key === "Q" || e.key === "q")) {
      e.preventDefault();
      toggleQueuePanel();
      return;
    }
    // Left arrow -> previous track
    if (e.code === "ArrowLeft" && isModifierFree) {
      e.preventDefault();
      skipPrev();
      return;
    }
    // Right arrow -> next track
    if (e.code === "ArrowRight" && isModifierFree) {
      e.preventDefault();
      skipNext();
      return;
    }
    // M -> mute/unmute
    if ((e.key === "m" || e.key === "M") && isModifierFree) {
      e.preventDefault();
      toggleMute();
      return;
    }
  }

  function onEnded() {
    skipNext();
  }

  function onAudioPlay() {
    setMediaSessionPlaybackState(false);
    if (track) setMediaSessionTrack(track);
  }

  function onAudioPause() {
    setMediaSessionPlaybackState(hasTrack ? true : null);
  }

  async function onAudioError() {
    if (streamRetryCount >= MAX_STREAM_RETRIES) return;

    streamRetryCount++;

    const fresh = await getStreamToken();
    if (!fresh) return;

    streamToken = fresh;
    const trackId = $playerState.trackId;
    if (!trackId || !audio) return;

    const url = streamUrl(trackId, streamToken);
    currentAudioSrc = url;
    audio.src = url;
    audio.currentTime = $playerState.positionMs / 1000;

    if (!$playerState.paused) {
      audio.play().catch((e) => {
        if (e.name !== "AbortError") {
          console.warn("Audio retry play failed", e);
        }
      });
    }
  }

  function onTimeUpdate() {
    // The seek bar is driven by displayPosition from the rAF loop.
    const debounceMs = 5000;
    if (!seekSyncTimer) {
      seekSyncTimer = setTimeout(() => {
        seekSyncTimer = null;
        wsSend("playback.seek", {
          position_ms: audio.currentTime * 1000
        });
      }, debounceMs);
    }
  }

  function changeAudioDuration() {
    if (audio && isFinite(audio.duration)) {
      audioDurationMs = audio.duration * 1000;
    }

    if (track) setMediaSessionTrack(track);
  }
</script>

<svelte:window onkeydown={handleKeyDown} />

<div class="player">
  <audio
    bind:this={audio}
    ontimeupdate={onTimeUpdate}
    onended={onEnded}
    onplay={onAudioPlay}
    onpause={onAudioPause}
    onerror={onAudioError}
    onloadedmetadata={changeAudioDuration}
    ondurationchange={changeAudioDuration}
    preload="metadata"
  ></audio>
  <div class="now-playing">
    <div class="art">
      {#if track}
        <img
          src={artworkUrl(track.id)}
          alt={track.title}
          onerror={(e) => {
            (e.currentTarget as HTMLImageElement).style.display = "none";
          }}
          onload={(e) => {
            (e.currentTarget as HTMLImageElement).style.display = "";
          }}
        />
        <div class="art-placeholder" style="position:absolute">
          <Music size={16} />
        </div>
      {:else}
        <div class="art-placeholder"><Music size={18} /></div>
      {/if}
    </div>
    <div class="info">
      {#if track}
        <span class="title truncate">{track.title}</span>
        <span class="artist truncate text-2"
          >{track.artist_name || track.album_artist || "Unknown Artist"}</span
        >
      {:else}
        <span class="text-3">No track selected</span>
      {/if}
    </div>
  </div>

  <div class="center">
    <div class="controls">
      <button
        class="ctrl-btn"
        class:active-toggle={$playerState.shuffle}
        onclick={toggleShuffle}
        title="Shuffle"><Shuffle size={16} /></button
      >
      <button
        class="ctrl-btn"
        onclick={skipPrev}
        title="Previous"
        disabled={!hasTrack}><SkipBack size={16} /></button
      >
      <button
        class="play-btn"
        onclick={togglePause}
        title={$playerState.paused ? "Play" : "Pause"}
        disabled={!hasTrack}
      >
        {#if $playerState.paused}
          <Play size={16} />
        {:else}
          <Pause size={16} />
        {/if}
      </button>
      <button
        class="ctrl-btn"
        onclick={skipNext}
        title="Next"
        disabled={!hasTrack}><SkipForward size={16} /></button
      >
      <button
        class="ctrl-btn repeat-btn"
        class:active-toggle={$playerState.repeat !== 0}
        onclick={toggleRepeat}
        title="Repeat: {repeatLabel}"
      >
        <Repeat size={16} />{#if $playerState.repeat === 2}<span
            class="repeat-badge">1</span
          >{/if}
      </button>
    </div>
    <div class="seek-row">
      <span class="ts text-3">{formatDuration(displayPosition)}</span>
      <input
        type="range"
        class="seek-bar"
        min="0"
        max={durationMs}
        value={displayPosition}
        oninput={onSeekInput}
        onchange={onSeekChange}
      />
      <span class="ts text-3">{formatDuration(durationMs)}</span>
    </div>
  </div>

  <div class="right-controls">
    <button
      class="ctrl-btn queue-toggle"
      class:active-toggle={$activePanel === "queue"}
      onclick={toggleQueuePanel}
      title="Queue"
    >
      <List size={18} />
    </button>
    <span class="vol-icon"
      >{#if volume === 0}
        <VolumeX size={16} />
      {:else if volume < 0.4}
        <Volume1 size={16} />
      {:else if volume < 0.8}
        <Volume2 size={16} />
      {:else}
        <Volume2 size={16} />
      {/if}</span
    >
    <input
      type="range"
      class="vol-bar"
      min="0"
      max="1"
      step="0.01"
      value={volume}
      oninput={setVolume}
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
  .art-placeholder {
    font-size: 24px;
    color: var(--text-3);
  }

  .info {
    display: flex;
    flex-direction: column;
    min-width: 0;
    gap: 2px;
  }
  .title {
    font-size: 13px;
    font-weight: 600;
  }
  .artist {
    font-size: 12px;
  }

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
  .ctrl-btn:hover {
    color: var(--text-1);
  }
  .active-toggle {
    color: var(--accent) !important;
  }

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
  .play-btn:hover:not(:disabled) {
    transform: scale(1.06);
  }
  .play-btn:disabled {
    opacity: 0.4;
    cursor: not-allowed;
  }

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

  .seek-row {
    display: flex;
    align-items: center;
    gap: 6px;
    width: 100%;
    max-width: 600px;
  }
  .ts {
    font-size: 11px;
    min-width: 34px;
    text-align: center;
  }

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
  .vol-icon {
    font-size: 14px;
    flex-shrink: 0;
  }
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
  .queue-toggle:hover {
    color: var(--text-1);
  }
</style>
