<script lang="ts">
  import { searchTracks, searchAlbumGroups, clearSearch as clearSearchStore, type RemoteAlbumGroup } from "../stores/library"
  import { searchLocalTracksQuery, searchLocalAlbumGroups, fetchLocalAlbumTracks, type LocalAlbumGroup, type LocalTrack } from "../stores/localLibrary"
  import { playerState } from "../stores/player"
  import type { Track } from "../stores/player"
  import { connected, serverFetch, artworkUrl, localBase } from "../utils/api"
  import { wsSend } from "../stores/ws"
  import { pushNav } from "../stores/ui"

  export let query = ""
  let debounce: number

  interface TaggedTrack extends Track { _source: "remote" | "local" }

  let combinedResults: TaggedTrack[] = []
  let remoteAlbumResults: RemoteAlbumGroup[] = []
  let localAlbumResults: LocalAlbumGroup[] = []

  function onInput() {
    clearTimeout(debounce)
    debounce = window.setTimeout(async () => {
      const q = query.trim()
      if (q.length < 2) {
        clearSearchStore()
        combinedResults = []
        remoteAlbumResults = []
        localAlbumResults = []
        return
      }
      try {
        const [remoteResults, localResults, remoteAlbums, localAlbums] = await Promise.all([
          searchTracks(q),
          searchLocalTracksQuery(q),
          searchAlbumGroups(q),
          searchLocalAlbumGroups(q),
        ])
        buildCombined(remoteResults ?? [], localResults ?? [])
        remoteAlbumResults = remoteAlbums ?? []
        localAlbumResults = localAlbums ?? []
      } catch (e) {
        console.warn("Search error:", e)
      }
    }, 300)
  }

  function buildCombined(remoteResults: Track[], localResults: LocalTrack[]) {
    const remote: TaggedTrack[] = remoteResults.map(t => ({ ...t, _source: "remote" as const }))
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
    remoteAlbumResults = []
    localAlbumResults = []
  }

  function localTrackToTrack(t: LocalTrack): Track {
    return {
      id: t.path, path: t.path, title: t.title,
      artist_id: "", album_id: "",
      artist_name: t.artist, album_artist: t.album_artist,
      album_name: t.album, genre: t.genre, year: t.year,
      track_number: t.track_number, disc_number: t.disc_number,
      duration_ms: t.duration_ms, bitrate_kbps: 0,
      replay_gain_track: 0, artwork_id: "",
    }
  }

  async function playTrack(track: TaggedTrack) {
    if (track._source === "local") {
      let albumTracks: Track[] = []
      try {
        const locals = await fetchLocalAlbumTracks(track.album_name ?? "", track.album_artist ?? "")
        albumTracks = locals
          .sort((a, b) => (a.disc_number ?? 0) - (b.disc_number ?? 0) || (a.track_number ?? 0) - (b.track_number ?? 0))
          .map(localTrackToTrack)
      } catch {}
      if (albumTracks.length === 0) {
        albumTracks = combinedResults.filter(t => t._source === "local")
      }
      const idx = albumTracks.findIndex(t => t.id === track.id)
      const startIdx = Math.max(0, idx)
      const queue = albumTracks.slice(startIdx).map(t => t.id)
      const baseQueue = albumTracks.map(t => t.id)
      playerState.update(s => ({
        ...s, trackId: track.id, track, queue, baseQueue, queueIndex: 0, positionMs: 0, paused: false,
      }))
      return
    }

    if (!$connected) return

    let albumTracks: Track[] = []
    try {
      const params = new URLSearchParams()
      params.set("album_name", track.album_name ?? "")
      if (track.album_artist) params.set("album_artist", track.album_artist)
      const r = await serverFetch(`/api/library/tracks?${params}`)
      if (r.ok) {
        const data = await r.json()
        const fetched: Track[] = Array.isArray(data) ? data : (data.tracks ?? [])
        albumTracks = fetched.sort((a, b) =>
          (a.disc_number ?? 0) - (b.disc_number ?? 0) || (a.track_number ?? 0) - (b.track_number ?? 0)
        )
      }
    } catch {}
    if (albumTracks.length === 0) {
      albumTracks = combinedResults.filter(t => t._source === "remote")
    }
    const idx = albumTracks.findIndex(t => t.id === track.id)
    const startIdx = Math.max(0, idx)
    const queue = albumTracks.slice(startIdx).map(t => t.id)
    const baseQueue = albumTracks.map(t => t.id)
    playerState.update(s => ({
      ...s, trackId: track.id, track, queue, baseQueue, queueIndex: 0, positionMs: 0, paused: false,
    }))
    wsSend("playback.queue", { device_id: "desktop", track_ids: queue, start_index: 0 })
    wsSend("playback.play",  { device_id: "desktop", track_id: track.id, position_ms: 0 })
  }

  function addToQueue(track: TaggedTrack) {
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

  function openRemoteAlbum(album: RemoteAlbumGroup) {
    pushNav({ view: "library", tab: "library", subTab: "albums", albumKey: album.key })
    clearSearch()
  }

  function openLocalAlbum(album: LocalAlbumGroup) {
    pushNav({ view: "library", tab: "local", subTab: "albums", albumKey: album.key })
    clearSearch()
  }

  function localAlbumArtUrl(album: LocalAlbumGroup): string {
    const base = localBase()
    if (!base || !album.first_track_path) return ""
    return `${base}/local/art?path=${encodeURIComponent(album.first_track_path)}`
  }

  $: hasAlbumResults = remoteAlbumResults.length > 0 || localAlbumResults.length > 0
  $: hasTrackResults = combinedResults.length > 0
  $: hasAnyResults = hasAlbumResults || hasTrackResults

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
    {#if hasAnyResults}
      {#if hasAlbumResults}
        <p class="section-label">Albums</p>
        {#each localAlbumResults as album (album.key + "-local")}
          <button class="album-row" on:click={() => openLocalAlbum(album)}>
            <div class="album-thumb">
              <img src={localAlbumArtUrl(album)} alt="" on:error={(e) => { (e.currentTarget as HTMLImageElement).style.display = 'none' }} />
              <span class="album-thumb-ph">♫</span>
            </div>
            <div class="album-info">
              <span class="album-name">{album.name || "Unorganized"}</span>
              <span class="album-meta">{album.artist || "Unknown Artist"} · {album.track_count} tracks</span>
            </div>
          </button>
        {/each}
        {#each remoteAlbumResults as album (album.key + "-remote")}
          <button class="album-row" on:click={() => openRemoteAlbum(album)}>
            <div class="album-thumb">
              <img src={artworkUrl(album.first_track_id)} alt="" on:error={(e) => { (e.currentTarget as HTMLImageElement).style.display = 'none' }} />
              <span class="album-thumb-ph">♫</span>
            </div>
            <div class="album-info">
              <span class="album-name">{album.name || "Unorganized"}</span>
              <span class="album-meta">{album.artist || "Unknown Artist"} · {album.track_count} tracks</span>
            </div>
          </button>
        {/each}
      {/if}

      {#if hasTrackResults}
        {#if hasAlbumResults}<p class="section-label">Tracks</p>{/if}
        {#each combinedResults as track (track._source + ':' + track.id)}
          <button
            class="track-row"
            class:active={$playerState.trackId === track.id}
            on:dblclick={() => playTrack(track)}
            on:contextmenu|preventDefault={(e) => { addToQueue(track) }}
          >
            <span class="track-title">{track.title ?? "Unknown"}</span>
            <span class="track-artist">{track.artist_name || track.album_artist || ""}</span>
          </button>
        {/each}
      {/if}
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
  .search-bar:focus-within { border-color: var(--accent); }
  .search-icon { color: var(--text-3); flex-shrink: 0; }
  input {
    flex: 1;
    background: none;
    border: none;
    color: var(--fg);
    font-size: 13px;
    outline: none;
    padding: 0;
  }
  input::placeholder { color: var(--text-3); }
  input[type="search"]::-webkit-search-cancel-button { display: none; }
  .clear-btn {
    background: none;
    border: none;
    color: var(--text-3);
    font-size: 18px;
    cursor: pointer;
    padding: 0 2px;
    line-height: 1;
  }
  .clear-btn:hover { color: var(--fg); }

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
    transition: background 0.1s;
  }
  .album-row:hover { background: var(--surface-hover); }

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
  .album-thumb img { position: absolute; width: 100%; height: 100%; object-fit: cover; z-index: 1; }
  .album-thumb-ph { font-size: 14px; color: var(--text-3); }

  .album-info { display: flex; flex-direction: column; min-width: 0; flex: 1; gap: 1px; }
  .album-name { font-size: 13px; font-weight: 600; white-space: nowrap; overflow: hidden; text-overflow: ellipsis; }
  .album-meta { font-size: 11px; color: var(--text-3); white-space: nowrap; overflow: hidden; text-overflow: ellipsis; }

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
    transition: background 0.1s;
  }
  .track-row:hover, .track-row.active { background: var(--surface-hover); }
  .track-row.active .track-title { color: var(--accent); }
  .track-title { font-size: 13px; font-weight: 500; white-space: nowrap; overflow: hidden; text-overflow: ellipsis; }
  .track-artist { font-size: 11px; color: var(--text-3); white-space: nowrap; overflow: hidden; text-overflow: ellipsis; }
</style>
