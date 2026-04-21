<script lang="ts">
  import {
    Play,
    Pause,
    SkipBack,
    SkipForward,
    Shuffle,
    Repeat
  } from "@lucide/svelte";
  import { playerState } from "../../lib/stores/playback";

  interface Props {
    hasTrack: boolean;
    repeatLabel: string;
    onToggleShuffle: () => void;
    onSkipPrev: () => void;
    onTogglePause: () => void;
    onSkipNext: () => void;
    onToggleRepeat: () => void;
  }

  let {
    hasTrack,
    repeatLabel,
    onToggleShuffle,
    onSkipPrev,
    onTogglePause,
    onSkipNext,
    onToggleRepeat
  }: Props = $props();
</script>

<div class="controls">
  <button
    class="ctrl-btn"
    class:active-toggle={$playerState.shuffle}
    onclick={onToggleShuffle}
    title="Shuffle"
  >
    <Shuffle size={16} />
  </button>
  <button
    class="ctrl-btn"
    onclick={onSkipPrev}
    title="Previous"
    disabled={!hasTrack}
  >
    <SkipBack size={16} />
  </button>
  <button
    class="play-btn"
    onclick={onTogglePause}
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
    onclick={onSkipNext}
    title="Next"
    disabled={!hasTrack}
  >
    <SkipForward size={16} />
  </button>
  <button
    class="ctrl-btn repeat-btn"
    class:active-toggle={$playerState.repeat !== 0}
    onclick={onToggleRepeat}
    title="Repeat: {repeatLabel}"
  >
    <Repeat size={16} />
    {#if $playerState.repeat === 2}
      <span class="repeat-badge">1</span>
    {/if}
  </button>
</div>

<style>
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
    color: var(--on-accent);
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
</style>
