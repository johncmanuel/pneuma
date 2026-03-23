<script lang="ts">
  import { onMount } from "svelte";
  import {
    albumGroups,
    loading,
    loadAlbumGroupsPage
  } from "../lib/stores/library";
  import { playerState } from "../lib/stores/playback";
  import { artworkUrl } from "../lib/api";
  import { wsSend } from "../lib/ws";
  import { pushNav } from "../lib/stores/ui";
  import { Music } from "@lucide/svelte";
  import { fetchAlbumTracks } from "../lib/stores/library";

  let { onnavigate }: { onnavigate?: (id: string) => void } = $props();

  onMount(() => {
    loadAlbumGroupsPage(0);
  });

  function openAlbum(album: import("../lib/types").AlbumGroup) {
    pushNav({ view: "library", albumKey: album.key });
  }

  function hideImg(e: Event) {
    const img = e.currentTarget as HTMLImageElement;
    if (img) img.style.display = "none";
  }
</script>

<section>
  <div class="hero">
    <h1>Home</h1>
  </div>

  {#if $loading && $albumGroups.length === 0}
    <p class="text-3" style="text-align: center; padding: 40px;">Loading...</p>
  {:else if $albumGroups.length === 0}
    <p class="text-3" style="text-align: center; padding: 40px;">
      No music yet. Your library will appear here.
    </p>
  {:else}
    <div class="section-header">
      <h2>Recent Albums</h2>
      <button class="see-all" onclick={() => onnavigate?.("library")}>
        See all
      </button>
    </div>

    <div class="album-grid">
      {#each $albumGroups as album (album.key)}
        <button class="album-card" onclick={() => openAlbum(album)}>
          <div class="album-art">
            <img
              src={artworkUrl(album.first_track_id)}
              alt={album.name}
              onerror={hideImg}
              loading="lazy"
            />
            <div class="album-art-placeholder"><Music size={24} /></div>
          </div>
          <p class="album-title truncate">{album.name}</p>
          <p class="album-artist truncate text-3">
            {album.artist} · {album.track_count} tracks
          </p>
        </button>
      {/each}
    </div>
  {/if}
</section>

<style>
  section {
    min-height: 100%;
  }

  .hero {
    margin-bottom: 24px;
  }

  h1 {
    margin: 0;
    font-size: 28px;
    font-weight: 700;
  }

  .section-header {
    display: flex;
    align-items: center;
    justify-content: space-between;
    margin-bottom: 16px;
  }

  h2 {
    margin: 0;
    font-size: 20px;
    font-weight: 700;
  }

  .see-all {
    font-size: 13px;
    color: var(--accent);
    font-weight: 600;
  }

  .see-all:hover {
    text-decoration: underline;
  }

  .album-grid {
    display: grid;
    grid-template-columns: repeat(auto-fill, minmax(150px, 1fr));
    gap: 20px;
  }

  .album-card {
    text-align: left;
    padding: 0;
    cursor: pointer;
  }
  .album-card:hover .album-art {
    border-color: var(--accent);
  }

  .album-art {
    aspect-ratio: 1;
    border-radius: 6px;
    overflow: hidden;
    border: 2px solid transparent;
    background: var(--surface);
    display: flex;
    align-items: center;
    justify-content: center;
    margin-bottom: 8px;
    transition: border-color 0.15s;
    position: relative;
  }

  .album-art img {
    width: 100%;
    height: 100%;
    object-fit: cover;
    position: relative;
    z-index: 1;
  }

  .album-art-placeholder {
    position: absolute;
    font-size: 36px;
    color: var(--text-3);
  }

  .album-title {
    margin: 0;
    font-size: 13px;
    font-weight: 600;
  }
  .album-artist {
    margin: 2px 0 0;
    font-size: 11px;
  }
</style>
