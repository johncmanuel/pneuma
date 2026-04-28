<script lang="ts">
  import { Music, ChevronRight } from "@lucide/svelte";
  import { portal } from "@pneuma/shared";
  import type { Track } from "@pneuma/shared";

  interface PlaylistMenuItem {
    id: string;
    name: string;
  }

  interface Props {
    track: Track | null | undefined;
    trackArtSrc: string;
    onHideArtworkAndRememberMissing: (e: Event, trackID?: string) => void;
    onResetArtworkVisibility: (e: Event) => void;
    onJumpFromNowPlaying: () => void;
    onAddToQueue?: (track: Track) => void;
    onToggleFavorite?: (track: Track | null) => void;
    isFavorite?: (track: Track) => boolean;
    playlists?: PlaylistMenuItem[];
    onAddToPlaylist?: (track: Track, playlistId: string) => void;
  }

  let {
    track,
    trackArtSrc,
    onHideArtworkAndRememberMissing,
    onResetArtworkVisibility,
    onJumpFromNowPlaying,
    onAddToQueue,
    onToggleFavorite,
    isFavorite = () => false,
    playlists = [],
    onAddToPlaylist
  }: Props = $props();

  let showMenu = $state(false);
  let menuX = $state(0);
  let menuY = $state(0);
  let playlistSub = $state(false);

  function handleContextMenu(e: MouseEvent) {
    if (!track) return;
    e.preventDefault();

    let x = e.clientX;
    let y = e.clientY;

    const menuHeightPx = 200;
    const menuWidthPx = 180;

    // Prevent menu from going out of bounds of viewport
    if (y + menuHeightPx > window.innerHeight) {
      y = window.innerHeight - menuHeightPx - 8;
    }
    if (x + menuWidthPx > window.innerWidth) {
      x = window.innerWidth - menuWidthPx - 8;
    }

    menuX = x;
    menuY = y;

    showMenu = true;
    playlistSub = false;
    const close = () => {
      showMenu = false;
      window.removeEventListener("click", close);
    };
    window.addEventListener("click", close);
  }

  function handleAddToQueue() {
    if (track && onAddToQueue) {
      onAddToQueue(track);
      showMenu = false;
    }
  }

  function handleToggleFavorite() {
    if (track && onToggleFavorite) {
      onToggleFavorite(track);
      showMenu = false;
    }
  }

  function handleAddToPlaylist(playlistId: string) {
    if (track && onAddToPlaylist) {
      onAddToPlaylist(track, playlistId);
      showMenu = false;
    }
  }
</script>

<div class="now-playing" role="presentation" oncontextmenu={handleContextMenu}>
  <div class="art">
    {#if track}
      {#if trackArtSrc}
        <img
          src={trackArtSrc}
          alt={track.title}
          onerror={(e) => onHideArtworkAndRememberMissing(e, track.id)}
          onload={onResetArtworkVisibility}
        />
      {/if}
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
        onclick={onJumpFromNowPlaying}
        title="Go to song source"
      >
        {track.title}
      </button>
      <span class="artist truncate text-2">
        {track.artist_name || track.album_artist || "Unknown Artist"}
      </span>
    {:else}
      <span class="text-3">No track selected</span>
    {/if}
  </div>
</div>

{#if showMenu}
  <div class="ctx-menu" use:portal style="left:{menuX}px;top:{menuY}px">
    {#if onAddToQueue}
      <button onclick={handleAddToQueue}>Add to queue</button>
    {/if}
    {#if onToggleFavorite}
      <button onclick={handleToggleFavorite}
        >{track && isFavorite(track) ? "Unfavorite" : "Favorite"}</button
      >
    {/if}
    {#if playlists.length > 0 && onAddToPlaylist}
      <div
        role="presentation"
        class="ctx-submenu-wrap"
        onmouseenter={() => (playlistSub = true)}
        onmouseleave={() => (playlistSub = false)}
      >
        <button class="has-sub"
          >Add to playlist <ChevronRight size={14} /></button
        >
        {#if playlistSub}
          <div class="ctx-submenu">
            {#each playlists as pl (pl.id)}
              <button onclick={() => handleAddToPlaylist(pl.id)}
                >{pl.name}</button
              >
            {/each}
          </div>
        {/if}
      </div>
    {/if}
  </div>
{/if}

<style>
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

  .ctx-menu {
    position: fixed;
    z-index: 9999;
    background: var(--surface-2);
    border: 1px solid var(--border);
    border-radius: var(--r-md);
    padding: 4px 0;
    box-shadow: var(--shadow-pop);
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
    cursor: pointer;
    background: none;
    border: none;
  }
  .ctx-menu button:hover {
    background: var(--surface-hover);
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
    left: 100%;
    top: 0;
    background: var(--surface-2);
    border: 1px solid var(--border);
    border-radius: var(--r-md);
    padding: 4px 0;
    box-shadow: var(--shadow-pop);
    min-width: 160px;
    max-height: 240px;
    overflow-y: auto;
  }
</style>
