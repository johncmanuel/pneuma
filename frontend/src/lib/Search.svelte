<script lang="ts">
  import { searchResults, searchTracks } from "../stores/library"
  import { playerState } from "../stores/player"
  import TrackRow from "./TrackRow.svelte"
  import type { Track } from "../stores/player"
  import { connected } from "./api"
  import { wsSend } from "../stores/ws"

  let query = ""
  let debounce: number

  function onInput() {
    clearTimeout(debounce)
    debounce = window.setTimeout(() => {
      if (query.trim().length >= 2) searchTracks(query.trim())
      else searchResults.set([])
    }, 300)
  }

  function playTrack(track: Track) {
    if (!$connected) return
    playerState.update(s => ({ ...s, trackId: track.id, track, positionMs: 0, paused: false }))
    wsSend("playback.play", { device_id: "desktop", track_id: track.id, position_ms: 0 })
  }

  function addToQueue(track: Track) {
    playerState.update(s => ({
      ...s,
      queue: [...s.queue, track.id],
    }))
  }
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
    {:else if query.trim().length >= 2}
      <p class="text-3">No results for "{query}"</p>
    {:else}
      <p class="text-3">Type at least 2 characters to search.</p>
    {/if}
  </div>
</section>

<style>
  section { height: 100%; display: flex; flex-direction: column; }
  h2 { margin: 0 0 16px; font-size: 20px; font-weight: 700; }

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

  .search-input:focus { outline: none; border-color: var(--accent); }

  .results { flex: 1; overflow-y: auto; }
</style>
