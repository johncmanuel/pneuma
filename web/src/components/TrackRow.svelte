<script lang="ts">
  import { formatDuration } from "../lib/utils";
  import { onDestroy } from "svelte";
  import { portal } from "../lib/dom";
  import type { Track } from "../lib/types";

  export let track: Track | null = null;
  export let active: boolean = false;
  export let hideAlbum: boolean = false;
  export let dateAdded: string | undefined = undefined;
  export let showRemove: boolean = false;
  export let isLocal: boolean = false;

  export let onplay: ((track: Track | null) => void) | undefined = undefined;
  export let onselect: (() => void) | undefined = undefined;
  export let onaddtoqueue: ((track: Track | null) => void) | undefined =
    undefined;
  export let onremove: ((track: Track | null) => void) | undefined = undefined;

  let showMenu = false;
  let menuX = 0;
  let menuY = 0;
  let closeMenuListener: (() => void) | null = null;

  function onContext(e: MouseEvent) {
    e.preventDefault();

    menuX = e.clientX;
    menuY = e.clientY;
    showMenu = true;

    if (closeMenuListener)
      window.removeEventListener("click", closeMenuListener);

    closeMenuListener = () => {
      showMenu = false;
      window.removeEventListener("click", closeMenuListener!);
      closeMenuListener = null;
    };
    // defer so this very click doesn't immediately close it
    setTimeout(() => window.addEventListener("click", closeMenuListener!), 0);
  }

  function handleAddToQueue() {
    onaddtoqueue?.(track);
    showMenu = false;
  }

  function handleRemove() {
    onremove?.(track);
    showMenu = false;
  }

  onDestroy(() => {
    showMenu = false;
    if (closeMenuListener)
      window.removeEventListener("click", closeMenuListener);
  });
</script>

<button
  class="track-row"
  class:active
  class:hide-album={hideAlbum}
  class:local-only={isLocal}
  ondblclick={() => !isLocal && onplay?.(track)}
  onclick={() => onselect?.()}
  oncontextmenu={onContext}
  disabled={isLocal}
  title={isLocal
    ? "Local track — only available on the desktop app"
    : undefined}
>
  <span class="num text-3">{track?.track_number || "-"}</span>
  <span class="title truncate">{track?.title ?? "Unknown"}</span>
  <span class="artist truncate text-2"
    >{dateAdded !== undefined
      ? track?.album_name || "-"
      : track?.artist_name || track?.album_artist || "-"}</span
  >
  {#if !hideAlbum}
    <span class="album truncate text-2"
      >{dateAdded !== undefined ? dateAdded : track?.album_name || "-"}</span
    >
  {/if}
  <span class="duration text-3">{formatDuration(track?.duration_ms ?? 0)}</span>
</button>

{#if showMenu}
  <div class="ctx-menu" use:portal style="left:{menuX}px;top:{menuY}px">
    {#if !isLocal}
      <button onclick={handleAddToQueue}>Add to queue</button>
    {/if}
    {#if showRemove}
      {#if !isLocal}<hr class="ctx-sep" />{/if}
      <button class="ctx-danger" onclick={handleRemove}
        >Remove from playlist</button
      >
    {/if}
    {#if isLocal && !showRemove}
      <button disabled class="ctx-disabled">Local track</button>
    {/if}
  </div>
{/if}

<style>
  .track-row {
    display: grid;
    grid-template-columns: 32px 2fr 1fr 1fr 76px;
    align-items: center;
    gap: 0 12px;
    padding: 6px 12px;
    width: 100%;
    text-align: left;
    border-radius: var(--r-sm);
    color: var(--text-1);
    transition:
      background 0.1s,
      color 0.1s;
  }
  .track-row.hide-album {
    grid-template-columns: 32px 2fr 1fr 76px;
  }
  .track-row:hover {
    background: var(--surface-hover);
  }
  .track-row.active {
    color: var(--accent);
  }

  .track-row.local-only {
    opacity: 0.4;
    cursor: default;
  }
  .track-row.local-only:hover {
    opacity: 0.5;
  }

  .num {
    font-size: 12px;
    text-align: right;
  }
  .duration {
    font-size: 12px;
    text-align: right;
  }

  .ctx-menu {
    position: fixed;
    z-index: 9999;
    background: var(--surface-2);
    border: 1px solid var(--border);
    border-radius: var(--r-md);
    padding: 4px 0;
    box-shadow: 0 8px 24px rgba(0, 0, 0, 0.5);
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
  .ctx-menu button:hover {
    background: var(--surface-hover);
  }

  .ctx-sep {
    border: none;
    border-top: 1px solid var(--border);
    margin: 4px 0;
  }
  .ctx-danger {
    color: #e74c3c !important;
  }
  .ctx-danger:hover {
    background: rgba(231, 76, 60, 0.1) !important;
  }
  .ctx-disabled {
    color: var(--text-3) !important;
    cursor: default;
  }
</style>
