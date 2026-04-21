<script lang="ts">
  import { onMount } from "svelte";
  import { playerState } from "../../lib/stores/playback";
  import { wsSend } from "../../lib/ws";
  import {
    markMissingTrackArtID,
    missingTrackArtIDs
  } from "../../lib/stores/missing-art";
  import {
    storageKeys,
    isLocalID,
    shuffle,
    RepeatLabels,
    RepeatModeEnum
  } from "@pneuma/shared";
  import {
    activePanel,
    closePanel,
    toggleQueuePanel
  } from "../../lib/stores/ui";
  import { currentView, pushNav } from "../../lib/stores/ui";
  import {
    favoritesPlaylistId,
    playingPlaylistId
  } from "../../lib/stores/playlists";
  import { streamQuality } from "../../lib/stores/settings";
  import { artworkUrl } from "../../lib/api";
  import { VolumeX, Volume1, Volume2, List } from "@lucide/svelte";

  import AudioEngine from "./AudioEngine.svelte";
  import MediaSession from "./MediaSession.svelte";
  import Controls from "./Controls.svelte";
  import ProgressBar from "./ProgressBar.svelte";
  import TrackInfo from "./TrackInfo.svelte";
  import MobileSheet from "./MobileSheet.svelte";

  const UNORGANIZED_KEY = "__unorganized__";
  const VOLUME_KEY = storageKeys.volume;

  interface Props {
    mobileView?: boolean;
  }
  let { mobileView = false }: Props = $props();

  let audio: HTMLAudioElement = $state() as HTMLAudioElement;
  let volume = $state(1);
  let prevVolume = $state(1);
  let mobilePlayerExpanded = $state(false);

  let displayPosition = $state(0);
  let audioDurationMs = $state(0);
  let seeking = $state(false);

  let track = $derived($playerState.track);
  let hasTrack = $derived(!!$playerState.trackId);
  let trackArtSrc = $derived(
    track && !$missingTrackArtIDs[track.id] ? artworkUrl(track.id) : ""
  );
  let repeatLabel = $derived(RepeatLabels[$playerState.repeat] ?? "Off");
  let durationMs = $derived(
    audioDurationMs > 0 ? audioDurationMs : (track?.duration_ms ?? 0)
  );

  // Load volume from localStorage on mount
  onMount(() => {
    const saved = parseFloat(localStorage.getItem(VOLUME_KEY) ?? "1");
    volume = isNaN(saved) ? 1 : Math.max(0, Math.min(1, saved));
    prevVolume = volume > 0 ? volume : 1;
  });

  // close mobile player when switching to desktop view
  $effect(() => {
    if (!mobileView && mobilePlayerExpanded) {
      mobilePlayerExpanded = false;
    }
  });

  // close mobile player when switching to a different panel
  $effect(() => {
    if (mobileView && $activePanel !== null && mobilePlayerExpanded) {
      mobilePlayerExpanded = false;
    }
  });

  function hideArtworkAndRememberMissing(e: Event, trackID?: string) {
    (e.currentTarget as HTMLImageElement).style.display = "none";
    if (trackID) markMissingTrackArtID(trackID);
  }

  function resetArtworkVisibility(e: Event) {
    (e.currentTarget as HTMLImageElement).style.display = "";
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

  function restartCurrentTrack() {
    if (!audio) return;

    audio.currentTime = 0;
    displayPosition = 0;

    audio.play().catch((e) => {
      if (e.name !== "AbortError") console.warn("Audio play failed", e);
    });
  }

  async function skipNext() {
    if (!hasTrack) return;

    const rq = $playerState.queue.filter((id) => !isLocalID(id));
    if (rq.length === 0) return;

    // if repeat mode is "one", restart the current track
    if ($playerState.repeat === 2) {
      restartCurrentTrack();
      playerState.update((s) => ({ ...s, positionMs: 0 }));
      wsSend("playback.play", {
        track_id: $playerState.trackId,
        position_ms: 0
      });
      return;
    }

    const idx = rq.indexOf($playerState.trackId);
    if (idx < 0) return;

    const nextIdx = idx + 1;

    // if we're at the end of the queue, handle repeat and shuffle
    // by restarting the queue or stopping playback
    if (nextIdx >= rq.length) {
      if ($playerState.repeat === 1) {
        const base = $playerState.baseQueue;
        const source = base.length > 0 ? base : rq;
        let newQueue: string[] = [];

        // if shuffle is on and there's more than one track,
        // pick a random track to be the first track in the new queue.
        // afterwards, shuffle the rest of the tracks and add them to the end.
        if ($playerState.shuffle && source.length > 1) {
          const lastTrack = rq[rq.length - 1];
          const rest = source.filter((id) => id !== lastTrack);

          newQueue = [
            rest[Math.floor(Math.random() * rest.length)],
            ...shuffle(rest)
          ];

          const seen = new Set<string>();
          newQueue = newQueue.filter((id) => {
            if (seen.has(id)) return false;
            seen.add(id);
            return true;
          });

          newQueue = [...newQueue, ...source.filter((id) => !seen.has(id))];
        } else {
          newQueue = [...source];
        }

        playerState.update((s) => ({
          ...s,
          queue: newQueue,
          queueIndex: 0,
          trackId: newQueue[0],
          track: null,
          positionMs: 0,
          paused: false
        }));

        wsSend("playback.queue", { track_ids: newQueue, start_index: 0 });
        wsSend("playback.play", { track_id: newQueue[0], position_ms: 0 });
        return;
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

    wsSend("playback.play", { track_id: rq[nextIdx], position_ms: 0 });
  }

  async function skipPrev() {
    if (!hasTrack) return;

    const rq = $playerState.queue.filter((id) => !isLocalID(id));
    if (rq.length === 0) return;

    const idx = rq.indexOf($playerState.trackId);
    if (idx < 0) return;

    // If the track has been playing for more than 3 seconds, restart it
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
      if ($playerState.repeat === 1) prevIdx = rq.length - 1;
      else {
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

    wsSend("playback.play", { track_id: rq[prevIdx], position_ms: 0 });
  }

  function toggleShuffle() {
    const newState = !$playerState.shuffle;
    playerState.update((s) => ({ ...s, shuffle: newState }));
    wsSend("playback.shuffle", { enabled: newState });
  }

  function toggleRepeat() {
    const nextMode = (($playerState.repeat + 1) % 3) as RepeatModeEnum;
    playerState.update((s) => ({ ...s, repeat: nextMode }));
    wsSend("playback.repeat", { mode: nextMode });
  }

  function toggleMute() {
    if (!audio) return;
    if (audio.volume > 0) {
      prevVolume = audio.volume;
      volume = 0;
    } else {
      volume = prevVolume || 1;
    }
  }

  function onSeekInput(e: Event) {
    seeking = true;
    displayPosition = Number((e.target as HTMLInputElement).value);
  }

  function onSeekChange(e: Event) {
    seeking = false;
    const ms = Number((e.target as HTMLInputElement).value);
    if (audio) audio.currentTime = ms / 1000;
    displayPosition = ms;
    wsSend("playback.seek", { position_ms: ms });
  }

  function setVolume(e: Event) {
    volume = Number((e.target as HTMLInputElement).value);
    if (volume > 0) prevVolume = volume;
    localStorage.setItem(VOLUME_KEY, String(volume));
  }

  function handleKeyDown(e: KeyboardEvent) {
    const target = e.target as HTMLElement;
    if (
      target.tagName === "INPUT" ||
      target.tagName === "TEXTAREA" ||
      target.isContentEditable
    )
      return;

    const ctrl = e.ctrlKey || e.metaKey;
    const isModifierFree = !ctrl && !e.altKey && !e.shiftKey;
    const isOnlyCtrl = ctrl && !e.altKey && !e.shiftKey;

    if (e.key === "Escape" && mobileView && mobilePlayerExpanded) {
      e.preventDefault();
      mobilePlayerExpanded = false;
      return;
    }

    if (e.code === "Space" && isModifierFree) {
      e.preventDefault();
      togglePause();
      return;
    }
    if (isOnlyCtrl && e.key === "s") {
      e.preventDefault();
      toggleShuffle();
      return;
    }
    if (isOnlyCtrl && e.key === "r") {
      e.preventDefault();
      toggleRepeat();
      return;
    }
    if (e.altKey && e.shiftKey && (e.key === "Q" || e.key === "q")) {
      e.preventDefault();
      toggleQueuePanel();
      return;
    }
    if (e.code === "ArrowLeft" && isModifierFree) {
      e.preventDefault();
      skipPrev();
      return;
    }
    if (e.code === "ArrowRight" && isModifierFree) {
      e.preventDefault();
      skipNext();
      return;
    }
    if ((e.key === "m" || e.key === "M") && isModifierFree) {
      e.preventDefault();
      toggleMute();
      return;
    }
  }

  function onQueueToggle() {
    if (mobileView && mobilePlayerExpanded) {
      mobilePlayerExpanded = false;
    }
    if (mobileView && $activePanel === "queue") {
      closePanel();
      return;
    }
    toggleQueuePanel();
  }

  function jumpToAlbum() {
    if (!track) return;
    const albumName = track.album_name?.trim() ?? "";
    const albumArtist = track.album_artist?.trim() ?? "";
    const albumKey =
      albumName && albumName !== UNORGANIZED_KEY
        ? `${albumName}|||${albumArtist}`
        : UNORGANIZED_KEY;
    pushNav({ view: "library", albumKey, playlistId: null });
  }

  function jumpFromNowPlaying() {
    if (!track) return;

    const isPlaylistView =
      $currentView === "playlists" || $currentView === "favorites";

    if (!isPlaylistView && $playingPlaylistId) {
      const targetView =
        $favoritesPlaylistId && $playingPlaylistId === $favoritesPlaylistId
          ? "favorites"
          : "playlists";
      pushNav({
        view: targetView,
        playlistId: $playingPlaylistId,
        albumKey: null
      });
      if (mobileView) mobilePlayerExpanded = false;
      return;
    }
    jumpToAlbum();
    if (mobileView) mobilePlayerExpanded = false;
  }
</script>

<svelte:window onkeydown={handleKeyDown} />

<AudioEngine
  bind:audio
  {volume}
  {mobileView}
  quality={$streamQuality}
  bind:displayPosition
  bind:audioDurationMs
  {seeking}
  onEnded={skipNext}
/>

<MediaSession
  onTogglePause={togglePause}
  onSkipNext={skipNext}
  onSkipPrev={skipPrev}
/>

<div class="player" class:mobile={mobileView}>
  {#if mobileView}
    <MobileSheet
      {track}
      {hasTrack}
      {trackArtSrc}
      bind:mobilePlayerExpanded
      {repeatLabel}
      {displayPosition}
      {durationMs}
      bind:seeking
      {volume}
      onTogglePause={togglePause}
      onSkipPrev={skipPrev}
      onSkipNext={skipNext}
      onToggleShuffle={toggleShuffle}
      onToggleRepeat={toggleRepeat}
      onHideArtworkAndRememberMissing={hideArtworkAndRememberMissing}
      onResetArtworkVisibility={resetArtworkVisibility}
      {onQueueToggle}
      onCloseMobilePlayer={() => (mobilePlayerExpanded = false)}
      onToggleMobilePlayer={() =>
        hasTrack && (mobilePlayerExpanded = !mobilePlayerExpanded)}
      onJumpFromNowPlaying={jumpFromNowPlaying}
      {onSeekInput}
      {onSeekChange}
      onSetVolume={setVolume}
    />
  {:else}
    <TrackInfo
      {track}
      {trackArtSrc}
      onHideArtworkAndRememberMissing={hideArtworkAndRememberMissing}
      onResetArtworkVisibility={resetArtworkVisibility}
      onJumpFromNowPlaying={jumpFromNowPlaying}
    />

    <div class="center">
      <Controls
        {hasTrack}
        {repeatLabel}
        onToggleShuffle={toggleShuffle}
        onSkipPrev={skipPrev}
        onTogglePause={togglePause}
        onSkipNext={skipNext}
        onToggleRepeat={toggleRepeat}
      />
      <ProgressBar
        {displayPosition}
        {durationMs}
        bind:seeking
        {onSeekInput}
        {onSeekChange}
      />
    </div>

    <div class="right-controls">
      <button
        class="ctrl-btn queue-toggle"
        class:active-toggle={$activePanel === "queue"}
        onclick={onQueueToggle}
        title="Queue"
        aria-label="Queue"
      >
        <List size={18} />
      </button>

      <span class="vol-icon">
        {#if volume === 0}
          <VolumeX size={16} />
        {:else if volume < 0.4}
          <Volume1 size={16} />
        {:else}
          <Volume2 size={16} />
        {/if}
      </span>
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
  {/if}
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

  .player.mobile {
    display: block;
    height: auto;
    padding: 0;
    border-top: none;
    background: transparent;
  }

  .center {
    display: flex;
    flex-direction: column;
    align-items: center;
    gap: 2px;
    min-width: 0;
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
    background: none;
    border: none;
    cursor: pointer;
  }

  .queue-toggle:hover {
    color: var(--text-1);
  }

  .active-toggle {
    color: var(--accent) !important;
  }
</style>
