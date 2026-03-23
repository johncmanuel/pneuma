<script lang="ts">
  import { onMount, onDestroy } from "svelte";
  import { derived } from "svelte/store";
  import {
    searchTracks,
    searchAlbumGroups,
    fetchAlbumTracks
  } from "../lib/stores/library";
  import { playerState } from "../lib/stores/playback";
  import { artworkUrl } from "../lib/api";
  import { wsSend } from "../lib/ws";
  import { pushNav } from "../lib/stores/ui";
  import { Music, Search } from "@lucide/svelte";
  import type { Track, AlbumGroup } from "../lib/types";

  const currentTrackId = derived(playerState, ($s) => $s.trackId);

  let query = "";
  let debounce: number;
  let reqSeq = 0;
  let inputEl: HTMLInputElement;

  let trackResults: Track[] = [];
  let albumResults: AlbumGroup[] = [];

  $: hasAnyResults = trackResults.length > 0 || albumResults.length > 0;
  $: showResults = query.trim().length >= 2;

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
        const [tracks, albums] = await Promise.all([
          searchTracks(q),
          searchAlbumGroups(q)
        ]);

        if (id !== reqSeq) return;

        trackResults = tracks ?? [];
        albumResults = albums ?? [];
      } catch (e) {
        console.warn("Search error:", e);
      }
    }, 300);
  }

  function clearSearch() {
    query = "";
    trackResults = [];
    albumResults = [];
  }

  async function playTrack(track: Track) {
    let albumTracks: Track[] = [];
    try {
      albumTracks = await fetchAlbumTracks(
        track.album_name ?? "",
        track.album_artist ?? ""
      );
    } catch {
      console.error("Failed to fetch album tracks for search play");
    }

    if (albumTracks.length === 0) {
      albumTracks = trackResults;
    }

    const idx = albumTracks.findIndex((t) => t.id === track.id);
    const queueIds = albumTracks.map((t) => t.id);

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

  function openAlbum(album: AlbumGroup) {
    pushNav({ view: "library", albumKey: album.key });
  }

  function hideImg(e: Event) {
    const img = e.currentTarget as HTMLImageElement;
    if (img) img.style.display = "none";
  }
</script>

<section>
  <div class="search-bar">
    <Search size={16} stroke="currentColor" stroke-width={2} />
    <input
      type="search"
      placeholder="Search tracks, albums..."
      bind:value={query}
      bind:this={inputEl}
      oninput={onInput}
    />
    {#if query.length > 0}
      <button class="clear-btn" onclick={clearSearch}>&times;</button>
    {/if}
  </div>

  {#if showResults}
    <div class="results">
      {#if hasAnyResults}
        {#if albumResults.length > 0}
          <h3 class="section-label">Albums</h3>
          <div class="album-results">
            {#each albumResults as album (album.key)}
              <button class="album-row" onclick={() => openAlbum(album)}>
                <div class="album-thumb">
                  <img
                    src={artworkUrl(album.first_track_id)}
                    alt=""
                    onerror={hideImg}
                  />
                  <span class="album-thumb-ph"><Music size={14} /></span>
                </div>
                <div class="album-info">
                  <span class="album-name">{album.name || "Unorganized"}</span>
                  <span class="album-meta"
                    >{album.artist || "Unknown Artist"} · {album.track_count} tracks</span
                  >
                </div>
              </button>
            {/each}
          </div>
        {/if}

        {#if trackResults.length > 0}
          {#if albumResults.length > 0}<h3 class="section-label">
              Tracks
            </h3>{/if}
          <div class="track-results">
            {#each trackResults as track (track.id)}
              <button
                class="track-row"
                class:active={$currentTrackId === track.id}
                onclick={() => playTrack(track)}
              >
                <span class="track-title">{track.title ?? "Unknown"}</span>
                <span class="track-artist"
                  >{track.artist_name || track.album_artist || ""}</span
                >
              </button>
            {/each}
          </div>
        {/if}
      {:else}
        <p class="no-results text-3">No results for "{query}"</p>
      {/if}
    </div>
  {:else}
    <p class="text-3" style="text-align: center; padding: 40px;">
      Type at least 2 characters to search.
    </p>
  {/if}
</section>

<style>
  section {
    max-width: 600px;
  }

  .search-bar {
    display: flex;
    align-items: center;
    gap: 8px;
    background: var(--surface);
    border: 1px solid var(--border);
    border-radius: 20px;
    padding: 8px 16px;
    width: 100%;
    margin-bottom: 20px;
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
    font-size: 14px;
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

  .results {
    display: flex;
    flex-direction: column;
    gap: 4px;
  }

  .section-label {
    font-size: 11px;
    font-weight: 700;
    text-transform: uppercase;
    letter-spacing: 0.06em;
    color: var(--text-3);
    padding: 12px 0 6px;
    margin: 0;
  }

  .album-results {
    display: flex;
    flex-direction: column;
    gap: 2px;
  }

  .album-row {
    display: flex;
    align-items: center;
    gap: 10px;
    width: 100%;
    padding: 8px 12px;
    background: none;
    border: none;
    color: inherit;
    cursor: pointer;
    text-align: left;
    border-radius: var(--r-sm);
  }
  .album-row:hover {
    background: var(--surface-hover);
  }

  .album-thumb {
    width: 40px;
    height: 40px;
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
    gap: 2px;
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

  .track-results {
    display: flex;
    flex-direction: column;
    gap: 2px;
  }

  .track-row {
    display: flex;
    flex-direction: column;
    gap: 2px;
    width: 100%;
    padding: 8px 12px;
    background: none;
    border: none;
    color: inherit;
    cursor: pointer;
    text-align: left;
    border-radius: var(--r-sm);
  }
  .track-row:hover,
  .track-row.active {
    background: var(--surface-hover);
  }
  .track-row.active .track-title {
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

  .no-results {
    padding: 20px 0;
    font-size: 13px;
  }
</style>
