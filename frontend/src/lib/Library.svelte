<script lang="ts">
  import { tracks, loading } from "../stores/library"
  import { localTracks, localLoading, localFolders, addLocalFolder, removeLocalFolder, scanLocalFolders, localDuplicates, scanningDuplicates, autoDupeCheck } from "../stores/localLibrary"
  import { playerState } from "../stores/player"
  import TrackRow from "./TrackRow.svelte"
  import Duplicates from "./Duplicates.svelte"
  import type { Track } from "../stores/player"
  import { serverFetch, artworkUrl, connected, isReconnecting, localBase } from "./api"
  import { wsSend } from "../stores/ws"
  import { onMount } from "svelte"
  import { activeTab, localSubTab, selectedAlbum, pushNav, type LibTab } from "../stores/ui"
  import { get } from "svelte/store"
  import { recordRecentAlbum } from "../stores/recentAlbums"
  import { cachedArtUrl } from "../stores/artCache"

  // ─── Album detail: filter & sort ────────────────────────────────────────────
  let albumFilter = ""
  let albumGridFilter = ""
  type SortField = "default" | "title" | "artist" | "duration"
  let albumSortField: SortField = "default"
  let albumSortDir: "asc" | "desc" = "asc"

  function toggleSort(field: SortField) {
    if (albumSortField === field) {
      albumSortDir = albumSortDir === "asc" ? "desc" : "asc"
    } else {
      albumSortField = field
      albumSortDir = "asc"
    }
  }

  function sortIndicator(field: SortField): string {
    return albumSortField === field ? (albumSortDir === "asc" ? " ↑" : " ↓") : ""
  }

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

  $: albumGroups = $activeTab === "library" ? buildAlbumGroups($tracks) : buildLocalAlbumGroups($localTracks)
  $: isLoading = $activeTab === "library" ? $loading : $localLoading
  $: localDupeCount = $localDuplicates.length

  $: filteredAlbumGroups = (() => {
    if (!albumGridFilter.trim()) return albumGroups
    const f = albumGridFilter.toLowerCase()
    return albumGroups.filter(g =>
      g.name.toLowerCase().includes(f) || g.artist.toLowerCase().includes(f)
    )
  })()

  $: currentAlbumGroup = $selectedAlbum
    ? albumGroups.find(g => g.key === $selectedAlbum) ?? null
    : null

  // Reset filter/sort when switching albums
  $: if ($selectedAlbum) { albumFilter = ""; albumSortField = "default"; albumSortDir = "asc" }

  // Filtered + sorted tracks for album detail
  $: albumDetailTracks = (() => {
    if (!currentAlbumGroup) return []
    const f = albumFilter.toLowerCase()
    let result = currentAlbumGroup.tracks
    if (f) {
      result = result.filter(t =>
        (t.title ?? "").toLowerCase().includes(f) ||
        (t.artist_name ?? "").toLowerCase().includes(f)
      )
    }
    if (albumSortField !== "default") {
      const dir = albumSortDir === "asc" ? 1 : -1
      result = [...result].sort((a, b) => {
        if (albumSortField === "title") return dir * (a.title ?? "").localeCompare(b.title ?? "")
        if (albumSortField === "artist") return dir * (a.artist_name ?? "").localeCompare(b.artist_name ?? "")
        if (albumSortField === "duration") return dir * ((a.duration_ms ?? 0) - (b.duration_ms ?? 0))
        return 0
      })
    }
    return result
  })()

  // Album artwork for the detail header
  $: selectedAlbumArtUrl = (() => {
    if (!currentAlbumGroup) return ""
    return getArtUrl(currentAlbumGroup)
  })()

  // On mount, always load local tracks if the user has saved folders.
  // scanLocalFolders() restores from localStorage cache immediately (fast path)
  // and then kicks off a background rescan.
  // The duplicate check (fingerprinting) is gated on the autoDupeCheck preference.
  onMount(() => {
    if ($localFolders.length > 0) {
      scanLocalFolders({ checkDuplicates: $autoDupeCheck })
    }
  })

  // ─── Playback ──────────────────────────────────────────────────────────────

  async function playTrack(track: Track, albumTracks: Track[]) {
    const idx = albumTracks.findIndex(t => t.id === track.id)
    const queueIds = albumTracks.map(t => t.id)

    if (currentAlbumGroup) {
      recordRecentAlbum({
        key: currentAlbumGroup.key,
        name: currentAlbumGroup.name,
        artist: currentAlbumGroup.artist,
        isLocal: currentAlbumGroup.isLocal ?? false,
        firstTrackId: currentAlbumGroup.firstTrackId,
        firstLocalPath: currentAlbumGroup.firstLocalPath ?? "",
      })
    }

    if (get(activeTab) === "local") {
      // Local playback — just set state (Player.svelte handles audio)
      playerState.update(s => ({
        ...s,
        trackId: track.id,
        track,
        queue: queueIds,
        baseQueue: queueIds,
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
      baseQueue: queueIds,
      queueIndex: idx >= 0 ? idx : 0,
      positionMs: 0,
      paused: false,
    }))

    // Sync server state via WS (fire-and-forget).
    wsSend("playback.queue", { device_id: "desktop", track_ids: queueIds, start_index: idx >= 0 ? idx : 0 })
    wsSend("playback.play",  { device_id: "desktop", track_id: track.id, position_ms: 0 })
  }

  function addToQueue(track: Track) {
    // Insert directly after the currently playing track (Spotify-style),
    // not at the end. Do NOT send playback.queue to the server because
    // SetQueue resets PositionMS=0, which would interrupt playback.
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

  async function scanLibrary() {
    if (!$connected) return
    await serverFetch("/api/library/scan", { method: "POST" })
  }

  function goBack() {
    pushNav({ albumKey: null })
  }

  function switchTab(tab: LibTab) {
    albumGridFilter = ""
    pushNav({ tab, albumKey: null, subTab: "albums" })
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
    if (get(activeTab) === "local") {
      return localArtUrl(track)
    }
    return artworkUrl(track.id)
  }

  function openAlbum(album: AlbumGroup) {
    pushNav({ albumKey: album.key })
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
  <!-- Main tab bar -->
  <div class="tab-bar">
    <button
      class="lib-tab"
      class:active={$activeTab === "library"}
      on:click={() => switchTab("library")}
    >
      Library
    </button>
    <button
      class="lib-tab"
      class:active={$activeTab === "local"}
      on:click={() => switchTab("local")}
    >
      Local Files
    </button>
  </div>

  <!-- Sub-tab bar (Local Files only) — always visible, never scrolls away -->
  {#if $activeTab === "local"}
    <div class="subtab-bar">
      <button
        class="subtab"
        class:active={$localSubTab === "albums"}
        on:click={() => pushNav({ subTab: "albums", albumKey: null })}
      >Albums</button>
      <button
        class="subtab"
        class:active={$localSubTab === "duplicates"}
        on:click={() => pushNav({ subTab: "duplicates" })}
      >
        Duplicates
        {#if localDupeCount > 0}<span class="dupe-badge">{localDupeCount}</span>{/if}
        {#if $scanningDuplicates}<span class="scan-dot" title="Scanning…"></span>{/if}
      </button>
    </div>
  {/if}

  <!-- Scrollable body -->
  <div class="scroll-body">

    {#if $activeTab === "local" && $localSubTab === "duplicates"}
      <Duplicates />

    {:else if currentAlbumGroup}
      <!-- Album detail view -->
      <div class="album-detail-header">
        <div class="album-art-hero">
          {#if selectedAlbumArtUrl}
            {#await cachedArtUrl(currentAlbumGroup.key, selectedAlbumArtUrl) then blobUrl}
              <img src={blobUrl} alt={currentAlbumGroup.name} on:error={hideImgOnError} />
            {/await}
          {/if}
          <div class="album-art-hero-placeholder">♫</div>
        </div>
        <div class="album-detail-info">
          <h2 class="album-detail-title">{currentAlbumGroup.name}</h2>
          <p class="album-meta text-2">{currentAlbumGroup.artist} · {currentAlbumGroup.tracks.length} tracks</p>
          <div class="album-filter-bar">
            <input
              type="search"
              class="album-filter-input"
              placeholder="Filter songs…"
              bind:value={albumFilter}
            />
          </div>
        </div>
      </div>

      <div class="track-list">
        <div class="track-header album-detail-cols">
          <span>#</span>
          <button class="col-sort" on:click={() => toggleSort("title")}>Title{sortIndicator("title")}</button>
          <button class="col-sort" on:click={() => toggleSort("artist")}>Artist{sortIndicator("artist")}</button>
          <button class="col-sort" on:click={() => toggleSort("duration")}>Duration{sortIndicator("duration")}</button>
        </div>
        {#each albumDetailTracks as track (track.id)}
          <TrackRow
            {track}
            hideAlbum={true}
            active={$playerState.trackId === track.id}
            on:play={() => playTrack(track, albumDetailTracks)}
          on:select={() => {}}
          on:addToQueue={() => addToQueue(track)}
        />
      {/each}
      {#if albumFilter && albumDetailTracks.length === 0}
        <p class="no-results text-3">No songs match "{albumFilter}"</p>
      {/if}
    </div>
  {:else}
    <!-- Album grid view -->
    <div class="toolbar">
      <h2>{$activeTab === "library" ? "Library" : "Local Files"}</h2>
      <div class="toolbar-actions">
        {#if $activeTab === "library"}
          <button on:click={scanLibrary} title="Rescan watch folders">↺ Scan</button>
        {:else}
          <button on:click={handleAddFolder} title="Add a local music folder">+ Add Folder</button>
          {#if $localFolders.length > 0}
            <button on:click={() => scanLocalFolders()} title="Rescan local folders">↺ Rescan</button>
          {/if}
        {/if}
      </div>
    </div>

    <div class="album-grid-search">
      <svg class="grid-search-icon" viewBox="0 0 24 24" width="14" height="14" fill="none" stroke="currentColor" stroke-width="2"><circle cx="11" cy="11" r="8"/><path d="M21 21l-4.35-4.35"/></svg>
      <input
        type="search"
        class="album-grid-filter"
        placeholder="Search albums…"
        bind:value={albumGridFilter}
      />
      {#if albumGridFilter}
        <button class="grid-filter-clear" on:click={() => albumGridFilter = ""}>×</button>
      {/if}
    </div>

    {#if $activeTab === "local" && $localFolders.length > 0}
      <div class="folder-chips">
        {#each $localFolders as dir}
          <span class="folder-chip">
            {dir.split("/").pop() || dir}
            <button class="chip-remove" on:click={() => removeLocalFolder(dir)} title="Remove folder">×</button>
          </span>
        {/each}
      </div>
    {/if}

    {#if $activeTab === "library" && !$connected}
      <div class="offline-state">
        <span class="offline-icon">⚠</span>
        <p class="offline-title">{$isReconnecting ? "Reconnecting to server…" : "Not connected to a server"}</p>
        <p class="offline-sub">{$isReconnecting ? "Your library will appear once the connection is restored." : "Open Settings to connect to your pneuma server."}</p>
      </div>
    {:else if isLoading}
      <p class="text-3">Loading…</p>
    {:else if albumGroups.length === 0}
      {#if $activeTab === "local"}
        <p class="text-3">No local music. Click "Add Folder" to add a music directory.</p>
      {:else}
        <p class="text-3">No tracks found. Add a watch folder in Settings and scan.</p>
      {/if}
    {:else}
      {#if filteredAlbumGroups.length === 0}
        <p class="text-3">No albums match "{albumGridFilter}"</p>
      {:else}
      <div class="album-grid">
        {#each filteredAlbumGroups as album (album.key)}
          <button
            class="album-card"
            class:unorganized={album.key === UNORGANIZED_KEY}
            on:click={() => openAlbum(album)}
          >
            <div class="album-art" class:unorg-art={album.key === UNORGANIZED_KEY}>
              {#if album.key !== UNORGANIZED_KEY}
                {#await cachedArtUrl(album.key, getArtUrl(album)) then blobUrl}
                  <img src={blobUrl} alt={album.name} on:error={hideImgOnError} />
                {/await}
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

  {/if} <!-- /.scroll-body inner if -->

  </div> <!-- /.scroll-body -->
</section>

<style>
  section {
    display: flex;
    flex-direction: column;
    height: 100%;
    overflow: hidden;
  }

  /* Main tab bar */
  .tab-bar {
    display: flex;
    gap: 0;
    border-bottom: 1px solid var(--border);
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

  /* Sub-tab bar (Local Files) */
  .subtab-bar {
    display: flex;
    gap: 0;
    border-bottom: 1px solid var(--border);
    background: var(--surface);
    flex-shrink: 0;
    padding: 0 4px;
  }

  .subtab {
    display: flex;
    align-items: center;
    gap: 6px;
    padding: 6px 14px;
    font-size: 12px;
    color: var(--text-2);
    border-bottom: 2px solid transparent;
    transition: color 0.1s, border-color 0.1s;
  }
  .subtab:hover { color: var(--text-1); }
  .subtab.active { color: var(--text-1); border-bottom-color: var(--accent); }

  .dupe-badge {
    display: inline-block;
    min-width: 16px;
    padding: 0 4px;
    font-size: 10px;
    font-weight: 700;
    line-height: 16px;
    text-align: center;
    border-radius: 8px;
    background: var(--danger);
    color: #fff;
  }

  .scan-dot {
    display: inline-block;
    width: 6px;
    height: 6px;
    border-radius: 50%;
    background: var(--accent);
    animation: pulse 1s ease-in-out infinite;
  }
  @keyframes pulse {
    0%, 100% { opacity: 1; }
    50% { opacity: 0.3; }
  }

  /* Scrollable body — owns all vertical scroll */
  .scroll-body {
    flex: 1;
    min-height: 0;
    overflow-y: auto;
    padding: 16px 16px 0 0;
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

  .album-meta {
    font-size: 13px;
    margin: 0;
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

  /* Album grid filter bar */
  .album-grid-search {
    display: flex;
    align-items: center;
    gap: 8px;
    background: var(--surface);
    border: 1px solid var(--border);
    border-radius: 16px;
    padding: 5px 12px;
    margin-bottom: 16px;
    max-width: 280px;
    transition: border-color 0.15s;
  }
  .album-grid-search:focus-within { border-color: var(--accent); }

  .grid-search-icon { color: var(--text-3); flex-shrink: 0; }

  .album-grid-filter {
    flex: 1;
    background: none;
    border: none;
    color: var(--fg);
    font-size: 13px;
    outline: none;
    padding: 0;
  }
  .album-grid-filter::placeholder { color: var(--text-3); }
  .album-grid-filter::-webkit-search-cancel-button { display: none; }

  .grid-filter-clear {
    background: none;
    border: none;
    color: var(--text-3);
    font-size: 16px;
    cursor: pointer;
    padding: 0;
    line-height: 1;
  }
  .grid-filter-clear:hover { color: var(--fg); }

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

  /* Album detail columns — no Album column */
  .track-header.album-detail-cols {
    grid-template-columns: 32px 2fr 1fr 56px;
  }

  .col-sort {
    background: none;
    border: none;
    color: var(--fg-3);
    font-size: 11px;
    text-transform: uppercase;
    letter-spacing: 0.05em;
    cursor: pointer;
    padding: 0;
    text-align: left;
    white-space: nowrap;
  }
  .col-sort:hover { color: var(--text-1); }

  /* Album detail header with artwork */
  .album-detail-header {
    display: flex;
    flex-direction: row;
    align-items: flex-start;
    gap: 20px;
    margin-bottom: 20px;
  }

  .album-detail-info {
    display: flex;
    flex-direction: column;
    justify-content: flex-end;
    flex: 1;
    min-width: 0;
    padding-bottom: 4px;
  }

  .album-art-hero {
    width: 160px;
    height: 160px;
    flex-shrink: 0;
    border-radius: 8px;
    overflow: hidden;
    background: var(--surface);
    display: flex;
    align-items: center;
    justify-content: center;
    position: relative;
  }

  .album-art-hero img {
    width: 100%;
    height: 100%;
    object-fit: cover;
    position: relative;
    z-index: 1;
  }

  .album-art-hero-placeholder {
    position: absolute;
    font-size: 48px;
    color: var(--text-3);
  }

  .album-detail-title {
    margin: 0 0 4px;
    font-size: 20px;
    font-weight: 700;
  }

  /* Album filter bar */
  .album-filter-bar {
    margin-top: 12px;
  }

  .album-filter-input {
    width: 100%;
    max-width: 280px;
    padding: 6px 12px;
    background: var(--surface);
    border: 1px solid var(--border);
    border-radius: 16px;
    color: var(--fg);
    font-size: 13px;
    outline: none;
    transition: border-color 0.15s;
  }
  .album-filter-input:focus { border-color: var(--accent); }
  .album-filter-input::placeholder { color: var(--text-3); }
  /* hide default search clear button */
  .album-filter-input::-webkit-search-cancel-button { display: none; }

  .no-results {
    padding: 16px 8px;
    font-size: 13px;
  }
</style>
