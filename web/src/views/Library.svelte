<script lang="ts">
  import { onMount } from "svelte";
  import { createVirtualizer } from "@tanstack/svelte-virtual";
  import { derived } from "svelte/store";
  import {
    albumGroups,
    albumGroupsTotal,
    loading,
    loadAlbumGroupsPage,
    loadMoreAlbumGroups,
    fetchAlbumTracks
  } from "../lib/stores/library";
  import { playerState } from "../lib/stores/playback";
  import { selectedAlbum, pushNav } from "../lib/stores/ui";
  import { artworkUrl } from "../lib/api";
  import { wsSend } from "../lib/ws";
  import { totalDuration } from "../lib/utils";
  import type { Track, AlbumGroup } from "../lib/types";
  import TrackRow from "../components/TrackRow.svelte";
  import { Music, Search, X } from "@lucide/svelte";

  const currentTrackId = derived(playerState, ($s) => $s.trackId);

  let albumFilter = "";
  let trackListEl: HTMLDivElement;
  let albumGridFilter = "";

  let currentAlbumGroup: AlbumGroup | null = null;
  let albumDetailTracks: Track[] = [];
  let albumDetailLoading = false;

  $: hasMore = $albumGroups.length < $albumGroupsTotal;

  // Load the selected album's tracks when the selected album changes
  $: if ($selectedAlbum && !albumDetailLoading) {
    const group = $albumGroups.find((g) => g.key === $selectedAlbum) ?? null;
    if (group && (!currentAlbumGroup || currentAlbumGroup.key !== group.key)) {
      loadAlbumDetail(group);
    }
  }

  // Clear album detail when deselecting
  $: if (!$selectedAlbum) {
    currentAlbumGroup = null;
    albumDetailTracks = [];
    albumFilter = "";
  }

  async function loadAlbumDetail(group: AlbumGroup) {
    albumDetailLoading = true;
    currentAlbumGroup = group;
    albumFilter = "";

    try {
      const tracks = await fetchAlbumTracks(group.name, group.artist);
      albumDetailTracks = tracks;
    } catch (e) {
      console.warn("Failed to load album detail:", e);
      albumDetailTracks = [];
    } finally {
      albumDetailLoading = false;
    }
  }

  // filter the album's tracks based on the albumFilter input
  $: filteredTracks = (() => {
    if (!currentAlbumGroup) return [];

    const f = albumFilter.toLowerCase();
    if (!f) return albumDetailTracks;

    return albumDetailTracks.filter(
      (t) =>
        (t.title ?? "").toLowerCase().includes(f) ||
        (t.artist_name ?? "").toLowerCase().includes(f)
    );
  })();

  $: virtualizer = createVirtualizer<HTMLDivElement, HTMLDivElement>({
    count: filteredTracks.length,
    getScrollElement: () => trackListEl,
    estimateSize: () => 38,
    overscan: 5
  });

  onMount(() => {
    if ($albumGroups.length === 0) {
      loadAlbumGroupsPage(0);
    }
  });

  let gridFilterDebounce: ReturnType<typeof setTimeout>;

  function onAlbumGridFilterInput() {
    clearTimeout(gridFilterDebounce);
    gridFilterDebounce = setTimeout(() => {
      const q = albumGridFilter.trim();
      loadAlbumGroupsPage(0, q);
    }, 300);
  }

  function clearAlbumGridFilter() {
    albumGridFilter = "";
    loadAlbumGroupsPage(0);
  }

  let gridScrollEl: HTMLDivElement;
  let loadingMore = false;

  function handleGridScroll() {
    if (loadingMore || !hasMore || !gridScrollEl) return;
    const { scrollTop, scrollHeight, clientHeight } = gridScrollEl;
    if (scrollTop + clientHeight >= scrollHeight - 200) {
      loadMorePage();
    }
  }

  async function loadMorePage() {
    loadingMore = true;
    try {
      await loadMoreAlbumGroups(albumGridFilter.trim());
    } finally {
      loadingMore = false;
    }
  }

  async function playTrack(track: Track) {
    const idx = albumDetailTracks.findIndex((t) => t.id === track.id);
    const queueIds = albumDetailTracks.map((t) => t.id);

    playerState.update((s) => ({
      ...s,
      trackId: track.id,
      track,
      queue: queueIds,
      queueIndex: idx >= 0 ? idx : 0,
      positionMs: 0,
      paused: false
    }));

    wsSend("playback.queue", {
      track_ids: queueIds,
      start_index: idx >= 0 ? idx : 0
    });
    wsSend("playback.play", {
      track_id: track.id,
      position_ms: 0
    });
  }

  function addToQueue(track: Track) {
    playerState.update((s) => {
      const insertAt = s.queueIndex + 1;
      const newQueue = [
        ...s.queue.slice(0, insertAt),
        track.id,
        ...s.queue.slice(insertAt)
      ];
      return { ...s, queue: newQueue };
    });
  }

  function openAlbum(album: AlbumGroup) {
    pushNav({ view: "library", albumKey: album.key });
  }

  function hideImgOnError(e: Event) {
    const img = e.currentTarget as HTMLImageElement;
    if (img) img.style.display = "none";
  }

  let isScrolling = false;
  let scrollTimer: ReturnType<typeof setTimeout>;

  function handleScroll() {
    isScrolling = true;
    clearTimeout(scrollTimer);
    scrollTimer = setTimeout(() => {
      isScrolling = false;
    }, 150);
  }
</script>

<section>
  <div class="scroll-body">
    {#if currentAlbumGroup}
      <div class="album-detail-view">
        <div class="album-detail-header">
          <div class="album-art-hero">
            {#if currentAlbumGroup.first_track_id}
              <img
                src={artworkUrl(currentAlbumGroup.first_track_id)}
                alt={currentAlbumGroup.name}
                onerror={hideImgOnError}
                loading="lazy"
                decoding="async"
              />
            {/if}
            <div class="album-art-hero-placeholder"><Music size={24} /></div>
          </div>
          <div class="album-detail-info">
            <h2 class="album-detail-title">{currentAlbumGroup.name}</h2>
            <p class="album-meta text-2">
              {currentAlbumGroup.artist} · {albumDetailTracks.length} tracks ·
              {totalDuration(
                albumDetailTracks.reduce(
                  (sum, t) => sum + (t.duration_ms ?? 0),
                  0
                )
              )}
            </p>
            <div class="album-filter-bar">
              <input
                type="search"
                class="album-filter-input"
                placeholder="Filter songs..."
                bind:value={albumFilter}
              />
            </div>
          </div>
        </div>

        <div class="track-headers hide-album">
          <span class="num">#</span>
          <span>Title</span>
          <span>Artist</span>
          <span>Duration</span>
        </div>

        {#if albumFilter && filteredTracks.length === 0}
          <p class="no-results text-3">No songs match "{albumFilter}"</p>
        {:else}
          <div
            class="track-list"
            class:scrolling={isScrolling}
            bind:this={trackListEl}
            onscroll={handleScroll}
          >
            <div
              style="position: relative; width: 100%; height: {$virtualizer.getTotalSize()}px;"
            >
              {#each $virtualizer.getVirtualItems() as row (row.index)}
                <div
                  class="virtual-row"
                  style="height: {row.size}px; transform: translateY({row.start}px);"
                >
                  <TrackRow
                    track={filteredTracks[row.index]}
                    hideAlbum={true}
                    active={$currentTrackId === filteredTracks[row.index]?.id}
                    onplay={(t) => t && playTrack(t)}
                    onselect={() => {}}
                    onaddtoqueue={(t) => t && addToQueue(t)}
                  />
                </div>
              {/each}
            </div>
          </div>
        {/if}
      </div>
    {:else}
      <div
        class="grid-scroll-wrapper"
        bind:this={gridScrollEl}
        onscroll={handleGridScroll}
      >
        <div class="toolbar">
          <h2>Library</h2>
        </div>

        <div class="album-grid-search">
          <Search size={14} />
          <input
            type="search"
            class="album-grid-filter"
            placeholder="Search albums..."
            bind:value={albumGridFilter}
            oninput={onAlbumGridFilterInput}
          />
          {#if albumGridFilter}
            <button class="grid-filter-clear" onclick={clearAlbumGridFilter}
              ><X size={14} /></button
            >
          {/if}
        </div>

        {#if $loading && $albumGroups.length === 0}
          <p class="text-3" style="text-align: center; padding: 24px;">
            Loading...
          </p>
        {:else if $albumGroups.length === 0}
          <p class="text-3">
            No tracks found. Add music to the server and scan.
          </p>
        {:else}
          <div class="album-grid">
            {#each $albumGroups as album (album.key)}
              <button class="album-card" onclick={() => openAlbum(album)}>
                <div class="album-art">
                  <img
                    src={artworkUrl(album.first_track_id)}
                    alt={album.name}
                    onerror={hideImgOnError}
                    loading="lazy"
                  />
                  <div class="album-art-placeholder">
                    <Music size={24} />
                  </div>
                </div>
                <p class="album-title truncate">
                  {album.name}
                </p>
                <p class="album-artist truncate text-3">
                  {album.artist} · {album.track_count} tracks
                </p>
              </button>
            {/each}
          </div>
          {#if hasMore}
            <p class="text-3" style="text-align:center;padding:12px;">
              Loading more...
            </p>
          {/if}
        {/if}
      </div>
    {/if}
  </div>
</section>

<style>
  section {
    display: flex;
    flex-direction: column;
    height: 100%;
    overflow: hidden;
  }

  .scroll-body {
    flex: 1;
    min-height: 0;
    overflow: hidden;
    display: flex;
    flex-direction: column;
    padding: 16px 16px 0 0;
  }

  .album-detail-view {
    flex: 1;
    display: flex;
    flex-direction: column;
    min-height: 0;
    overflow: hidden;
  }

  .grid-scroll-wrapper {
    flex: 1;
    min-height: 0;
    overflow-y: auto;
  }

  .toolbar {
    display: flex;
    align-items: center;
    justify-content: space-between;
    margin-bottom: 16px;
    gap: 12px;
    flex-shrink: 0;
  }

  h2 {
    margin: 0;
    font-size: 20px;
    font-weight: 700;
  }

  .album-grid-search {
    display: flex;
    align-items: center;
    gap: 8px;
    background: var(--surface);
    border: 1px solid var(--border);
    border-radius: 16px;
    padding: 5px 12px;
    margin-bottom: 16px;
    max-width: 280px;
    transition: border-color 0.15s;
  }
  .album-grid-search:focus-within {
    border-color: var(--accent);
  }

  .album-grid-filter {
    flex: 1;
    background: none;
    border: none;
    color: var(--text-1);
    font-size: 13px;
    outline: none;
    padding: 0;
  }
  .album-grid-filter::placeholder {
    color: var(--text-3);
  }
  .album-grid-filter::-webkit-search-cancel-button {
    display: none;
  }

  .grid-filter-clear {
    background: none;
    border: none;
    color: var(--text-3);
    font-size: 16px;
    cursor: pointer;
    padding: 0;
    line-height: 1;
  }
  .grid-filter-clear:hover {
    color: var(--text-1);
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

  .virtual-row {
    position: absolute;
    top: 0;
    left: 0;
    width: 100%;
  }

  .track-list.scrolling .virtual-row {
    pointer-events: none;
  }

  .track-list {
    flex: 1;
    min-height: 0;
    overflow-y: auto;
  }

  .album-detail-header {
    display: flex;
    flex-direction: row;
    align-items: flex-start;
    gap: 20px;
    margin-bottom: 20px;
  }

  .album-detail-info {
    display: flex;
    flex-direction: column;
    justify-content: flex-end;
    flex: 1;
    min-width: 0;
    padding-bottom: 4px;
  }

  .album-art-hero {
    width: 160px;
    height: 160px;
    flex-shrink: 0;
    border-radius: 8px;
    overflow: hidden;
    background: var(--surface);
    display: flex;
    align-items: center;
    justify-content: center;
    position: relative;
  }

  .album-art-hero img {
    width: 100%;
    height: 100%;
    object-fit: cover;
    position: relative;
    z-index: 1;
  }

  .album-art-hero-placeholder {
    position: absolute;
    font-size: 48px;
    color: var(--text-3);
  }

  .album-detail-title {
    margin: 0 0 4px;
    font-size: 20px;
    font-weight: 700;
  }

  .album-meta {
    font-size: 13px;
    margin: 0;
  }

  .album-filter-bar {
    margin-top: 12px;
  }

  .album-filter-input {
    width: 100%;
    max-width: 280px;
    padding: 6px 12px;
    background: var(--surface);
    border: 1px solid var(--border);
    border-radius: 16px;
    color: var(--text-1);
    font-size: 13px;
    outline: none;
    transition: border-color 0.15s;
  }
  .album-filter-input:focus {
    border-color: var(--accent);
  }
  .album-filter-input::placeholder {
    color: var(--text-3);
  }
  .album-filter-input::-webkit-search-cancel-button {
    display: none;
  }

  .no-results {
    padding: 16px 8px;
    font-size: 13px;
  }

  .track-headers {
    display: grid;
    grid-template-columns: 32px 2fr 1fr 76px;
    gap: 0 12px;
    padding: 4px 12px;
    font-size: 11px;
    font-weight: 600;
    text-transform: uppercase;
    letter-spacing: 0.05em;
    color: var(--text-3);
    border-bottom: 1px solid var(--border);
  }

  .track-headers .num {
    text-align: right;
  }
</style>
