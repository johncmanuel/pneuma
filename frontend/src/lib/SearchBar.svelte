<script lang="ts">
  import { searchResults, searchTracks } from "../stores/library"
  import { playerState } from "../stores/player"
  import TrackRow from "./TrackRow.svelte"
  import type { Track } from "../stores/player"
  import { serverFetch, connected } from "./api"

  export let query = ""
  let debounce: number

  function onInput() {
    clearTimeout(debounce)
    debounce = window.setTimeout(() => {
      if (query.trim().length >= 2) searchTracks(query.trim())
      else searchResults.set([])
    }, 300)
  }

  function clearSearch() {
    query = ""
    searchResults.set([])
  }

  async function playTrack(track: Track) {
    if (!$connected) return
    const q = $searchResults.map(t => t.id)
    const idx = q.indexOf(track.id)
    const queue = [...q.slice(idx), ...q.slice(0, idx)]
    await serverFetch("/api/playback/desktop/queue", {
      method: "POST",
      headers: { "Content-Type": "application/json" },
      body: JSON.stringify({ track_ids: queue }),
    })
    const res = await serverFetch("/api/playback/desktop/play", {
      method: "POST",
      headers: { "Content-Type": "application/json" },
      body: JSON.stringify({ track_id: track.id, position_ms: 0 }),
    })
    if (res.ok) {
      playerState.update(s => ({
        ...s,
        trackId: track.id,
        track,
        queue,
        positionMs: 0,
        paused: false,
      }))
    }
  }

  function addToQueue(track: Track) {
    if (!$connected) return
    playerState.update(s => {
      const newQueue = [...s.queue, track.id]
      serverFetch("/api/playback/desktop/queue", {
        method: "POST",
        headers: { "Content-Type": "application/json" },
        body: JSON.stringify({ track_ids: newQueue }),
      })
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
    {#if $searchResults.length > 0}
      {#each $searchResults as track (track.id)}
        <TrackRow
          {track}
          active={$playerState.trackId === track.id}
          on:play={() => playTrack(track)}
          on:select={() => {}}
          on:addToQueue={() => addToQueue(track)}
        />
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
</style>
