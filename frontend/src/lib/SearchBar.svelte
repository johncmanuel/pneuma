<script lang="ts">
  import { searchResults, searchTracks, clearSearch as clearSearchStore } from "../stores/library"
  import { searchLocalTracksQuery } from "../stores/localLibrary"
  import { playerState } from "../stores/player"
  import TrackRow from "./TrackRow.svelte"
  import type { Track } from "../stores/player"
  import { connected } from "../utils/api"
  import { wsSend } from "../stores/ws"

  export let query = ""
  let debounce: number

  interface TaggedTrack extends Track { _source: "remote" | "local" }

  let combinedResults: TaggedTrack[] = []

  function onInput() {
    clearTimeout(debounce)
    debounce = window.setTimeout(async () => {
      const q = query.trim()
      if (q.length < 2) {
        clearSearchStore()
        combinedResults = []
        return
      }
      // Fire remote search (updates searchResults store) and local search
      // concurrently — both go to their respective backends.
      const [, localResults] = await Promise.all([
        searchTracks(q),
        searchLocalTracksQuery(q),
      ])
      // Build combined list from backend results
      buildCombined(localResults ?? [])
    }, 300)
  }

  function buildCombined(localResults: import("../stores/localLibrary").LocalTrack[]) {
    // Tag remote results
    const remote: TaggedTrack[] = $searchResults.map(t => ({ ...t, _source: "remote" as const }))
    // Convert local results from Go IPC (already limited to 50 server-side)
    const local: TaggedTrack[] = localResults
      .slice(0, 20)
      .map(t => ({
        id: t.path,
        path: t.path,
        title: t.title,
        artist_id: "",
        album_id: "",
        artist_name: t.artist,
        album_artist: t.album_artist,
        album_name: t.album,
        genre: t.genre,
        year: t.year,
        track_number: t.track_number,
        disc_number: t.disc_number,
        duration_ms: t.duration_ms,
        bitrate_kbps: 0,
        replay_gain_track: 0,
        artwork_id: "",
        _source: "local" as const,
      }))
    combinedResults = [...remote, ...local]
  }

  function clearSearch() {
    query = ""
    clearSearchStore()
    combinedResults = []
  }

  function playTrack(track: TaggedTrack) {
    if (track._source === "local") {
      // Local playback
      const q = combinedResults.filter(t => t._source === "local").map(t => t.id)
      const idx = q.indexOf(track.id)
      const queue = [...q.slice(idx), ...q.slice(0, idx)]
      playerState.update(s => ({
        ...s, trackId: track.id, track, queue, baseQueue: queue, queueIndex: 0, positionMs: 0, paused: false,
      }))
      return
    }
    if (!$connected) return
    const q = combinedResults.filter(t => t._source === "remote").map(t => t.id)
    const idx = q.indexOf(track.id)
    const queue = [...q.slice(idx), ...q.slice(0, idx)]
    playerState.update(s => ({
      ...s, trackId: track.id, track, queue, baseQueue: queue, queueIndex: 0, positionMs: 0, paused: false,
    }))
    wsSend("playback.queue", { device_id: "desktop", track_ids: queue, start_index: 0 })
    wsSend("playback.play",  { device_id: "desktop", track_id: track.id, position_ms: 0 })
  }

  function addToQueue(track: TaggedTrack) {
    // Insert directly after the currently playing track (Spotify-style).
    // Do NOT send playback.queue to the server — SetQueue resets PositionMS=0.
    playerState.update(s => {
      const insertAt = s.queueIndex + 1
      const newQueue = [
        ...s.queue.slice(0, insertAt),
        track.id,
        ...s.queue.slice(insertAt),
      ]
      return { ...s, queue: newQueue }
    })
  }

  export const hasResults = () => query.trim().length >= 2
</script>

<div class="search-bar">
  <svg class="search-icon" viewBox="0 0 24 24" width="16" height="16" fill="none" stroke="currentColor" stroke-width="2">
    <circle cx="11" cy="11" r="8"/><path d="M21 21l-4.35-4.35"/>
  </svg>
  <input
    type="search"
    placeholder="Search tracks, artists, albums…"
    bind:value={query}
    on:input={onInput}
  />
  {#if query.length > 0}
    <button class="clear-btn" on:click={clearSearch}>&times;</button>
  {/if}
</div>

{#if query.trim().length >= 2}
  <div class="search-results">
    {#if combinedResults.length > 0}
      {#each combinedResults as track (track._source + ':' + track.id)}
        <div class="result-row">
          <TrackRow
            track={track}
            active={$playerState.trackId === track.id}
            on:play={() => playTrack(track)}
            on:select={() => {}}
            on:addToQueue={() => addToQueue(track)}
          />
          <span class="source-badge" class:local={track._source === "local"} class:remote={track._source === "remote"}>
            {track._source === "local" ? "Local" : "Remote"}
          </span>
        </div>
      {/each}
    {:else}
      <p class="no-results">No results for "{query}"</p>
    {/if}
  </div>
{/if}

<style>
  .search-bar {
    display: flex;
    align-items: center;
    gap: 8px;
    background: var(--surface);
    border: 1px solid var(--border);
    border-radius: 20px;
    padding: 6px 14px;
    max-width: 420px;
    width: 100%;
    transition: border-color 0.15s;
  }

  .search-bar:focus-within {
    border-color: var(--accent);
  }

  .search-icon {
    color: var(--text-3);
    flex-shrink: 0;
  }

  input {
    flex: 1;
    background: none;
    border: none;
    color: var(--fg);
    font-size: 13px;
    outline: none;
    padding: 0;
  }

  input::placeholder {
    color: var(--text-3);
  }

  /* hide default search clear button */
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
    color: var(--fg);
  }

  .search-results {
    position: absolute;
    top: 100%;
    left: 0;
    right: 0;
    max-height: calc(100vh - 120px);
    overflow-y: auto;
    background: var(--surface);
    border: 1px solid var(--border);
    border-radius: 8px;
    margin-top: 4px;
    box-shadow: 0 8px 24px rgba(0,0,0,0.4);
    z-index: 100;
  }

  .no-results {
    padding: 16px;
    color: var(--text-3);
    font-size: 13px;
  }

  .result-row {
    position: relative;
  }

  .source-badge {
    position: absolute;
    top: 50%;
    right: 12px;
    transform: translateY(-50%);
    font-size: 9px;
    font-weight: 700;
    text-transform: uppercase;
    letter-spacing: 0.04em;
    padding: 2px 6px;
    border-radius: 4px;
    pointer-events: none;
  }
  .source-badge.remote {
    background: rgba(96, 165, 250, 0.15);
    color: rgb(96, 165, 250);
  }
  .source-badge.local {
    background: rgba(74, 222, 128, 0.15);
    color: rgb(74, 222, 128);
  }
</style>
