<script lang="ts">
  import { formatDuration, portal } from "@pneuma/shared";
  import { onDestroy } from "svelte";
  import { ChevronRight, Heart } from "@lucide/svelte";
  import type { Track } from "@pneuma/shared";

  interface PlaylistMenuItem {
    id: string;
    name: string;
  }

  interface Props {
    track?: Track | null;
    active?: boolean;
    hideAlbum?: boolean;
    dateAdded?: string;
    showRemove?: boolean;
    isLocal?: boolean;
    offline?: boolean;
    disableLocal?: boolean;
    isFavorite?: boolean;
    hideFavoriteIcon?: boolean;
    playlists?: PlaylistMenuItem[];
    onplay?: (track: Track | null) => void;
    onselect?: () => void;
    onaddtoqueue?: (track: Track | null) => void;
    onremove?: (track: Track | null) => void;
    onaddtoplaylist?: (track: Track | null, playlistId: string) => void;
    onToggleFavorite?: (track: Track | null) => void;
  }

  let {
    track = null,
    active = false,
    hideAlbum = false,
    dateAdded = undefined,
    showRemove = false,
    isLocal = false,
    offline = false,
    disableLocal = true,
    isFavorite = false,
    hideFavoriteIcon = false,
    playlists = [],
    onplay,
    onselect,
    onaddtoqueue,
    onremove,
    onaddtoplaylist,
    onToggleFavorite
  }: Props = $props();

  let isDisabled = $derived(disableLocal && isLocal);

  let showMenu = $state(false);
  let menuX = $state(0);
  let menuY = $state(0);
  let showPlaylistSub = $state(false);
  let closeMenuListener: (() => void) | null = $state(null);

  function onContext(e: MouseEvent) {
    e.preventDefault();
    menuX = e.clientX;
    menuY = e.clientY;
    showMenu = true;
    showPlaylistSub = false;

    if (closeMenuListener) {
      window.removeEventListener("click", closeMenuListener);
    }

    closeMenuListener = () => {
      showMenu = false;
      showPlaylistSub = false;
      if (closeMenuListener) {
        window.removeEventListener("click", closeMenuListener);
      }
      closeMenuListener = null;
    };
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

  function handleAddToPlaylist(pl: PlaylistMenuItem) {
    onaddtoplaylist?.(track, pl.id);
    showMenu = false;
    showPlaylistSub = false;
  }

  function handleToggleFavorite() {
    onToggleFavorite?.(track);
    showMenu = false;
  }

  onDestroy(() => {
    showMenu = false;
    if (closeMenuListener) {
      window.removeEventListener("click", closeMenuListener);
    }
  });
</script>

<button
  class="track-row"
  class:active
  class:hide-album={hideAlbum}
  class:offline
  class:local-only={isLocal && disableLocal}
  ondblclick={() => !isDisabled && onplay?.(track)}
  onclick={() => onselect?.()}
  oncontextmenu={onContext}
  disabled={isDisabled}
  title={isDisabled
    ? "Local track — only available on the desktop app"
    : undefined}
>
  <span class="num text-3">{track?.track_number || "-"}</span>
  <span class="title-cell">
    <span class="title truncate">{track?.title ?? "Unknown"}</span>
    <span class="fav-indicator" class:visible={isFavorite && !hideFavoriteIcon}>
      <Heart size={11} fill="currentColor" />
    </span>
  </span>
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
    {#if !isLocal || !disableLocal}
      <button onclick={handleAddToQueue}>Add to queue</button>
      {#if !isLocal && onToggleFavorite}
        <button onclick={handleToggleFavorite}
          >{isFavorite ? "Unfavorite" : "Favorite"}</button
        >
      {/if}
      {#if playlists.length > 0}
        <div
          role="presentation"
          class="ctx-submenu-wrap"
          onmouseenter={() => (showPlaylistSub = true)}
          onmouseleave={() => (showPlaylistSub = false)}
        >
          <button class="has-sub"
            >Add to playlist <ChevronRight size={14} /></button
          >
          {#if showPlaylistSub}
            <div class="ctx-submenu">
              {#each playlists as pl (pl.id)}
                <button onclick={() => handleAddToPlaylist(pl)}
                  >{pl.name}</button
                >
              {/each}
            </div>
          {/if}
        </div>
      {/if}
    {/if}
    {#if showRemove}
      {#if !isLocal || !disableLocal}<hr class="ctx-sep" />{/if}
      <button class="ctx-danger" onclick={handleRemove}
        >Remove from playlist</button
      >
    {/if}
    {#if isLocal && disableLocal && !showRemove}
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

  .track-row.offline,
  .track-row.local-only {
    opacity: 0.4;
    cursor: default;
  }
  .track-row.offline:hover,
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

  .title-cell {
    display: flex;
    align-items: center;
    gap: 5px;
    min-width: 0;
  }
  .title-cell .title {
    min-width: 0;
  }

  .fav-indicator {
    display: flex;
    align-items: center;
    flex-shrink: 0;
    color: var(--accent);
    opacity: 0;
    transition: opacity 0.15s;
  }
  .track-row:hover .fav-indicator.visible {
    opacity: 1;
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

  .ctx-submenu-wrap {
    position: relative;
  }

  .has-sub {
    display: flex !important;
    align-items: center;
    justify-content: space-between;
  }

  .ctx-submenu {
    position: absolute;
    top: -4px;
    left: 100%;
    background: var(--surface-2);
    border: 1px solid var(--border);
    border-radius: var(--r-md);
    padding: 4px 0;
    box-shadow: 0 8px 24px rgba(0, 0, 0, 0.5);
    min-width: 160px;
    max-height: 240px;
    overflow-y: auto;
  }
</style>
