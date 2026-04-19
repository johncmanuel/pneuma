<script lang="ts">
  import { onDestroy } from "svelte";
  import { Music, Search } from "@lucide/svelte";
  import type { Track, AlbumGroup } from "@pneuma/shared";

  interface SearchResult {
    tracks: Track[];
    albums: AlbumGroup[];
  }

  interface Props {
    placeholder?: string;
    searchFn?: (query: string) => Promise<SearchResult>;
    onPlayTrack?: (track: Track) => void;
    onOpenAlbum?: (album: AlbumGroup) => void;
    onAddToQueue?: (track: Track) => void;
    artworkUrl?: (trackId: string) => string;
    onAlbumArtError?: (album: AlbumGroup) => void;
  }

  let {
    placeholder = "Search tracks, albums...",
    searchFn,
    onPlayTrack,
    onOpenAlbum,
    onAddToQueue,
    artworkUrl = () => "",
    onAlbumArtError
  }: Props = $props();

  let query = $state("");
  let debounce: number;
  let reqSeq = 0;
  let inputEl: HTMLInputElement;
  let resultsEl = $state<HTMLDivElement | null>(null);
  let focused = $state(false);

  let trackResults: Track[] = $state([]);
  let albumResults: AlbumGroup[] = $state([]);
  let activeResultKey: string | null = $state(null);

  const NAV_INTERVAL_MS = 80;
  let lastNavAt = 0;

  export function focus() {
    inputEl?.focus();
    inputEl?.select();
  }

  export function hasResults() {
    return trackResults.length > 0 || albumResults.length > 0;
  }

  function onInput() {
    clearTimeout(debounce);
    debounce = window.setTimeout(async () => {
      const q = query.trim();
      if (q.length < 2) {
        trackResults = [];
        albumResults = [];
        return;
      }

      const id = ++reqSeq;
      try {
        if (searchFn) {
          const result = await searchFn(q);
          if (id !== reqSeq) return;
          trackResults = result.tracks ?? [];
          albumResults = result.albums ?? [];
        }
      } catch (e) {
        console.warn("Search error:", e);
      }
    }, 300);
  }

  function clearSearch() {
    query = "";
    trackResults = [];
    albumResults = [];
    activeResultKey = null;
  }

  function hideImg(e: Event) {
    (e.currentTarget as HTMLImageElement).style.display = "none";
  }

  function handleAlbumArtError(e: Event, album: AlbumGroup) {
    hideImg(e);
    onAlbumArtError?.(album);
  }

  let hasAnyResults = $derived(
    trackResults.length > 0 || albumResults.length > 0
  );
  let showResults = $derived(focused && query.trim().length >= 2);

  $effect(() => {
    const key = activeResultKey;
    if (key && resultsEl) {
      const el = resultsEl.querySelector(
        `[data-result-key="${CSS.escape(key)}"]`
      ) as HTMLElement | null;
      if (el) {
        el.focus({ preventScroll: true });
        scrollResultIntoView(el);
      } else {
        activeResultKey = null;
      }
    }
  });

  function resultButtons(): HTMLElement[] {
    if (!resultsEl) return [];
    return Array.from(
      resultsEl.querySelectorAll<HTMLElement>("button[data-result-key]")
    );
  }

  function scrollResultIntoView(el: HTMLElement) {
    if (!resultsEl) return;
    const top = el.offsetTop;
    const bottom = top + el.offsetHeight;
    if (top < resultsEl.scrollTop) {
      resultsEl.scrollTop = top;
    } else if (bottom > resultsEl.scrollTop + resultsEl.clientHeight) {
      resultsEl.scrollTop = bottom - resultsEl.clientHeight;
    }
  }

  function throttledNav(): boolean {
    const now = Date.now();
    if (now - lastNavAt < NAV_INTERVAL_MS) return false;
    lastNavAt = now;
    return true;
  }

  function navDown() {
    if (!throttledNav()) return;
    const btns = resultButtons();
    if (!btns.length) return;
    const idx = btns.indexOf(document.activeElement as HTMLElement);
    const next = idx < btns.length - 1 ? btns[idx + 1] : btns[btns.length - 1];
    next.focus({ preventScroll: true });
    scrollResultIntoView(next);
  }

  function navUp() {
    if (!throttledNav()) return;
    const btns = resultButtons();
    if (!btns.length) return;
    const idx = btns.indexOf(document.activeElement as HTMLElement);
    if (idx === 0) {
      inputEl?.focus();
    } else if (idx > 0) {
      const prev = btns[idx - 1];
      prev.focus({ preventScroll: true });
      scrollResultIntoView(prev);
    }
  }

  function activateFocused() {
    const active = document.activeElement as HTMLElement | null;
    if (active?.dataset?.resultKey) {
      active.click();
      return;
    }
    resultButtons()[0]?.click();
  }

  function handleKeydown(e: KeyboardEvent) {
    if (e.key === "ArrowDown") {
      e.preventDefault();
      navDown();
    } else if (e.key === "ArrowUp") {
      e.preventDefault();
      navUp();
    } else if (e.key === "Enter") {
      e.preventDefault();
      activateFocused();
    } else if (e.key === "Escape") {
      clearSearch();
      inputEl?.blur();
    }
  }

  function handleContainerFocusOut(e: FocusEvent) {
    if (!(e.currentTarget as HTMLElement).contains(e.relatedTarget as Node)) {
      focused = false;
    }
  }

  onDestroy(() => {
    clearTimeout(debounce);
  });
</script>

<!-- svelte-ignore a11y_no_static_element_interactions -->
<div
  class="search-container"
  onfocusin={() => (focused = true)}
  onfocusout={handleContainerFocusOut}
  onkeydown={handleKeydown}
>
  <div class="search-bar">
    <Search size={16} stroke="currentColor" stroke-width={2} />
    <input
      type="search"
      {placeholder}
      bind:value={query}
      bind:this={inputEl}
      oninput={onInput}
    />
    {#if query.length > 0}
      <button class="clear-btn" onclick={clearSearch}>&times;</button>
    {/if}
  </div>

  {#if showResults}
    <div class="search-results" bind:this={resultsEl}>
      {#if hasAnyResults}
        {#if albumResults.length > 0}
          <p class="section-label">Albums</p>
          {#each albumResults as album (album.key)}
            {@const key = "album:" + album.key}
            <button
              class="album-row"
              data-result-key={key}
              onclick={() => onOpenAlbum?.(album)}
              onfocus={() => {
                activeResultKey = key;
              }}
            >
              <div class="album-thumb">
                <img
                  src={artworkUrl(album.first_track_id)}
                  alt=""
                  onerror={(e) => handleAlbumArtError(e, album)}
                />
                <span class="album-thumb-ph"><Music size={14} /></span>
              </div>
              <div class="album-info">
                <span class="album-name">{album.name || "Unorganized"}</span>
                <span class="album-meta"
                  >{album.artist || "Unknown Artist"} &middot; {album.track_count}
                  tracks</span
                >
              </div>
            </button>
          {/each}
        {/if}

        {#if trackResults.length > 0}
          {#if albumResults.length > 0}<p class="section-label">Tracks</p>{/if}
          {#each trackResults as track (track.id)}
            {@const key = "track:" + track.id}
            <button
              class="track-row"
              data-result-key={key}
              onclick={() => onPlayTrack?.(track)}
              onfocus={() => {
                activeResultKey = key;
              }}
            >
              <span class="track-title">{track.title ?? "Unknown"}</span>
              <span class="track-artist"
                >{track.artist_name || track.album_artist || ""}</span
              >
            </button>
          {/each}
        {/if}
      {:else}
        <p class="no-results">No results for "{query}"</p>
      {/if}
    </div>
  {/if}
</div>

<style>
  .search-container {
    position: relative;
    width: 100%;
  }

  .search-bar {
    display: flex;
    align-items: center;
    gap: 8px;
    background: var(--surface);
    border: 1px solid var(--border);
    border-radius: 20px;
    padding: 5px 14px;
    max-width: 420px;
    width: 100%;
    transition: border-color 0.15s;
  }
  .search-bar:focus-within {
    border-color: var(--accent);
  }

  input {
    flex: 1;
    background: none;
    border: none;
    color: var(--text-1);
    font-size: 13px;
    outline: none;
    padding: 0;
  }
  input::placeholder {
    color: var(--text-3);
  }
  input[type="search"]::-webkit-search-cancel-button {
    display: none;
  }
  .clear-btn {
    background: none;
    border: none;
    color: var(--text-3);
    font-size: 18px;
    cursor: pointer;
    padding: 0 2px;
    line-height: 1;
  }
  .clear-btn:hover {
    color: var(--text-1);
  }

  .search-results {
    position: absolute;
    top: 100%;
    left: 0;
    right: 0;
    max-height: calc(100vh - 120px);
    overflow-y: auto;
    overscroll-behavior: contain;
    background: var(--surface);
    border: 1px solid var(--border);
    border-radius: 8px;
    margin-top: 4px;
    box-shadow: var(--shadow-pop);
    z-index: 100;
  }

  .no-results {
    padding: 16px;
    color: var(--text-3);
    font-size: 13px;
  }

  .section-label {
    font-size: 10px;
    font-weight: 700;
    text-transform: uppercase;
    letter-spacing: 0.08em;
    color: var(--text-3);
    padding: 10px 14px 4px;
    margin: 0;
  }

  .album-row {
    display: flex;
    align-items: center;
    gap: 10px;
    width: 100%;
    padding: 6px 14px;
    background: none;
    border: none;
    color: inherit;
    cursor: pointer;
    text-align: left;
    outline: none;
  }
  .album-row:hover,
  .album-row:focus {
    background: var(--surface-hover);
    outline: none;
  }

  .album-thumb {
    width: 36px;
    height: 36px;
    border-radius: 4px;
    background: var(--surface-2);
    flex-shrink: 0;
    overflow: hidden;
    position: relative;
    display: flex;
    align-items: center;
    justify-content: center;
  }
  .album-thumb img {
    position: absolute;
    width: 100%;
    height: 100%;
    object-fit: cover;
    z-index: 1;
  }
  .album-thumb-ph {
    font-size: 14px;
    color: var(--text-3);
  }

  .album-info {
    display: flex;
    flex-direction: column;
    min-width: 0;
    flex: 1;
    gap: 1px;
  }
  .album-name {
    font-size: 13px;
    font-weight: 600;
    white-space: nowrap;
    overflow: hidden;
    text-overflow: ellipsis;
  }
  .album-meta {
    font-size: 11px;
    color: var(--text-3);
    white-space: nowrap;
    overflow: hidden;
    text-overflow: ellipsis;
  }

  .track-row {
    display: flex;
    flex-direction: column;
    gap: 2px;
    width: 100%;
    padding: 7px 14px;
    background: none;
    border: none;
    color: inherit;
    cursor: pointer;
    text-align: left;
    outline: none;
  }
  .track-row:hover,
  .track-row:focus {
    background: var(--surface-hover);
    outline: none;
  }
  .track-row:focus .track-title {
    color: var(--accent);
  }
  .track-title {
    font-size: 13px;
    font-weight: 500;
    white-space: nowrap;
    overflow: hidden;
    text-overflow: ellipsis;
  }
  .track-artist {
    font-size: 11px;
    color: var(--text-3);
    white-space: nowrap;
    overflow: hidden;
    text-overflow: ellipsis;
  }
</style>
