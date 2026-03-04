<script lang="ts">
  import { tracks, loading } from "../stores/library"
  import { playerState } from "../stores/player"
  import TrackRow from "./TrackRow.svelte"
  import type { Track } from "../stores/player"
  import { apiBase } from "./api"

  let selectedAlbum: string | null = null  // album_name to filter by

  // Derive unique albums from tracks (grouped by album_name + album_artist)
  $: albumGroups = buildAlbumGroups($tracks)

  interface AlbumGroup {
    key: string
    name: string
    artist: string
    tracks: Track[]
    firstTrackId: string  // for album art
  }

  const UNORGANIZED_KEY = "__unorganized__"

  function buildAlbumGroups(allTracks: Track[]): AlbumGroup[] {
    const map = new Map<string, AlbumGroup>()
    for (const t of allTracks) {
      const hasAlbum = (t.album_name ?? "").trim() !== ""
      const name = hasAlbum ? t.album_name : "Unorganized"
      const artist = hasAlbum ? (t.album_artist || "Unknown Artist") : "Various"
      const key = hasAlbum ? `${name}|||${artist}` : UNORGANIZED_KEY
      let g = map.get(key)
      if (!g) {
        g = { key, name, artist, tracks: [], firstTrackId: t.id }
        map.set(key, g)
      }
      g.tracks.push(t)
    }
    // Separate unorganized from normal albums
    const unorg = map.get(UNORGANIZED_KEY)
    map.delete(UNORGANIZED_KEY)
    // Sort albums alphabetically
    const groups = Array.from(map.values())
    groups.sort((a, b) => a.name.localeCompare(b.name))
    // Sort each album's tracks by disc/track number
    for (const g of groups) {
      g.tracks.sort((a, b) => (a.disc_number ?? 0) - (b.disc_number ?? 0) || (a.track_number ?? 0) - (b.track_number ?? 0))
    }
    // Unorganized tracks sorted by title
    if (unorg) {
      unorg.tracks.sort((a, b) => (a.title || "").localeCompare(b.title || ""))
      groups.push(unorg) // place at end
    }
    return groups
  }

  $: currentAlbumGroup = selectedAlbum
    ? albumGroups.find(g => g.key === selectedAlbum) ?? null
    : null

  async function playTrack(track: Track, albumTracks: Track[]) {
    const idx = albumTracks.findIndex(t => t.id === track.id)
    const queueIds = albumTracks.map(t => t.id)

    await fetch(`${apiBase()}/api/playback/desktop/queue`, {
      method: "POST",
      headers: { "Content-Type": "application/json" },
      body: JSON.stringify({ track_ids: queueIds, start_index: idx >= 0 ? idx : 0 }),
    })

    const res = await fetch(`${apiBase()}/api/playback/desktop/play`, {
      method: "POST",
      headers: { "Content-Type": "application/json" },
      body: JSON.stringify({ track_id: track.id, position_ms: 0 }),
    })
    if (res.ok) {
      playerState.update(s => ({
        ...s,
        trackId: track.id,
        track,
        queue: queueIds,
        queueIndex: idx >= 0 ? idx : 0,
        positionMs: 0,
        paused: false,
      }))
    }
  }

  function addToQueue(track: Track) {
    const newQueue = [...$playerState.queue, track.id]
    fetch(`${apiBase()}/api/playback/desktop/queue`, {
      method: "POST",
      headers: { "Content-Type": "application/json" },
      body: JSON.stringify({ track_ids: newQueue, start_index: $playerState.queueIndex }),
    })
    playerState.update(s => ({
      ...s,
      queue: newQueue,
    }))
  }

  async function scanLibrary() {
    await fetch(`${apiBase()}/api/library/scan`, { method: "POST" })
  }

  function goBack() {
    selectedAlbum = null
  }
</script>

<section>
  {#if currentAlbumGroup}
    <!-- Album detail view -->
    <div class="toolbar">
      <button class="back-btn" on:click={goBack} title="Back to albums">← Back</button>
      <h2>{currentAlbumGroup.name}</h2>
    </div>
    <p class="album-meta text-2">{currentAlbumGroup.artist} · {currentAlbumGroup.tracks.length} tracks</p>

    <div class="track-list">
      <div class="track-header">
        <span>#</span><span>Title</span><span>Artist</span><span>Album</span><span>Duration</span>
      </div>
      {#each currentAlbumGroup.tracks as track (track.id)}
        <TrackRow
          {track}
          active={$playerState.trackId === track.id}
          on:play={() => playTrack(track, currentAlbumGroup.tracks)}
          on:select={() => {}}
          on:addToQueue={() => addToQueue(track)}
        />
      {/each}
    </div>
  {:else}
    <!-- Album grid view -->
    <div class="toolbar">
      <h2>Library</h2>
      <button on:click={scanLibrary} title="Rescan watch folders">↺ Scan</button>
    </div>

    {#if $loading}
      <p class="text-3">Loading…</p>
    {:else if albumGroups.length === 0}
      <p class="text-3">No tracks found. Add a watch folder in Settings and scan.</p>
    {:else}
      <div class="album-grid">
        {#each albumGroups as album (album.key)}
          <button
            class="album-card"
            class:unorganized={album.key === UNORGANIZED_KEY}
            on:click={() => { selectedAlbum = album.key }}
          >
            <div class="album-art" class:unorg-art={album.key === UNORGANIZED_KEY}>
              {#if album.key !== UNORGANIZED_KEY}
                <img
                  src="{apiBase()}/api/library/tracks/{album.firstTrackId}/art"
                  alt={album.name}
                  on:error={(e) => { e.currentTarget.style.display = 'none' }}
                />
              {/if}
              <div class="album-art-placeholder">{album.key === UNORGANIZED_KEY ? "📂" : "♫"}</div>
            </div>
            <p class="album-title truncate" class:unorg-title={album.key === UNORGANIZED_KEY}>{album.name}</p>
            <p class="album-artist truncate text-3">{album.artist} · {album.tracks.length} tracks</p>
          </button>
        {/each}
      </div>
    {/if}
  {/if}
</section>

<style>
  section { height: 100%; display: flex; flex-direction: column; overflow-y: auto; }

  .toolbar {
    display: flex;
    align-items: center;
    justify-content: space-between;
    margin-bottom: 16px;
    gap: 12px;
    flex-shrink: 0;
  }

  h2 { margin: 0; font-size: 20px; font-weight: 700; }

  .back-btn {
    font-size: 13px;
    color: var(--text-2);
    padding: 4px 8px;
    border-radius: var(--r-sm);
  }
  .back-btn:hover { color: var(--text-1); background: var(--surface-hover); }

  .album-meta {
    font-size: 13px;
    margin: -8px 0 16px;
  }

  /* Album grid */
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
  .album-card:hover .album-art { border-color: var(--accent); }

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

  .album-title { margin: 0; font-size: 13px; font-weight: 600; }
  .album-artist { margin: 2px 0 0; font-size: 11px; }

  .unorganized .album-art,
  .unorg-art {
    border: 2px dashed var(--text-3);
    background: transparent;
  }
  .unorg-title { font-style: italic; }

  /* Track list within album detail */
  .track-list { flex: 1; overflow-y: auto; }

  .track-header {
    display: grid;
    grid-template-columns: 32px 2fr 1fr 1fr 56px;
    gap: 0 12px;
    padding: 4px 8px;
    font-size: 11px;
    text-transform: uppercase;
    letter-spacing: 0.05em;
    color: var(--fg-3);
    border-bottom: 1px solid var(--border);
    margin-bottom: 4px;
  }
</style>
