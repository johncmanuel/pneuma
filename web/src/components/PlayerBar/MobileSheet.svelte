<script lang="ts">
  import { onMount, onDestroy } from "svelte";
  import { playerState } from "../../lib/stores/playback";
  import {
    Play,
    Pause,
    SkipBack,
    SkipForward,
    Shuffle,
    Repeat,
    ChevronDown,
    VolumeX,
    Volume1,
    Volume2,
    Music,
    List
  } from "@lucide/svelte";
  import type { Track } from "@pneuma/shared";
  import ProgressBar from "./ProgressBar.svelte";

  interface Props {
    track: Track | null | undefined;
    hasTrack: boolean;
    trackArtSrc: string;
    mobilePlayerExpanded: boolean;
    repeatLabel: string;
    displayPosition: number;
    durationMs: number;
    seeking: boolean;
    volume: number;
    onTogglePause: () => void;
    onSkipPrev: () => void;
    onSkipNext: () => void;
    onToggleShuffle: () => void;
    onToggleRepeat: () => void;
    onHideArtworkAndRememberMissing: (e: Event, trackID?: string) => void;
    onResetArtworkVisibility: (e: Event) => void;
    onQueueToggle: () => void;
    onCloseMobilePlayer: () => void;
    onToggleMobilePlayer: () => void;
    onJumpFromNowPlaying: () => void;
    onSeekInput: (e: Event) => void;
    onSeekChange: (e: Event) => void;
    onSetVolume: (e: Event) => void;
  }

  let {
    track,
    hasTrack,
    trackArtSrc,
    mobilePlayerExpanded = $bindable(),
    repeatLabel,
    displayPosition,
    durationMs,
    seeking = $bindable(),
    volume,
    onTogglePause,
    onSkipPrev,
    onSkipNext,
    onToggleShuffle,
    onToggleRepeat,
    onHideArtworkAndRememberMissing,
    onResetArtworkVisibility,
    onQueueToggle,
    onCloseMobilePlayer,
    onToggleMobilePlayer,
    onJumpFromNowPlaying,
    onSeekInput,
    onSeekChange,
    onSetVolume
  }: Props = $props();

  let sheetDragStartY = $state<number | null>(null);
  let sheetDragOffsetY = $state(0);

  let miniArtistViewport: HTMLSpanElement | null = $state(null);
  let miniArtistMeasureEl: HTMLSpanElement | null = $state(null);
  let miniArtistUseMarquee = $state(false);
  let miniArtistMarqueeDuration = $state(12);
  let miniArtistTextWidth = $state(0);
  let miniArtistMeasureRaf = 0;
  let miniArtistResizeObserver: ResizeObserver | null = null;

  let miniProgressPercent = $derived(
    durationMs > 0
      ? Math.max(0, Math.min(100, (displayPosition / durationMs) * 100))
      : 0
  );

  let miniArtistLabel = $derived(
    track?.artist_name || track?.album_artist || "Unknown Artist"
  );

  function clearMiniArtistMeasureFrame() {
    if (!miniArtistMeasureRaf) return;
    cancelAnimationFrame(miniArtistMeasureRaf);
    miniArtistMeasureRaf = 0;
  }

  function measureMiniArtistOverflow() {
    if (!track || !miniArtistViewport || !miniArtistMeasureEl) {
      miniArtistUseMarquee = false;
      miniArtistTextWidth = 0;
      miniArtistMarqueeDuration = 12;
      return;
    }

    const viewportWidth = miniArtistViewport.clientWidth;
    const textWidth = miniArtistMeasureEl.scrollWidth;

    miniArtistTextWidth = textWidth;
    const overflow = textWidth - viewportWidth;
    miniArtistUseMarquee = overflow > 8;

    if (!miniArtistUseMarquee) {
      miniArtistMarqueeDuration = 12;
      return;
    }

    const gap = 24;
    const speedPxPerSec = 22;
    const travelDistance = textWidth + gap;
    miniArtistMarqueeDuration = Math.max(
      12,
      Math.min(40, travelDistance / speedPxPerSec)
    );
  }

  function scheduleMiniArtistMeasure() {
    clearMiniArtistMeasureFrame();
    miniArtistMeasureRaf = requestAnimationFrame(() => {
      miniArtistMeasureRaf = 0;
      measureMiniArtistOverflow();
    });
  }

  onMount(() => {
    miniArtistResizeObserver = new ResizeObserver(() => {
      scheduleMiniArtistMeasure();
    });

    if (miniArtistViewport) {
      miniArtistResizeObserver.observe(miniArtistViewport);
    }

    if ("fonts" in document) {
      document.fonts.ready.then(() => {
        scheduleMiniArtistMeasure();
      });
    }

    scheduleMiniArtistMeasure();
  });

  onDestroy(() => {
    clearMiniArtistMeasureFrame();
    if (miniArtistResizeObserver) {
      miniArtistResizeObserver.disconnect();
      miniArtistResizeObserver = null;
    }
  });

  $effect(() => {
    miniArtistLabel;
    scheduleMiniArtistMeasure();
  });

  $effect(() => {
    miniArtistViewport;
    if (!miniArtistResizeObserver) return;
    miniArtistResizeObserver.disconnect();
    if (miniArtistViewport) {
      miniArtistResizeObserver.observe(miniArtistViewport);
    }
  });

  function onSheetDragStart(e: TouchEvent) {
    if (!mobilePlayerExpanded) return;
    sheetDragStartY = e.touches[0]?.clientY ?? null;
    sheetDragOffsetY = 0;
  }

  function onSheetDragMove(e: TouchEvent) {
    if (sheetDragStartY === null) return;
    const currentY = e.touches[0]?.clientY ?? sheetDragStartY;
    const deltaY = currentY - sheetDragStartY;
    sheetDragOffsetY = deltaY > 0 ? Math.min(180, deltaY) : 0;
  }

  function finishSheetDrag() {
    if (sheetDragStartY === null) return;
    const shouldClose = sheetDragOffsetY > 88;
    sheetDragStartY = null;
    sheetDragOffsetY = 0;

    if (shouldClose) {
      onCloseMobilePlayer();
    }
  }

  $effect(() => {
    if (!mobilePlayerExpanded) {
      sheetDragStartY = null;
      sheetDragOffsetY = 0;
    }
  });
</script>

<div class="mobile-player-shell">
  <div class="mini-player" class:disabled={!hasTrack}>
    <span class="mini-progress-track" aria-hidden="true">
      <span style="width: {miniProgressPercent}%;"></span>
    </span>

    <button
      class="mini-main"
      onclick={onToggleMobilePlayer}
      aria-label={hasTrack ? "Open now playing" : "No track selected"}
      disabled={!hasTrack}
    >
      <div class="mini-art">
        {#if track}
          {#if trackArtSrc}
            <img
              src={trackArtSrc}
              alt={track.title}
              onerror={(e) => onHideArtworkAndRememberMissing(e, track.id)}
              onload={onResetArtworkVisibility}
            />
          {/if}
          <div class="art-placeholder mini-art-placeholder">
            <Music size={16} />
          </div>
        {:else}
          <div class="art-placeholder mini-art-placeholder">
            <Music size={18} />
          </div>
        {/if}
      </div>

      <div class="mini-info">
        {#if track}
          <span class="mini-title truncate">{track.title}</span>
          <span
            bind:this={miniArtistViewport}
            class="mini-artist text-2"
            class:marquee={miniArtistUseMarquee}
            aria-label={miniArtistLabel}
          >
            {#if miniArtistUseMarquee}
              <span
                class="mini-artist-track"
                style="--mini-artist-duration: {miniArtistMarqueeDuration}s; --mini-artist-width: {miniArtistTextWidth}px;"
              >
                <span>{miniArtistLabel}</span>
                <span aria-hidden="true">{miniArtistLabel}</span>
              </span>
            {:else}
              <span class="mini-artist-static truncate">{miniArtistLabel}</span>
            {/if}

            <span
              bind:this={miniArtistMeasureEl}
              class="mini-artist-measure"
              aria-hidden="true">{miniArtistLabel}</span
            >
          </span>
        {:else}
          <span class="mini-title text-3">No track selected</span>
        {/if}
      </div>
    </button>

    <div class="mini-actions">
      <button
        class="mini-play-btn"
        onclick={onTogglePause}
        title={$playerState.paused ? "Play" : "Pause"}
        aria-label={$playerState.paused ? "Play" : "Pause"}
        disabled={!hasTrack}
      >
        {#if $playerState.paused}
          <Play size={16} />
        {:else}
          <Pause size={16} />
        {/if}
      </button>
    </div>
  </div>

  {#if mobilePlayerExpanded}
    <button
      class="mobile-player-backdrop"
      onclick={onCloseMobilePlayer}
      aria-label="Close now playing"
    ></button>

    <section
      class="mobile-player-sheet"
      class:dragging={sheetDragStartY !== null}
      style="transform: translateY({sheetDragOffsetY}px);"
      aria-label="Now Playing"
    >
      <div
        class="sheet-drag-zone"
        role="presentation"
        ontouchstart={onSheetDragStart}
        ontouchmove={onSheetDragMove}
        ontouchend={finishSheetDrag}
        ontouchcancel={finishSheetDrag}
      >
        <span class="sheet-grabber" aria-hidden="true"></span>
      </div>

      <div class="sheet-top">
        <button
          class="sheet-icon-btn"
          onclick={onCloseMobilePlayer}
          title="Collapse player"
          aria-label="Collapse player"
        >
          <ChevronDown size={22} />
        </button>

        <span class="sheet-label">Now Playing</span>

        <button
          class="sheet-icon-btn"
          onclick={onQueueToggle}
          title="Open queue"
          aria-label="Open queue"
        >
          <List size={19} />
        </button>
      </div>

      <div class="sheet-art-wrap">
        <div class="sheet-art">
          {#if track}
            {#if trackArtSrc}
              <img
                src={trackArtSrc}
                alt={track.title}
                onerror={(e) => onHideArtworkAndRememberMissing(e, track.id)}
                onload={onResetArtworkVisibility}
              />
            {/if}
            <div class="art-placeholder"><Music size={34} /></div>
          {:else}
            <div class="art-placeholder"><Music size={34} /></div>
          {/if}
        </div>
      </div>

      <div class="sheet-meta">
        {#if track}
          <button
            class="sheet-title truncate title-link"
            onclick={onJumpFromNowPlaying}
            title="Go to song source"
          >
            {track.title}
          </button>
          <span class="sheet-artist truncate text-2"
            >{track.artist_name || track.album_artist || "Unknown Artist"}</span
          >
        {:else}
          <span class="sheet-title text-3">No track selected</span>
        {/if}
      </div>

      <ProgressBar
        {displayPosition}
        {durationMs}
        bind:seeking
        {onSeekInput}
        {onSeekChange}
        isMobileSheet={true}
      />

      <div class="sheet-main-controls">
        <button
          class="ctrl-btn"
          class:active-toggle={$playerState.shuffle}
          onclick={onToggleShuffle}
          title="Shuffle"
        >
          <Shuffle size={20} />
        </button>
        <button
          class="ctrl-btn"
          onclick={onSkipPrev}
          title="Previous"
          disabled={!hasTrack}
        >
          <SkipBack size={23} />
        </button>
        <button
          class="play-btn sheet-play-btn"
          onclick={onTogglePause}
          title={$playerState.paused ? "Play" : "Pause"}
          disabled={!hasTrack}
        >
          {#if $playerState.paused}
            <Play size={22} />
          {:else}
            <Pause size={22} />
          {/if}
        </button>
        <button
          class="ctrl-btn"
          onclick={onSkipNext}
          title="Next"
          disabled={!hasTrack}
        >
          <SkipForward size={23} />
        </button>
        <button
          class="ctrl-btn repeat-btn"
          class:active-toggle={$playerState.repeat !== 0}
          onclick={onToggleRepeat}
          title="Repeat: {repeatLabel}"
        >
          <Repeat size={20} />{#if $playerState.repeat === 2}<span
              class="repeat-badge">1</span
            >{/if}
        </button>
      </div>

      <div class="sheet-bottom-row">
        <button class="sheet-pill" onclick={onQueueToggle}>
          <List size={16} />
          Queue
        </button>

        <div class="sheet-volume">
          <span class="vol-icon"
            >{#if volume === 0}
              <VolumeX size={17} />
            {:else if volume < 0.4}
              <Volume1 size={17} />
            {:else}
              <Volume2 size={17} />
            {/if}</span
          >
          <input
            type="range"
            class="vol-bar"
            min="0"
            max="1"
            step="0.01"
            value={volume}
            oninput={onSetVolume}
          />
        </div>
      </div>
    </section>
  {/if}
</div>

<style>
  .mobile-player-shell {
    position: relative;
  }

  .mini-player {
    display: grid;
    grid-template-columns: minmax(0, 1fr) auto;
    align-items: center;
    gap: 8px;
    min-width: 0;
    position: relative;
    border-radius: 14px;
    border: 1px solid var(--border);
    background:
      linear-gradient(
        120deg,
        rgba(55, 171, 134, 0.24),
        rgba(55, 171, 134, 0.02) 52%
      ),
      var(--surface);
    box-shadow: var(--shadow-pop);
    overflow: hidden;
  }

  .mini-player.disabled {
    opacity: 0.75;
  }

  .mini-main {
    width: 100%;
    min-width: 0;
    display: flex;
    align-items: center;
    gap: 10px;
    padding: 8px 6px 12px 8px;
    position: relative;
    text-align: left;
    overflow: hidden;
    background: none;
    border: none;
    color: inherit;
    cursor: pointer;
  }

  .mini-main:disabled {
    opacity: 1;
    cursor: default;
  }

  .mini-progress-track {
    position: absolute;
    left: 0;
    right: 0;
    bottom: 0;
    height: 3px;
    background: rgba(255, 255, 255, 0.16);
    pointer-events: none;
  }

  .mini-progress-track span {
    display: block;
    height: 100%;
    background: var(--accent);
    transition: width 0.15s linear;
  }

  .mini-art {
    width: 42px;
    height: 42px;
    border-radius: 8px;
    overflow: hidden;
    flex-shrink: 0;
    background: var(--surface-2);
    position: relative;
    display: flex;
    align-items: center;
    justify-content: center;
  }

  .mini-art img {
    position: absolute;
    inset: 0;
    width: 100%;
    height: 100%;
    object-fit: cover;
    z-index: 1;
  }

  .mini-art-placeholder {
    font-size: 18px;
  }

  .mini-info {
    flex: 1 1 0;
    width: 0;
    min-width: 0;
    display: flex;
    flex-direction: column;
    gap: 1px;
    overflow: hidden;
  }

  .mini-title {
    display: block;
    width: 100%;
    max-width: 100%;
    font-size: 13px;
    font-weight: 600;
  }

  .mini-artist {
    position: relative;
    display: block;
    width: 100%;
    max-width: 100%;
    min-width: 0;
    font-size: 11px;
    overflow: hidden;
    white-space: nowrap;
  }

  .mini-artist-static {
    display: block;
    width: 100%;
    max-width: 100%;
  }

  .mini-artist-measure {
    position: absolute;
    top: 0;
    left: 0;
    opacity: 0;
    pointer-events: none;
    white-space: nowrap;
    user-select: none;
  }

  .mini-artist-track {
    --mini-artist-gap: 24px;
    display: inline-flex;
    align-items: center;
    gap: var(--mini-artist-gap);
    min-width: max-content;
  }

  .mini-artist.marquee .mini-artist-track {
    animation: mini-artist-marquee var(--mini-artist-duration, 12s) linear
      infinite;
    animation-delay: 2s;
    animation-fill-mode: both;
  }

  .mini-artist.marquee {
    --mini-artist-fade: 14px;
    -webkit-mask-image: linear-gradient(
      to right,
      transparent 0,
      #000 var(--mini-artist-fade),
      #000 calc(100% - var(--mini-artist-fade)),
      transparent 100%
    );
    mask-image: linear-gradient(
      to right,
      transparent 0,
      #000 var(--mini-artist-fade),
      #000 calc(100% - var(--mini-artist-fade)),
      transparent 100%
    );
  }

  @keyframes mini-artist-marquee {
    from {
      transform: translateX(0);
    }
    to {
      transform: translateX(
        calc(-1 * (var(--mini-artist-width, 0px) + var(--mini-artist-gap)))
      );
    }
  }

  .mini-actions {
    display: flex;
    align-items: center;
    justify-content: center;
    gap: 2px;
    flex-shrink: 0;
    min-width: 44px;
    padding-right: 12px;
  }

  .sheet-icon-btn {
    width: 34px;
    height: 34px;
    border-radius: 50%;
    display: inline-flex;
    align-items: center;
    justify-content: center;
    color: var(--text-2);
    background: none;
    border: none;
    cursor: pointer;
    transition:
      background 0.12s,
      color 0.12s;
  }

  .sheet-icon-btn:hover {
    background: var(--surface-hover);
    color: var(--text-1);
  }

  .mini-play-btn {
    width: 34px;
    height: 34px;
    border-radius: 50%;
    display: inline-flex;
    align-items: center;
    justify-content: center;
    background: var(--accent);
    color: var(--on-accent);
    margin-right: 2px;
    border: none;
    cursor: pointer;
  }

  .mini-play-btn:disabled {
    opacity: 0.4;
    cursor: not-allowed;
  }

  .mobile-player-backdrop {
    position: fixed;
    inset: 0;
    border: none;
    background: var(--overlay-strong);
    z-index: 140;
  }

  .mobile-player-sheet {
    position: fixed;
    inset: 0;
    z-index: 145;
    display: flex;
    flex-direction: column;
    gap: 18px;
    padding: max(12px, env(safe-area-inset-top)) 16px
      max(18px, calc(env(safe-area-inset-bottom) + 10px));
    background:
      linear-gradient(
        180deg,
        rgba(33, 108, 86, 0.24),
        rgba(18, 25, 29, 0.04) 44%
      ),
      var(--bg);
    overflow-y: auto;
    transition: transform 0.16s ease;
  }

  .mobile-player-sheet.dragging {
    transition: none;
  }

  .sheet-drag-zone {
    width: 100%;
    display: flex;
    justify-content: center;
    padding: 2px 0 0;
  }

  .sheet-grabber {
    width: 44px;
    height: 5px;
    border-radius: 999px;
    background: rgba(255, 255, 255, 0.28);
  }

  .sheet-top {
    display: flex;
    align-items: center;
    justify-content: space-between;
  }

  .sheet-label {
    font-size: 12px;
    font-weight: 700;
    letter-spacing: 0.08em;
    text-transform: uppercase;
    color: var(--text-2);
  }

  .sheet-art-wrap {
    width: 100%;
    display: flex;
    justify-content: center;
  }

  .sheet-art {
    width: min(78vw, 340px);
    aspect-ratio: 1;
    border-radius: 14px;
    overflow: hidden;
    border: 1px solid var(--border);
    background: var(--surface-2);
    display: flex;
    align-items: center;
    justify-content: center;
    position: relative;
    box-shadow: var(--shadow-pop);
  }

  .sheet-art img {
    position: absolute;
    inset: 0;
    width: 100%;
    height: 100%;
    object-fit: cover;
    z-index: 1;
  }

  .sheet-meta {
    text-align: center;
    min-height: 46px;
    display: flex;
    flex-direction: column;
    justify-content: center;
    gap: 3px;
  }

  .sheet-title {
    font-size: 21px;
    font-weight: 700;
  }

  .sheet-title.title-link {
    text-align: center;
    background: none;
    border: none;
    color: inherit;
    font-family: inherit;
    cursor: pointer;
  }

  .sheet-artist {
    font-size: 14px;
  }

  .sheet-main-controls {
    display: flex;
    align-items: center;
    justify-content: space-between;
    gap: 8px;
    max-width: 360px;
    width: 100%;
    margin: 0 auto;
  }

  .sheet-play-btn {
    width: 54px;
    height: 54px;
  }

  .sheet-bottom-row {
    display: flex;
    align-items: center;
    justify-content: space-between;
    gap: 10px;
  }

  .sheet-pill {
    display: inline-flex;
    align-items: center;
    gap: 6px;
    border: 1px solid var(--border);
    border-radius: 999px;
    padding: 7px 12px;
    color: var(--text-2);
    font-size: 12px;
    font-weight: 600;
    background: none;
    cursor: pointer;
  }

  .sheet-pill:hover {
    background: var(--surface-hover);
    color: var(--text-1);
  }

  .sheet-volume {
    display: flex;
    align-items: center;
    gap: 8px;
    min-width: 0;
    flex: 1;
    justify-content: flex-end;
  }

  .sheet-volume .vol-bar {
    width: min(42vw, 220px);
    flex: 1;
  }

  .ctrl-btn {
    background: none;
    border: none;
    color: var(--text-2);
    cursor: pointer;
  }
  .ctrl-btn:hover {
    color: var(--text-1);
  }
  .active-toggle {
    color: var(--accent) !important;
  }

  .play-btn {
    background: var(--accent);
    color: var(--on-accent);
    border-radius: 50%;
    display: flex;
    align-items: center;
    justify-content: center;
    border: none;
    cursor: pointer;
  }
  .play-btn:disabled {
    opacity: 0.4;
    cursor: not-allowed;
  }

  .repeat-btn {
    position: relative;
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

  @media (max-width: 580px) {
    .mini-play-btn {
      width: 32px;
      height: 32px;
    }

    .sheet-title {
      font-size: 18px;
    }

    .sheet-main-controls {
      max-width: none;
    }

    .sheet-bottom-row {
      flex-direction: column;
      align-items: stretch;
    }

    .sheet-volume {
      justify-content: flex-start;
    }

    .sheet-volume .vol-bar {
      width: 100%;
      max-width: none;
    }
  }
</style>
