<script lang="ts">
  import { createVirtualizer } from "@tanstack/svelte-virtual";
  import SortButton from "./SortButton.svelte";
  import { Music } from "@lucide/svelte";
  import { formatTotalDuration } from "./utils";
  import "@pneuma/ui/css/components.css";

  interface Track {
    id: string;
    title?: string;
    artist_name?: string;
    album_name?: string;
    duration_ms?: number;
    disc_number?: number;
    track_number?: number;
  }

  interface AlbumGroup {
    key: string;
    name: string;
    artist: string;
    track_count: number;
    first_track_id: string;
  }

  interface Props {
    album: AlbumGroup | null;
    tracks: Track[];
    loading?: boolean;
    currentTrackId?: string | null;
    active?: boolean;
    isLocal?: boolean;
    showFavoriteState?: boolean;
    favoriteTrackIds?: Set<string>;
    onPlayTrack?: (track: Track) => void;
    onSelectTrack?: (track: Track) => void;
    onAddToQueue?: (track: Track) => void;
    onToggleFavorite?: (track: Track) => void;
    getArtUrl?: () => string;
    hideImgOnError?: (e: Event) => void;
    trackRowComponent?: any;
  }

  let {
    album = null,
    tracks = [],
    loading = false,
    currentTrackId = null,
    active = false,
    isLocal = false,
    showFavoriteState = false,
    favoriteTrackIds = new Set(),
    onPlayTrack = () => {},
    onSelectTrack = () => {},
    onAddToQueue = () => {},
    onToggleFavorite = () => {},
    getArtUrl = () => "",
    hideImgOnError = () => {},
    trackRowComponent = null
  }: Props = $props();

  type SortField = "default" | "title" | "artist" | "duration";
  type SortDir = "asc" | "desc";

  let albumFilter = $state("");
  let albumSortField: SortField = $state("default");
  let albumSortDir: SortDir = $state("asc");

  let trackListEl: HTMLDivElement | undefined = $state();
  let isScrolling = $state(false);
  let scrollTimer: ReturnType<typeof setTimeout>;

  let filteredTracks = $derived(
    (() => {
      if (!album) return [];

      const f = albumFilter.toLowerCase();
      let result = tracks;

      if (f) {
        result = result.filter(
          (t) =>
            (t.title ?? "").toLowerCase().includes(f) ||
            (t.artist_name ?? "").toLowerCase().includes(f)
        );
      }

      if (albumSortField !== "default") {
        const dir = albumSortDir === "asc" ? 1 : -1;

        result = [...result].sort((a, b) => {
          if (albumSortField === "title")
            return dir * (a.title ?? "").localeCompare(b.title ?? "");
          if (albumSortField === "artist")
            return (
              dir * (a.artist_name ?? "").localeCompare(b.artist_name ?? "")
            );
          if (albumSortField === "duration")
            return dir * ((a.duration_ms ?? 0) - (b.duration_ms ?? 0));
          return 0;
        });
      }
      return result;
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

  function handleScroll() {
    isScrolling = true;
    clearTimeout(scrollTimer);
    scrollTimer = setTimeout(() => {
      isScrolling = false;
    }, 150);
  }

  function handlePlay(track: Track | null) {
    if (track) onPlayTrack(track);
  }

  function handleQueue(track: Track | null) {
    if (track) onAddToQueue(track);
  }
</script>

<div class="album-detail-view">
  <div class="album-detail-header">
    <div class="album-art-hero">
      {#if getArtUrl() && getArtUrl() !== ""}
        <img
          src={getArtUrl()}
          alt={album?.name ?? ""}
          onerror={hideImgOnError}
          loading="lazy"
          decoding="async"
        />
      {/if}
      <div class="album-art-hero-placeholder"><Music size={24} /></div>
    </div>
    <div class="album-detail-info">
      <h2 class="album-detail-title">{album?.name ?? ""}</h2>
      <p class="album-meta text-2">
        {album?.artist ?? ""} · {tracks.length} tracks ·
        {formatTotalDuration(
          tracks.reduce((sum, t) => sum + (t.duration_ms ?? 0), 0)
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
            {#if trackRowComponent}
              {@const TrackRowCmp = trackRowComponent}
              <TrackRowCmp
                track={filteredTracks[row.index]}
                hideAlbum={true}
                {isLocal}
                isFavorite={showFavoriteState &&
                  favoriteTrackIds.has(filteredTracks[row.index]?.id ?? "")}
                active={currentTrackId === filteredTracks[row.index]?.id}
                onPlay={() => handlePlay(filteredTracks[row.index])}
                onSelect={() => onSelectTrack(filteredTracks[row.index])}
                onAddToQueue={() => handleQueue(filteredTracks[row.index])}
                onToggleFavorite={() =>
                  onToggleFavorite(filteredTracks[row.index])}
              />
            {:else}
              <div class="fallback-track-row">
                <span class="num"
                  >{filteredTracks[row.index].track_number ??
                    row.index + 1}</span
                >
                <span class="title"
                  >{filteredTracks[row.index].title ?? "Unknown"}</span
                >
                <span class="artist"
                  >{filteredTracks[row.index].artist_name ?? ""}</span
                >
              </div>
            {/if}
          </div>
        {/each}
      </div>
    </div>
  {/if}
</div>

<style>
  .album-detail-view {
    flex: 1;
    display: flex;
    flex-direction: column;
    min-height: 0;
    overflow: hidden;
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

  .track-headers {
    display: grid;
    grid-template-columns: 30px 1fr 1fr 68px;
    gap: 8px;
    padding: 8px 12px;
    border-bottom: 1px solid var(--border);
    font-size: 11px;
    text-transform: uppercase;
    color: var(--text-3);
  }

  .track-headers :global(.sortable) {
    text-align: left;
  }

  .no-results {
    padding: 16px 8px;
    font-size: 13px;
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

  .fallback-track-row {
    display: grid;
    grid-template-columns: 30px 1fr 1fr 68px;
    gap: 8px;
    padding: 8px 12px;
    align-items: center;
    font-size: 13px;
  }

  .fallback-track-row .num {
    color: var(--text-3);
  }

  .fallback-track-row .title {
    font-weight: 500;
  }

  .fallback-track-row .artist {
    color: var(--text-2);
  }
</style>
