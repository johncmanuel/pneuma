<script lang="ts">
  import { formatDuration } from "@pneuma/shared";

  interface Props {
    displayPosition: number;
    durationMs: number;
    seeking: boolean;
    onSeekInput: (e: Event) => void;
    onSeekChange: (e: Event) => void;
    isMobileSheet?: boolean;
  }

  let {
    displayPosition,
    durationMs,
    seeking = $bindable(),
    onSeekInput,
    onSeekChange,
    isMobileSheet = false
  }: Props = $props();

  function handleInput(e: Event) {
    seeking = true;
    onSeekInput(e);
  }

  function handleChange(e: Event) {
    seeking = false;
    onSeekChange(e);
  }
</script>

<div class={isMobileSheet ? "sheet-seek-row" : "seek-row"}>
  <span class="ts text-3">{formatDuration(displayPosition)}</span>
  <input
    type="range"
    class="seek-bar"
    min="0"
    max={durationMs}
    value={displayPosition}
    oninput={handleInput}
    onchange={handleChange}
  />
  <span class="ts text-3">{formatDuration(durationMs)}</span>
</div>

<style>
  .seek-row {
    display: flex;
    align-items: center;
    gap: 6px;
    width: 100%;
    max-width: 600px;
    min-width: 0;
  }
  .sheet-seek-row {
    display: flex;
    align-items: center;
    gap: 8px;
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
</style>
