<script lang="ts">
  import { createEventDispatcher } from "svelte"
  import type { Track } from "../stores/player"

  export let track: Track | null = null
  export let active: boolean = false
  export let hideAlbum: boolean = false

  const dispatch = createEventDispatcher()

  let showMenu = false
  let menuX = 0
  let menuY = 0

  function onContext(e: MouseEvent) {
    e.preventDefault()
    menuX = e.clientX
    menuY = e.clientY
    showMenu = true
    const close = () => { showMenu = false; window.removeEventListener("click", close) }
    window.addEventListener("click", close)
  }

  function handleAddToQueue() {
    dispatch("addToQueue", track)
    showMenu = false
  }
</script>

<button
  class="track-row"
  class:active
  class:hide-album={hideAlbum}
  on:dblclick={() => dispatch("play", track)}
  on:click={() => dispatch("select")}
  on:contextmenu={onContext}
>
  <span class="num text-3">{track?.track_number || "-"}</span>
  <span class="title truncate">{track?.title ?? "Unknown"}</span>
  <span class="artist truncate text-2">{track?.artist_name || track?.album_artist || "-"}</span>
  {#if !hideAlbum}
    <span class="album truncate text-2">{track?.album_name || "-"}</span>
  {/if}
  <span class="duration text-3">{formatDuration(track?.duration_ms ?? 0)}</span>
</button>

{#if showMenu}
  <div class="ctx-menu" style="left:{menuX}px;top:{menuY}px">
    <button on:click={handleAddToQueue}>Add to queue</button>
  </div>
{/if}

<script lang="ts" context="module">
  export function formatDuration(ms: number): string {
    const s = Math.floor(ms / 1000)
    const m = Math.floor(s / 60)
    return `${m}:${String(s % 60).padStart(2, "0")}`
  }
</script>

<style>
  .track-row {
    display: grid;
    grid-template-columns: 32px 2fr 1fr 1fr 56px;
    align-items: center;
    gap: 0 12px;
    padding: 6px 12px;
    width: 100%;
    text-align: left;
    border-radius: var(--r-sm);
    color: var(--text-1);
    transition: background 0.1s;
  }
  .track-row.hide-album {
    grid-template-columns: 32px 2fr 1fr 56px;
  }
  .track-row:hover { background: var(--surface-hover); }
  .track-row.active { color: var(--accent); }
  .num { font-size: 12px; text-align: right; }
  .duration { font-size: 12px; text-align: right; }

  .ctx-menu {
    position: fixed;
    z-index: 999;
    background: var(--surface-2);
    border: 1px solid var(--border);
    border-radius: var(--r-md);
    padding: 4px 0;
    box-shadow: 0 8px 24px rgba(0,0,0,0.5);
    min-width: 160px;
  }
  .ctx-menu button {
    display: block;
    width: 100%;
    text-align: left;
    padding: 8px 14px;
    font-size: 13px;
    color: var(--text-1);
    border-radius: 0;
  }
  .ctx-menu button:hover { background: var(--surface-hover); }
</style>
