<script lang="ts">
  import { playerState } from "../stores/player";
  import { fetchTracksByIDs, UNORGANIZED_KEY } from "../stores/library";
  import { resolveLocalTracksByPaths } from "../stores/localLibrary";
  import {
    activePanel,
    toggleQueuePanel,
    currentView,
    pushNav
  } from "../stores/ui";
  import {
    formatDuration,
    setupMediaSessionActions,
    setMediaSessionPlaybackState,
    setMediaSessionTrack,
    updateMediaSessionMetadata,
    shuffle,
    isRemoteTrack,
    storageKeys,
    type Track,
    isLocalID,
    RepeatModeEnum,
    addToast,
    RepeatLabels
  } from "@pneuma/shared";
  import { streamUrl, artworkUrl, connected } from "../utils/api";
  import { wsSend } from "../stores/ws";
  import { onMount, onDestroy } from "svelte";
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

  let audio: HTMLAudioElement = $state() as HTMLAudioElement;
  let volume = $state(1);
  let prevVolume = $state(1); // last non-zero volume, restored on unmute

  const VOLUME_KEY = storageKeys.volume;

  // load volume upon mounting
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
    setMediaSessionTrack(null);
    setMediaSessionPlaybackState(null);
  });

  let audioDurationMs = $state(0); // actual duration from <audio> element
  let seeking = $state(false); // true while user is dragging seekbar
  let seekSyncTimer: ReturnType<typeof setTimeout> | null = $state(null);

  // Track the URL we set on audio.src; don't compare against
  // audio.src directly because the browser normalizes percent-encoding
  // (e.g. %27 -> ') so the comparison never matches for paths with special
  // characters, causing a continuous src reset that prevents playback.
  let currentAudioSrc = $state("");
  let lastMediaMetadataKey = $state("");

  let track = $derived($playerState.track);
  let hasTrack = $derived(!!$playerState.trackId);
  let durationMs = $derived(
    audioDurationMs > 0 ? audioDurationMs : (track?.duration_ms ?? 0)
  );

  // define a key from the track metadata to determine when to update Media Session metadata.
  let mediaMetadataKey = $derived(
    track
      ? `${track.id}|${track.title}|${track.artist_name}|${track.album_artist}|${track.album_name}`
      : ""
  );

  $effect(() => {
    if (
      mediaMetadataKey &&
      mediaMetadataKey !== lastMediaMetadataKey &&
      track
    ) {
      lastMediaMetadataKey = mediaMetadataKey;
      setMediaSessionTrack(track);
      updateMediaSessionMetadata(track, artworkUrl);
    }
  });

  $effect(() => {
    setMediaSessionPlaybackState(hasTrack ? $playerState.paused : null);
  });

  // Local tracks use their filesystem path as the ID; don't send WS events for them.
  let isLocal = $derived(isLocalID($playerState.trackId ?? ""));

  // Store original queues for restoration when reconnected
  let originalQueue: string[] = $state([]);
  let originalBaseQueue: string[] = $state([]);
  let wasConnected = $state(true);

  // Filter queue when transitioning to disconnected, remove any offline (remote) tracks
  $effect(() => {
    if (wasConnected && !$connected) {
      wasConnected = false;
      const q = $playerState.queue;
      const baseQ = $playerState.baseQueue;

      if (q.length > 0) {
        originalQueue = [...q];
        originalBaseQueue = [...baseQ];

        const isOffline = (id: string) => isRemoteTrack(id);
        const filteredQueue = q.filter((id) => !isOffline(id));
        const filteredBaseQueue = baseQ.filter((id) => !isOffline(id));

        // Adjust queueIndex if current position is now invalid
        let newIndex = $playerState.queueIndex;
        if (newIndex >= filteredQueue.length) {
          newIndex = Math.max(0, filteredQueue.length - 1);
        }

        playerState.update((s) => ({
          ...s,
          queue: filteredQueue,
          baseQueue: filteredBaseQueue,
          queueIndex: newIndex
        }));
      }
    }
  });

  // Restore original queues when reconnected
  $effect(() => {
    if (!wasConnected && $connected) {
      wasConnected = true;
      if (originalQueue.length > 0) {
        playerState.update((s) => ({
          ...s,
          queue: originalQueue,
          baseQueue: originalBaseQueue,
          queueIndex: 0
        }));
        originalQueue = [];
        originalBaseQueue = [];
      }
    }
  });

  const trackCache = new Map<string, Track>();
  const isLocalPath = isLocalID;

  /** Resolve a track ID to a Track object (server library OR local files). */
  async function findTrackById(id: string): Promise<Track | null> {
    if (trackCache.has(id)) return trackCache.get(id)!;

    try {
      if (isLocalPath(id)) {
        const locals = await resolveLocalTracksByPaths([id]);
        if (locals.length > 0) {
          const lt = locals[0];
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
            artwork_id: ""
          } as Track;
          trackCache.set(id, t);
          return t;
        }
      } else {
        const remotes = await fetchTracksByIDs([id]);
        if (remotes.length > 0) {
          trackCache.set(id, remotes[0]);
          return remotes[0];
        }
      }
    } catch {
      console.warn("Failed to find track by ID:", id);
    }
    return null;
  }

  // Navigate to the album of the current track
  function jumpToAlbum() {
    if (!track) return;

    const albumName = track.album_name?.trim() ?? "";
    const albumArtist = track.album_artist?.trim() ?? "";

    const hasAlbum = albumName !== "";
    const albumKey = hasAlbum
      ? `${albumName}|||${albumArtist}`
      : UNORGANIZED_KEY;

    pushNav({
      view: "library",
      tab: isLocal ? "local" : "library",
      subTab: "albums",
      albumKey
    });
  }

  function togglePause() {
    if (!hasTrack) return;
    const newPaused = !$playerState.paused;

    playerState.update((s) => ({ ...s, paused: newPaused }));

    if (!isLocal)
      wsSend("playback.pause", {
        paused: newPaused,
        // Include current playhead so the server stores the accurate position
        // and echoes it back in playback.changed (prevents seek-point regression).
        position_ms: audio
          ? Math.round(audio.currentTime * 1000)
          : $playerState.positionMs
      });
  }

  async function playQueueTrack(
    trackId: string,
    queue: string[],
    queueIndex: number
  ) {
    const track = await findTrackById(trackId);
    audioDurationMs = 0;
    playerState.update((s) => ({
      ...s,
      trackId,
      track,
      queue,
      queueIndex,
      positionMs: 0,
      paused: false
    }));
    if (!isLocalPath(trackId)) {
      wsSend("playback.play", {
        track_id: trackId,
        position_ms: 0
      });
    }
  }

  function getNextAvailableTrack() {
    const q = $playerState.queue;
    const baseQueue = $playerState.baseQueue;

    let nextIdx = $playerState.queueIndex + 1;
    let nextQueue = q;

    const wrapQueueIfNeeded = () => {
      if (nextIdx >= nextQueue.length) {
        nextQueue = baseQueue.length > 0 ? baseQueue : nextQueue;
        nextIdx = 0;
      }
    };

    wrapQueueIfNeeded();

    let nextId = nextQueue[nextIdx];
    let skippedCount = 0;
    const maxSkips = q.length + (baseQueue.length || 0);

    while (nextId && isRemoteTrack(nextId) && !$connected) {
      skippedCount++;
      nextIdx++;
      wrapQueueIfNeeded();
      nextId = nextQueue[nextIdx];

      // If we've looped through all tracks and they're all offline, stop
      if (!nextId || skippedCount >= maxSkips) {
        return null;
      }
    }

    return { nextId, nextIdx, nextQueue, skippedCount };
  }

  async function skipNext() {
    if (!hasTrack) return;

    const q = $playerState.queue;
    if (q.length === 0) return;

    if ($playerState.repeat === RepeatModeEnum.One) {
      await playQueueTrack(
        q[$playerState.queueIndex],
        q,
        $playerState.queueIndex
      );
      return;
    }

    const nextInfo = getNextAvailableTrack();
    if (!nextInfo) {
      playerState.update((s) => ({
        ...s,
        paused: true,
        trackId: "",
        track: null
      }));
      addToast("All tracks are offline", "warning");
      return;
    }

    if (nextInfo.skippedCount > 0) {
      addToast(
        `Skipped ${nextInfo.skippedCount} offline track${nextInfo.skippedCount > 1 ? "s" : ""}`,
        "info"
      );
    }

    await playQueueTrack(nextInfo.nextId, nextInfo.nextQueue, nextInfo.nextIdx);
  }

  async function skipPrev() {
    if (!hasTrack) return;

    const q = $playerState.queue;
    if (q.length === 0) return;

    let prevIdx = $playerState.queueIndex - 1;
    if (prevIdx < 0) prevIdx = q.length - 1;

    await playQueueTrack(q[prevIdx], q, prevIdx);
  }

  function toggleShuffle() {
    const isShuffleEnabled = !$playerState.shuffle;

    // Shuffle/unshuffle is applied client-side for both local and remote tracks
    // so the queue reorders immediately.
    //
    // Send playback.queue (not playback.shuffle) to the server with the already-
    // shuffled queue. playback.shuffle would cause the server to apply its own
    // independent random shuffle and echo back a different order via playback.changed.
    playerState.update((s) => {
      if (isShuffleEnabled && s.queue.length > 1) {
        // Pin current track at index 0, then shuffle the rest
        const current = s.queue[s.queueIndex];
        const rest = s.queue.filter((_, i) => i !== s.queueIndex);
        const shuffledRest = shuffle(rest);

        const newQueue = [current, ...shuffledRest];

        // Send the shuffled queue to the server so it has the correct order.
        // Don't send playback.shuffle since the server would apply its own random
        // shuffle and echo back a different order via playback.changed.
        if (!isLocal) {
          wsSend("playback.queue", {
            track_ids: newQueue,
            start_index: 0
          });
        }

        return {
          ...s,
          shuffle: true,
          queue: newQueue,
          queueIndex: 0
        };
      }
      // Turning shuffle off: restore the original album order from baseQueue
      if (!isShuffleEnabled && s.baseQueue.length > 0) {
        const currentId = s.queue[s.queueIndex];
        const restoredIdx = s.baseQueue.indexOf(currentId);

        // Send the restored queue to the server
        if (!isLocal) {
          wsSend("playback.queue", {
            track_ids: s.baseQueue,
            start_index: restoredIdx >= 0 ? restoredIdx : 0
          });
        }

        return {
          ...s,
          shuffle: false,
          queue: s.baseQueue,
          queueIndex: restoredIdx >= 0 ? restoredIdx : 0
        };
      }
      return { ...s, shuffle: isShuffleEnabled };
    });
  }

  function toggleRepeat() {
    const modes = [RepeatModeEnum.Off, RepeatModeEnum.All, RepeatModeEnum.One];
    const currentIdx = modes.indexOf($playerState.repeat);
    const nextMode = modes[(currentIdx + 1) % modes.length];
    playerState.update((s) => ({ ...s, repeat: nextMode }));

    if (!isLocal) wsSend("playback.repeat", { mode: nextMode });
  }

  let repeatLabel = $derived(RepeatLabels[$playerState.repeat] ?? "Off");

  function onSeekInput(e: Event) {
    seeking = true;
    const ms = Number((e.target as HTMLInputElement).value);
    playerState.update((s) => ({ ...s, positionMs: ms }));
  }

  function onSeekChange(e: Event) {
    seeking = false;
    const ms = Number((e.target as HTMLInputElement).value);

    if (audio) audio.currentTime = ms / 1000;

    playerState.update((s) => ({ ...s, positionMs: ms }));

    if (!isLocal) wsSend("playback.seek", { position_ms: ms });
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
    // don't intercept when typing in an input / textarea / contenteditable.
    const target = e.target as HTMLElement;
    if (
      target.tagName === "INPUT" ||
      target.tagName === "TEXTAREA" ||
      (target as HTMLElement).isContentEditable
    )
      return;

    const ctrl = e.ctrlKey || e.metaKey; // Ctrl on Linux/Win, Cmd on macOS
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
    // Ctrl/Cmd+, -> open settings
    if (isOnlyCtrl && e.key === ",") {
      e.preventDefault();
      currentView.set("settings");
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

  // Sync HTML audio element when track changes
  $effect(() => {
    if (audio && $playerState.trackId) {
      if (seekSyncTimer) {
        clearTimeout(seekSyncTimer);
        seekSyncTimer = null;
      }

      const url = streamUrl($playerState.trackId, $playerState.track?.path);

      if (currentAudioSrc !== url && url) {
        currentAudioSrc = url;
        audio.src = url;
        audio.currentTime = $playerState.positionMs / 1000;
        if (track) setMediaSessionTrack(track);
      }

      if (!$playerState.paused && !audio.seeking && audio.paused) {
        audio.play().catch((e) => {
          if (e.name !== "AbortError") {
            console.warn("Audio play failed", e);
          }
        });
      } else if ($playerState.paused && !audio.paused) {
        audio.pause();
      }
    }
  });

  // When the track is forcefully cleared (e.g. file removed from disk),
  // stop and reset the audio element immediately. Maybe add a toast
  // saying the file is no longer available.
  $effect(() => {
    if (audio && !$playerState.trackId && currentAudioSrc) {
      audio.pause();
      audio.src = "";

      currentAudioSrc = "";
      lastMediaMetadataKey = "";

      setMediaSessionTrack(null);
      setMediaSessionPlaybackState(null);
    }
  });

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

  function onAudioError(event: Event) {
    const target = event.currentTarget as HTMLAudioElement;
    const err = target.error;

    console.error(
      `[Audio]: ${err?.message ?? "no message"}, error code: ${err?.code ?? "unknown"}, src: ${target.src}`
    );
  }

  function onTimeUpdate() {
    if (!seeking) {
      playerState.update((s) => ({
        ...s,
        positionMs: audio.currentTime * 1000
      }));
    }

    // Debounced position sync to server (every 5s) for remote tracks
    // Local tracks don't need this.
    const debounceMs = 5000;
    if (!isLocal && !seekSyncTimer) {
      seekSyncTimer = setTimeout(() => {
        seekSyncTimer = null;
        if (!isLocalID($playerState.trackId ?? "")) {
          wsSend("playback.seek", {
            position_ms: audio.currentTime * 1000
          });
        }
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
    onloadedmetadata={changeAudioDuration}
    ondurationchange={changeAudioDuration}
    onerror={onAudioError}
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
        <button
          class="title truncate title-link"
          onclick={jumpToAlbum}
          title="Go to album">{track.title}</button
        >
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
        class:active-toggle={$playerState.repeat !== RepeatModeEnum.Off}
        onclick={toggleRepeat}
        title="Repeat: {repeatLabel}"
      >
        <Repeat
          size={16}
        />{#if $playerState.repeat === RepeatModeEnum.One}<span
            class="repeat-badge">1</span
          >{/if}
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
  .title-link:hover {
    text-decoration: underline;
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
