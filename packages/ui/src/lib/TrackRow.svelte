<script lang="ts">
  import { formatDuration, portal } from "@pneuma/shared";
  import "@pneuma/ui/css/components.css";
  import { onDestroy, onMount } from "svelte";
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
    onPlay?: (track: Track | null) => void;
    onSelect?: () => void;
    onAddToQueue?: (track: Track | null) => void;
    onRemove?: (track: Track | null) => void;
    onAddToPlaylist?: (track: Track | null, playlistId: string) => void;
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
    onPlay,
    onSelect,
    onAddToQueue,
    onRemove,
    onAddToPlaylist,
    onToggleFavorite
  }: Props = $props();

  let isDisabled = $derived(disableLocal && isLocal);

  let showMenu = $state(false);
  let menuX = $state(0);
  let menuY = $state(0);
  let showPlaylistSub = $state(false);
  let closeMenuListener: (() => void) | null = $state(null);
  let coarsePointer = $state(false);
  let coarsePointerQuery: MediaQueryList | null = $state(null);
  let coarsePointerHandler: ((event: MediaQueryListEvent) => void) | null =
    $state(null);

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
    onAddToQueue?.(track);
    showMenu = false;
  }

  function handleRemove() {
    onRemove?.(track);
    showMenu = false;
  }

  function handleAddToPlaylist(pl: PlaylistMenuItem) {
    onAddToPlaylist?.(track, pl.id);
    showMenu = false;
    showPlaylistSub = false;
  }

  function handleToggleFavorite() {
    onToggleFavorite?.(track);
    showMenu = false;
  }

  function handleClick() {
    onSelect?.();

    if (!isDisabled && coarsePointer) {
      onPlay?.(track);
    }
  }

  onMount(() => {
    coarsePointerQuery = window.matchMedia("(pointer: coarse)");
    coarsePointer = coarsePointerQuery.matches;

    coarsePointerHandler = (event: MediaQueryListEvent) => {
      coarsePointer = event.matches;
    };

    coarsePointerQuery.addEventListener("change", coarsePointerHandler);
  });

  onDestroy(() => {
    showMenu = false;

    if (coarsePointerQuery && coarsePointerHandler) {
      coarsePointerQuery.removeEventListener("change", coarsePointerHandler);
    }

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
  ondblclick={() => !isDisabled && onPlay?.(track)}
  onclick={handleClick}
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
      {#if onToggleFavorite}
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
</style>
