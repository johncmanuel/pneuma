<script lang="ts">
  import { onMount } from "svelte";
  import { createVirtualizer } from "@tanstack/svelte-virtual";
  import { derived, get } from "svelte/store";
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
  import {
    visiblePlaylistsForAddMenu,
    handleAddToPlaylist,
    handleAddTracksToPlaylist,
    playlists as playlistsStore,
    toggleFavoriteTrack,
    setPlayingPlaylistContext
  } from "../lib/stores/playlists";
  import { artworkUrl } from "../lib/api";
  import { wsSend } from "../lib/ws";
  import { recordRecentAlbum } from "../lib/stores/recent";
  import {
    portal,
    totalDuration,
    shuffle,
    type Track,
    type AlbumGroup
  } from "@pneuma/shared";
  import {
    markMissingTrackArtID,
    missingTrackArtIDs
  } from "../lib/stores/missing-art";
  import TrackRow from "../components/TrackRow.svelte";
  import { SortButton } from "@pneuma/ui";
  import { ChevronRight, Music, Search, X } from "@lucide/svelte";

  type SortField = "default" | "title" | "artist" | "duration";

  const currentTrackId = derived(playerState, ($s) => $s.trackId);

  let albumFilter = $state("");
  let albumSortField: SortField = $state("default");
  let albumSortDir: "asc" | "desc" = $state("asc");

  let trackListEl: HTMLDivElement | undefined = $state();
  let albumGridFilter = $state("");

  let currentAlbumGroup: AlbumGroup | null = $state(null);
  let albumDetailTracks: Track[] = $state([]);
  let albumDetailLoading = $state(false);

  let hasMore = $derived($albumGroups.length < $albumGroupsTotal);

  // Load the selected album's tracks when the selected album changes
  $effect(() => {
    if ($selectedAlbum && !albumDetailLoading) {
      const group = $albumGroups.find((g) => g.key === $selectedAlbum) ?? null;
      if (
        group &&
        (!currentAlbumGroup || currentAlbumGroup.key !== group.key)
      ) {
        loadAlbumDetail(group);
      }
    }
  });

  // Clear album detail when deselecting
  $effect(() => {
    if (!$selectedAlbum) {
      currentAlbumGroup = null;
      albumDetailTracks = [];
      albumFilter = "";
    }
  });

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
  let filteredTracks = $derived(
    (() => {
      if (!currentAlbumGroup) return [];

      let list = albumDetailTracks;
      const f = albumFilter.toLowerCase();
      if (f) {
        list = list.filter(
          (t) =>
            (t.title ?? "").toLowerCase().includes(f) ||
            (t.artist_name ?? "").toLowerCase().includes(f)
        );
      }

      return list.slice().sort((a, b) => {
        if (albumSortField === "default")
          return (
            (a.disc_number ?? 0) - (b.disc_number ?? 0) ||
            (a.track_number || 0) - (b.track_number || 0)
          );

        let cmp = 0;
        if (albumSortField === "title")
          cmp = (a.title || "").localeCompare(b.title || "");
        else if (albumSortField === "artist")
          cmp = (a.artist_name || "").localeCompare(b.artist_name || "");
        else if (albumSortField === "duration")
          cmp = (a.duration_ms || 0) - (b.duration_ms || 0);

        return albumSortDir === "desc" ? -cmp : cmp;
      });
    })()
  );

  let virtualizer = $derived(
    createVirtualizer<HTMLDivElement, HTMLDivElement>({
      count: filteredTracks.length,
      getScrollElement: () => trackListEl as HTMLDivElement,
      estimateSize: () => 38,
      overscan: 5
    })
  );

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

  let gridScrollEl: HTMLDivElement | undefined = $state();
  let loadingMore = $state(false);

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

    if (currentAlbumGroup) {
      recordRecentAlbum({
        album_artist: currentAlbumGroup.artist,
        album_name: currentAlbumGroup.name,
        first_track_id: currentAlbumGroup.first_track_id
      });
    }

    const currentShuffle = get(playerState).shuffle;
    const finalQueue =
      currentShuffle && queueIds.length > 1
        ? [track.id, ...shuffle(queueIds.filter((id) => id !== track.id))]
        : queueIds;

    setPlayingPlaylistContext(null);

    playerState.update((s) => ({
      ...s,
      trackId: track.id,
      track,
      queue: finalQueue,
      queueIndex: 0,
      positionMs: 0,
      paused: false
    }));

    wsSend("playback.queue", {
      track_ids: finalQueue,
      start_index: 0
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

      wsSend("playback.queue", {
        track_ids: newQueue,
        start_index: s.queueIndex
      });

      return { ...s, queue: newQueue };
    });
  }

  function openAlbum(album: AlbumGroup) {
    pushNav({ view: "library", albumKey: album.key });
  }

  let albumCtxMenu: { album: AlbumGroup; x: number; y: number } | null =
    $state(null);
  let albumCtxPlaylistSub = $state(false);

  function onAlbumContext(event: MouseEvent, album: AlbumGroup) {
    event.preventDefault();

    albumCtxMenu = { album, x: event.clientX, y: event.clientY };
    albumCtxPlaylistSub = false;

    const close = () => {
      albumCtxMenu = null;
      window.removeEventListener("click", close);
    };

    window.addEventListener("click", close);
  }

  async function addAlbumToPlaylist(playlistId: string, album: AlbumGroup) {
    albumCtxMenu = null;

    const parts =
      album.key === "__unorganized__" ? ["", ""] : album.key.split("|||");
    const tracks = await fetchAlbumTracks(parts[0] ?? "", parts[1] ?? "");

    await handleAddTracksToPlaylist(tracks, playlistId);
  }

  function hideImgOnError(e: Event) {
    const img = e.currentTarget as HTMLImageElement;
    if (img) img.style.display = "none";
  }

  function handleTrackArtError(e: Event, trackID?: string) {
    hideImgOnError(e);
    if (trackID) {
      markMissingTrackArtID(trackID);
    }
  }

  let isScrolling = $state(false);
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
            {#if currentAlbumGroup.first_track_id && !$missingTrackArtIDs[currentAlbumGroup.first_track_id]}
              <img
                src={artworkUrl(currentAlbumGroup.first_track_id)}
                alt={currentAlbumGroup.name}
                onerror={(e) =>
                  handleTrackArtError(e, currentAlbumGroup?.first_track_id)}
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
          <SortButton
            class="sortable"
            bind:currentField={albumSortField}
            bind:sortDir={albumSortDir}
            field="title">Title</SortButton
          >
          <SortButton
            class="sortable"
            bind:currentField={albumSortField}
            bind:sortDir={albumSortDir}
            field="artist">Artist</SortButton
          >
          <SortButton
            class="sortable"
            bind:currentField={albumSortField}
            bind:sortDir={albumSortDir}
            field="duration">Duration</SortButton
          >
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
                    playlists={$playlistsStore}
                    onPlay={(t) => t && playTrack(t)}
                    onSelect={() => {}}
                    onAddToQueue={(t) => t && addToQueue(t)}
                    onAddToPlaylist={(t, id) => handleAddToPlaylist(t, id)}
                    onToggleFavorite={toggleFavoriteTrack}
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
              <button
                class="album-card"
                onclick={() => openAlbum(album)}
                oncontextmenu={(e) => onAlbumContext(e, album)}
              >
                <div class="album-art">
                  {#if !$missingTrackArtIDs[album.first_track_id]}
                    <img
                      src={artworkUrl(album.first_track_id)}
                      alt={album.name}
                      onerror={(e) =>
                        handleTrackArtError(e, album.first_track_id)}
                      loading="lazy"
                    />
                  {/if}
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

  {#if albumCtxMenu}
    {@const album = albumCtxMenu.album}
    <div
      class="album-ctx-menu"
      use:portal
      style="left:{albumCtxMenu.x}px;top:{albumCtxMenu.y}px"
    >
      {#if $playlistsStore.length > 0}
        <!-- svelte-ignore a11y_no_static_element_interactions -->
        <div
          class="album-ctx-sub-wrap"
          onmouseenter={() => (albumCtxPlaylistSub = true)}
          onmouseleave={() => (albumCtxPlaylistSub = false)}
        >
          <button class="has-sub"
            >Add all to playlist <ChevronRight size={12} /></button
          >
          {#if albumCtxPlaylistSub}
            <div class="album-ctx-submenu">
              {#each visiblePlaylistsForAddMenu($playlistsStore) as playlist (playlist.id)}
                <button onclick={() => addAlbumToPlaylist(playlist.id, album)}
                  >{playlist.name}</button
                >
              {/each}
            </div>
          {/if}
        </div>
      {:else}
        <button disabled style="opacity:0.5">No playlists yet</button>
      {/if}
    </div>
  {/if}
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
    padding: 16px 8px 0 0;
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

  .album-ctx-menu {
    position: fixed;
    z-index: 9999;
    background: var(--surface-2);
    border: 1px solid var(--border);
    border-radius: var(--r-md);
    padding: 4px 0;
    box-shadow: var(--shadow-pop);
    min-width: 180px;
  }

  .album-ctx-menu button {
    display: block;
    width: 100%;
    text-align: left;
    padding: 8px 14px;
    font-size: 13px;
    color: var(--text-1);
    border-radius: 0;
    cursor: pointer;
  }

  .album-ctx-menu button:hover {
    background: var(--surface-hover);
  }

  .album-ctx-sub-wrap {
    position: relative;
  }

  .album-ctx-sub-wrap .has-sub {
    cursor: default;
  }

  .album-ctx-submenu {
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

  .album-ctx-submenu button {
    display: block;
    width: 100%;
    text-align: left;
    padding: 8px 14px;
    font-size: 13px;
    color: var(--text-1);
    border-radius: 0;
  }

  .album-ctx-submenu button:hover {
    background: var(--surface-hover);
  }

  @media (max-width: 980px) {
    .scroll-body {
      padding: 8px 0 0;
    }

    .toolbar {
      margin-bottom: 12px;
    }

    h2 {
      font-size: 18px;
    }

    .album-grid-search {
      max-width: none;
      margin-bottom: 12px;
    }

    .album-grid {
      grid-template-columns: repeat(auto-fill, minmax(128px, 1fr));
      gap: 12px;
    }

    .album-detail-header {
      flex-direction: column;
      align-items: stretch;
      gap: 10px;
      margin-bottom: 12px;
    }

    .album-art-hero {
      width: min(56vw, 220px);
      height: min(56vw, 220px);
      border-radius: 10px;
      margin: 0 auto;
    }

    .album-detail-info {
      align-items: center;
      text-align: center;
      padding-bottom: 0;
    }

    .album-detail-title {
      font-size: 17px;
      margin-bottom: 2px;
    }

    .album-meta {
      font-size: 12px;
      line-height: 1.35;
    }

    .album-filter-bar {
      width: 100%;
      margin-top: 10px;
    }

    .album-filter-input {
      max-width: none;
    }

    .track-headers {
      display: none;
    }

    .track-list :global(.track-row) {
      grid-template-columns: 30px minmax(0, 1fr) 68px;
      gap: 0 8px;
      min-height: 40px;
      padding: 8px 8px;
    }

    .track-list :global(.track-row .artist) {
      display: none;
    }

    .track-list :global(.track-row .num),
    .track-list :global(.track-row .duration) {
      font-size: 12px;
    }

    .track-list :global(.track-row .title) {
      font-size: 13px;
    }
  }

  @media (max-width: 620px) {
    .album-detail-header {
      align-items: stretch;
    }

    .album-art-hero {
      width: min(66vw, 220px);
      height: min(66vw, 220px);
    }

    .album-grid {
      grid-template-columns: repeat(2, minmax(0, 1fr));
    }
  }
</style>
