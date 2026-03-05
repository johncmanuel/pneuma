<script lang="ts">
  import { tracks, loading } from "../stores/library"
  import { localTracks, localLoading, localFolders, addLocalFolder, removeLocalFolder, scanLocalFolders } from "../stores/localLibrary"
  import { playerState } from "../stores/player"
  import TrackRow from "./TrackRow.svelte"
  import type { Track } from "../stores/player"
  import { serverFetch, artworkUrl, connected, isReconnecting, localBase } from "./api"
  import { wsSend } from "../stores/ws"
  import { onMount } from "svelte"

  type LibTab = "library" | "local"
  let activeTab: LibTab = "library"

  let selectedAlbum: string | null = null  // album_name to filter by

  // ─── Shared album grouping logic ────────────────────────────────────────────

  interface AlbumGroup {
    key: string
    name: string
    artist: string
    tracks: Track[]
    firstTrackId: string  // for album art
    isLocal?: boolean
    firstLocalPath?: string // for local art
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

  function buildLocalAlbumGroups(localTracks: import("../stores/localLibrary").LocalTrack[]): AlbumGroup[] {
    const map = new Map<string, AlbumGroup>()
    for (const t of localTracks) {
      const hasAlbum = (t.album ?? "").trim() !== ""
      const name = hasAlbum ? t.album : "Unorganized"
      const artist = hasAlbum ? (t.album_artist || t.artist || "Unknown Artist") : "Various"
      const key = hasAlbum ? `${name}|||${artist}` : UNORGANIZED_KEY
      let g = map.get(key)
      if (!g) {
        g = { key, name, artist, tracks: [], firstTrackId: "", isLocal: true, firstLocalPath: t.path }
        map.set(key, g)
      }
      // Convert LocalTrack to Track shape for TrackRow
      g.tracks.push({
        id: t.path, // use path as ID for local tracks
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
      })
    }
    const unorg = map.get(UNORGANIZED_KEY)
    map.delete(UNORGANIZED_KEY)
    const groups = Array.from(map.values())
    groups.sort((a, b) => a.name.localeCompare(b.name))
    for (const g of groups) {
      g.tracks.sort((a, b) => (a.disc_number ?? 0) - (b.disc_number ?? 0) || (a.track_number ?? 0) - (b.track_number ?? 0))
    }
    if (unorg) {
      unorg.tracks.sort((a, b) => (a.title || "").localeCompare(b.title || ""))
      groups.push(unorg)
    }
    return groups
  }

  // ─── Reactive derivations ──────────────────────────────────────────────────

  $: albumGroups = activeTab === "library" ? buildAlbumGroups($tracks) : buildLocalAlbumGroups($localTracks)
  $: isLoading = activeTab === "library" ? $loading : $localLoading

  $: currentAlbumGroup = selectedAlbum
    ? albumGroups.find(g => g.key === selectedAlbum) ?? null
    : null

  // On mount, do an initial scan of saved folders
  onMount(() => {
    if ($localFolders.length > 0) {
      scanLocalFolders()
    }
  })

  // ─── Playback ──────────────────────────────────────────────────────────────

  async function playTrack(track: Track, albumTracks: Track[]) {
    const idx = albumTracks.findIndex(t => t.id === track.id)
    const queueIds = albumTracks.map(t => t.id)

    if (activeTab === "local") {
      // Local playback — just set state (Player.svelte handles audio)
      playerState.update(s => ({
        ...s,
        trackId: track.id,
        track,
        queue: queueIds,
        queueIndex: idx >= 0 ? idx : 0,
        positionMs: 0,
        paused: false,
      }))
      return
    }

    if (!$connected) return

    // Update local state immediately — no round-trip needed.
    playerState.update(s => ({
      ...s,
      trackId: track.id,
      track,
      queue: queueIds,
      queueIndex: idx >= 0 ? idx : 0,
      positionMs: 0,
      paused: false,
    }))

    // Sync server state via WS (fire-and-forget).
    wsSend("playback.queue", { device_id: "desktop", track_ids: queueIds, start_index: idx >= 0 ? idx : 0 })
    wsSend("playback.play",  { device_id: "desktop", track_id: track.id, position_ms: 0 })
  }

  function addToQueue(track: Track) {
    if (activeTab === "local") {
      // For local tracks, just append to queue
      const newQueue = [...$playerState.queue, track.id]
      playerState.update(s => ({ ...s, queue: newQueue }))
      return
    }
    if (!$connected) return
    const newQueue = [...$playerState.queue, track.id]
    wsSend("playback.queue", { device_id: "desktop", track_ids: newQueue, start_index: $playerState.queueIndex })
    playerState.update(s => ({ ...s, queue: newQueue }))
  }

  async function scanLibrary() {
    if (!$connected) return
    await serverFetch("/api/library/scan", { method: "POST" })
  }

  function goBack() {
    selectedAlbum = null
  }

  function switchTab(tab: LibTab) {
    activeTab = tab
    selectedAlbum = null
  }

  function localArtUrl(track: Track): string {
    const base = localBase()
    if (!base) return ""
    return `${base}/local/art?path=${encodeURIComponent(track.path)}`
  }

  function getArtUrl(album: AlbumGroup): string {
    if (album.isLocal && album.firstLocalPath) {
      return localArtUrl({ path: album.firstLocalPath } as Track)
    }
    return artworkUrl(album.firstTrackId)
  }

  function getTrackArtUrl(track: Track): string {
    if (activeTab === "local") {
      return localArtUrl(track)
    }
    return artworkUrl(track.id)
  }

  async function handleAddFolder() {
    await addLocalFolder()
  }

  function hideImgOnError(e: Event) {
    const img = e.currentTarget as HTMLImageElement
    if (img) img.style.display = "none"
  }
</script>

<section>
  <!-- Tab bar -->
  <div class="tab-bar">
    <button
      class="lib-tab"
      class:active={activeTab === "library"}
      on:click={() => switchTab("library")}
    >
      Library
    </button>
    <button
      class="lib-tab"
      class:active={activeTab === "local"}
      on:click={() => switchTab("local")}
    >
      Local Files
    </button>
  </div>

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
      <h2>{activeTab === "library" ? "Library" : "Local Files"}</h2>
      <div class="toolbar-actions">
        {#if activeTab === "library"}
          <button on:click={scanLibrary} title="Rescan watch folders">↺ Scan</button>
        {:else}
          <button on:click={handleAddFolder} title="Add a local music folder">+ Add Folder</button>
          {#if $localFolders.length > 0}
            <button on:click={() => scanLocalFolders()} title="Rescan local folders">↺ Rescan</button>
          {/if}
        {/if}
      </div>
    </div>

    {#if activeTab === "local" && $localFolders.length > 0}
      <div class="folder-chips">
        {#each $localFolders as dir}
          <span class="folder-chip">
            {dir.split("/").pop() || dir}
            <button class="chip-remove" on:click={() => removeLocalFolder(dir)} title="Remove folder">×</button>
          </span>
        {/each}
      </div>
    {/if}

    {#if activeTab === "library" && !$connected}
      <div class="offline-state">
        <span class="offline-icon">⚠</span>
        <p class="offline-title">{$isReconnecting ? "Reconnecting to server…" : "Not connected to a server"}</p>
        <p class="offline-sub">{$isReconnecting ? "Your library will appear once the connection is restored." : "Open Settings to connect to your pneuma server."}</p>
      </div>
    {:else if isLoading}
      <p class="text-3">Loading…</p>
    {:else if albumGroups.length === 0}
      {#if activeTab === "local"}
        <p class="text-3">No local music. Click "Add Folder" to add a music directory.</p>
      {:else}
        <p class="text-3">No tracks found. Add a watch folder in Settings and scan.</p>
      {/if}
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
                  src="{getArtUrl(album)}"
                  alt={album.name}
                  on:error={hideImgOnError}
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

  /* Tab bar */
  .tab-bar {
    display: flex;
    gap: 0;
    border-bottom: 1px solid var(--border);
    margin-bottom: 16px;
    flex-shrink: 0;
  }

  .lib-tab {
    padding: 8px 20px;
    font-size: 14px;
    color: var(--text-2);
    border-bottom: 2px solid transparent;
    transition: color 0.12s, border-color 0.12s;
  }
  .lib-tab:hover { color: var(--text-1); }
  .lib-tab.active {
    color: var(--accent);
    border-bottom-color: var(--accent);
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

  /* Folder chips */
  .folder-chips {
    display: flex;
    flex-wrap: wrap;
    gap: 6px;
    margin-bottom: 12px;
  }

  .folder-chip {
    display: inline-flex;
    align-items: center;
    gap: 4px;
    padding: 3px 10px;
    background: var(--surface-2);
    border: 1px solid var(--border);
    border-radius: 999px;
    font-size: 12px;
    color: var(--text-2);
  }

  .chip-remove {
    font-size: 14px;
    color: var(--text-3);
    padding: 0 2px;
    line-height: 1;
  }
  .chip-remove:hover { color: var(--danger); }

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

  /* Offline state */
  .offline-state {
    display: flex;
    flex-direction: column;
    align-items: center;
    justify-content: center;
    flex: 1;
    gap: 8px;
    padding: 60px 20px;
    text-align: center;
  }
  .offline-icon { font-size: 36px; opacity: 0.4; }
  .offline-title { margin: 0; font-size: 15px; font-weight: 600; color: var(--text-1); }
  .offline-sub { margin: 0; font-size: 13px; color: var(--text-3); }

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
