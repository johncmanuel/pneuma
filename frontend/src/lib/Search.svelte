<script lang="ts">
  import {
    searchResults,
    albumSearchResults,
    searchTracks,
    searchAlbumGroups,
    clearSearch,
    type RemoteAlbumGroup
  } from "../stores/library";
  import {
    searchLocalAlbumGroups,
    type LocalAlbumGroup
  } from "../stores/localLibrary";
  import { playerState } from "../stores/player";
  import TrackRow from "./TrackRow.svelte";
  import type { Track } from "../stores/player";
  import { connected, artworkUrl, localBase } from "../utils/api";
  import { wsSend } from "../stores/ws";
  import { pushNav } from "../stores/ui";

  let query = "";
  let debounce: number;
  let localAlbumResults: LocalAlbumGroup[] = [];

  function onInput() {
    clearTimeout(debounce);
    debounce = window.setTimeout(async () => {
      const q = query.trim();
      if (q.length >= 2) {
        searchTracks(q);
        searchAlbumGroups(q);
        localAlbumResults = await searchLocalAlbumGroups(q);
      } else {
        clearSearch();
        localAlbumResults = [];
      }
    }, 300);
  }

  function playTrack(track: Track) {
    if (!$connected) return;
    playerState.update((s) => ({
      ...s,
      trackId: track.id,
      track,
      queue: [track.id],
      baseQueue: [track.id],
      queueIndex: 0,
      positionMs: 0,
      paused: false
    }));
    wsSend("playback.play", {
      device_id: "desktop",
      track_id: track.id,
      position_ms: 0
    });
  }

  function addToQueue(track: Track) {
    playerState.update((s) => ({
      ...s,
      queue: [...s.queue, track.id]
    }));
  }

  function openRemoteAlbum(album: RemoteAlbumGroup) {
    pushNav({
      view: "library",
      tab: "library",
      subTab: "albums",
      albumKey: album.key
    });
  }

  function openLocalAlbum(album: LocalAlbumGroup) {
    pushNav({
      view: "library",
      tab: "local",
      subTab: "albums",
      albumKey: album.key
    });
  }

  function localAlbumArtUrl(album: LocalAlbumGroup): string {
    const base = localBase();
    if (!base || !album.first_track_path) return "";
    return `${base}/local/art?path=${encodeURIComponent(album.first_track_path)}`;
  }

  $: hasAlbumResults =
    $albumSearchResults.length > 0 || localAlbumResults.length > 0;
  $: hasResults = hasAlbumResults || $searchResults.length > 0;
</script>

<section>
  <h2>Search</h2>
  <input
    type="search"
    placeholder="Search tracks, artists, albums…"
    bind:value={query}
    on:input={onInput}
    class="search-input"
  />

  <div class="results">
    {#if hasResults}
      {#if hasAlbumResults}
        <p class="section-label">Albums</p>
        {#each localAlbumResults as album (album.key + "-local")}
          <button class="album-row" on:click={() => openLocalAlbum(album)}>
            <div class="album-art">
              <img
                src={localAlbumArtUrl(album)}
                alt=""
                on:error={(e) => {
                  (e.currentTarget as HTMLImageElement).style.display = "none";
                }}
              />
              <span class="album-art-ph">♫</span>
            </div>
            <div class="album-info">
              <span class="album-name">{album.name || "Unorganized"}</span>
              <span class="album-meta"
                >{album.artist || "Unknown Artist"} · {album.track_count} tracks ·
                Local</span
              >
            </div>
            <span class="album-chevron">›</span>
          </button>
        {/each}
        {#each $albumSearchResults as album (album.key + "-remote")}
          <button class="album-row" on:click={() => openRemoteAlbum(album)}>
            <div class="album-art">
              <img
                src={artworkUrl(album.first_track_id)}
                alt=""
                on:error={(e) => {
                  (e.currentTarget as HTMLImageElement).style.display = "none";
                }}
              />
              <span class="album-art-ph">♫</span>
            </div>
            <div class="album-info">
              <span class="album-name">{album.name || "Unorganized"}</span>
              <span class="album-meta"
                >{album.artist || "Unknown Artist"} · {album.track_count} tracks</span
              >
            </div>
            <span class="album-chevron">›</span>
          </button>
        {/each}
      {/if}

      {#if $searchResults.length > 0}
        {#if hasAlbumResults}<p class="section-label">Tracks</p>{/if}
        {#each $searchResults as track (track.id)}
          <TrackRow
            {track}
            active={$playerState.trackId === track.id}
            on:play={() => playTrack(track)}
            on:select={() => {}}
            on:addToQueue={() => addToQueue(track)}
          />
        {/each}
      {/if}
    {:else if query.trim().length >= 2}
      <p class="text-3">No results for "{query}"</p>
    {:else}
      <p class="text-3">Type at least 2 characters to search.</p>
    {/if}
  </div>
</section>

<style>
  section {
    height: 100%;
    display: flex;
    flex-direction: column;
  }
  h2 {
    margin: 0 0 16px;
    font-size: 20px;
    font-weight: 700;
  }

  .search-input {
    width: 100%;
    max-width: 480px;
    margin-bottom: 20px;
    padding: 8px 12px;
    border-radius: 6px;
    border: 1px solid var(--border);
    background: var(--surface);
    color: var(--fg);
    font-size: 14px;
  }

  .search-input:focus {
    outline: none;
    border-color: var(--accent);
  }
  .results {
    flex: 1;
    overflow-y: auto;
  }

  .section-label {
    font-size: 11px;
    font-weight: 600;
    text-transform: uppercase;
    letter-spacing: 0.06em;
    color: var(--text-3);
    margin: 12px 0 6px;
    padding: 0 4px;
  }

  .album-row {
    display: flex;
    align-items: center;
    gap: 10px;
    width: 100%;
    padding: 6px 4px;
    border-radius: 6px;
    background: none;
    border: none;
    color: inherit;
    cursor: pointer;
    text-align: left;
    transition: background 0.1s;
  }
  .album-row:hover {
    background: var(--surface-2);
  }

  .album-art {
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
  .album-art img {
    width: 100%;
    height: 100%;
    object-fit: cover;
    position: absolute;
    inset: 0;
    z-index: 1;
  }
  .album-art-ph {
    font-size: 16px;
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
  .album-chevron {
    font-size: 18px;
    color: var(--text-3);
    flex-shrink: 0;
  }
</style>
