<script lang="ts">
  import { Search, X, Music, FolderOpen } from "@lucide/svelte";
  import type { Snippet } from "svelte";
  import "@pneuma/ui/css/components.css";

  interface AlbumGroup {
    key: string;
    name: string;
    artist: string;
    track_count: number;
    first_track_id: string;
  }

  interface Props {
    albums: AlbumGroup[];
    total?: number;
    hasMore?: boolean;
    loading?: boolean;
    title?: string;
    searchPlaceholder?: string;
    onLoadMore?: () => void;
    onSearch?: (query: string) => void;
    onSelectAlbum?: (album: AlbumGroup) => void;
    onContextMenu?: (e: MouseEvent, album: AlbumGroup) => void;
    getArtUrl?: (album: AlbumGroup) => string;
    hideImgOnError?: (e: Event) => void;
    toolbarActions?: Snippet;
    afterSearch?: Snippet;
    emptyState?: Snippet;
    unorganizedKey?: string;
  }

  let {
    albums = [],
    total = 0,
    hasMore = false,
    loading = false,
    title = "Library",
    searchPlaceholder = "Search albums...",
    onLoadMore = () => {},
    onSearch = () => {},
    onSelectAlbum = () => {},
    onContextMenu = () => {},
    getArtUrl = () => "",
    hideImgOnError = () => {},
    toolbarActions,
    afterSearch,
    emptyState,
    unorganizedKey = "__unorganized__"
  }: Props = $props();

  let searchQuery = $state("");
  let gridScrollEl: HTMLDivElement | undefined = $state();
  let loadingMore = $state(false);

  let searchDebounce: ReturnType<typeof setTimeout>;

  function handleSearchInput() {
    clearTimeout(searchDebounce);
    searchDebounce = setTimeout(() => {
      onSearch(searchQuery.trim());
    }, 300);
  }

  function clearSearch() {
    searchQuery = "";
    onSearch("");
  }

  function handleScroll() {
    if (loadingMore || !hasMore || !gridScrollEl) return;
    const { scrollTop, scrollHeight, clientHeight } = gridScrollEl;
    if (scrollTop + clientHeight >= scrollHeight - 200) {
      loadMore();
    }
  }

  async function loadMore() {
    loadingMore = true;
    try {
      await onLoadMore();
    } finally {
      loadingMore = false;
    }
  }
</script>

<div
  class="grid-scroll-wrapper"
  bind:this={gridScrollEl}
  onscroll={handleScroll}
>
  <div class="toolbar">
    <h2>{title}</h2>
    {#if toolbarActions}
      <div class="toolbar-actions">
        {@render toolbarActions()}
      </div>
    {/if}
  </div>

  <div class="album-grid-search">
    <Search size={14} />
    <input
      type="search"
      class="album-grid-filter"
      placeholder={searchPlaceholder}
      bind:value={searchQuery}
      oninput={handleSearchInput}
    />
    {#if searchQuery}
      <button class="grid-filter-clear" onclick={clearSearch}>
        <X size={14} />
      </button>
    {/if}
  </div>

  {#if afterSearch}
    <div class="after-search">
      {@render afterSearch()}
    </div>
  {/if}

  {#if loading && albums.length === 0}
    <p class="text-3" style="text-align: center; padding: 24px;">Loading...</p>
  {:else if albums.length === 0}
    {#if emptyState}
      {@render emptyState()}
    {:else}
      <p class="text-3">No albums found.</p>
    {/if}
  {:else}
    <div class="album-grid">
      {#each albums as album (album.key)}
        <button
          class="album-card"
          class:unorganized={album.key === unorganizedKey}
          onclick={() => onSelectAlbum(album)}
          oncontextmenu={(e) => onContextMenu(e, album)}
        >
          <div class="album-art" class:unorg-art={album.key === unorganizedKey}>
            {#if album.key !== unorganizedKey}
              <img
                src={getArtUrl(album)}
                alt={album.name}
                onerror={hideImgOnError}
                loading="lazy"
              />
            {/if}
            <div class="album-art-placeholder">
              {#if album.key === unorganizedKey}
                <FolderOpen size={24} />
              {:else}
                <Music size={24} />
              {/if}
            </div>
          </div>
          <p
            class="album-title truncate"
            class:unorg-title={album.key === unorganizedKey}
          >
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
        {loadingMore ? "Loading more..." : ""}
      </p>
    {/if}
  {/if}
</div>

<style>
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

  .toolbar-actions {
    display: flex;
    gap: 8px;
    align-items: center;
  }

  h2 {
    margin: 0;
    font-size: 20px;
    font-weight: 700;
  }

  .album-grid-filter {
    flex: 1;
    background: none;
    border: none;
    color: var(--fg);
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
    color: var(--fg);
  }

  .after-search {
    margin-bottom: 12px;
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
    background: none;
    border: none;
  }

  .album-card:hover .album-art {
    border-color: var(--accent);
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

  .unorganized .album-art,
  .unorg-art {
    border: 2px dashed var(--text-3);
    background: transparent;
  }

  .unorg-title {
    font-style: italic;
  }
</style>
