<script lang="ts">
  import { createEventDispatcher, onDestroy } from "svelte"
  import type { Track } from "../stores/player"
  import { playlists, addTrackToPlaylist, type PlaylistSummary } from "../stores/playlists"

  export let track: Track | null = null
  export let active: boolean = false
  export let hideAlbum: boolean = false
  export let isLocal: boolean = false

  const dispatch = createEventDispatcher()

  let showMenu = false
  let menuX = 0
  let menuY = 0
  let showPlaylistSub = false

  // Portal action: moves the node to document.body so it is never clipped
  // by any ancestor overflow or contain property.
  function portal(node: HTMLElement) {
    document.body.appendChild(node)
    return {
      destroy() { node.remove() }
    }
  }

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

  function handleAddToPlaylist(pl: PlaylistSummary) {
    if (track) {
      addTrackToPlaylist(pl.id, track, isLocal)
    }
    showMenu = false
    showPlaylistSub = false
  }

  onDestroy(() => { showMenu = false; showPlaylistSub = false })
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
  <div class="ctx-menu" use:portal style="left:{menuX}px;top:{menuY}px">
    <button on:click={handleAddToQueue}>Add to queue</button>
    {#if $playlists.length > 0}
      <!-- svelte-ignore a11y_no_static_element_interactions -->
      <div class="ctx-submenu-wrap"
        on:mouseenter={() => showPlaylistSub = true}
        on:mouseleave={() => showPlaylistSub = false}
      >
        <button class="has-sub">Add to playlist ›</button>
        {#if showPlaylistSub}
          <div class="ctx-submenu">
            {#each $playlists as pl (pl.id)}
              <button on:click={() => handleAddToPlaylist(pl)}>{pl.name}</button>
            {/each}
          </div>
        {/if}
      </div>
    {/if}
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
    z-index: 9999;
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

  .ctx-submenu-wrap { position: relative; }
  .ctx-submenu-wrap .has-sub { cursor: default; }
  .ctx-submenu {
    position: absolute;
    left: 100%;
    top: 0;
    background: var(--surface-2);
    border: 1px solid var(--border);
    border-radius: var(--r-md);
    padding: 4px 0;
    box-shadow: 0 8px 24px rgba(0,0,0,0.5);
    min-width: 160px;
    max-height: 240px;
    overflow-y: auto;
  }
  .ctx-submenu button {
    display: block;
    width: 100%;
    text-align: left;
    padding: 8px 14px;
    font-size: 13px;
    color: var(--text-1);
    border-radius: 0;
  }
  .ctx-submenu button:hover { background: var(--surface-hover); }
</style>
