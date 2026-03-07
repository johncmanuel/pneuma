<script lang="ts">
  import { loading,
           remoteAlbumGroups, remoteAlbumGroupsTotal, remoteAlbumGroupsOffset,
           loadRemoteAlbumGroupsPage, loadMoreRemoteAlbumGroups,
           type RemoteAlbumGroup } from "../stores/library"
  import { localLoading, localFolders, addLocalFolder, removeLocalFolder, scanLocalFolders, checkLocalDuplicates, localDuplicates, scanningDuplicates, autoDupeCheck, localAlbumGroups, localAlbumGroupsTotal, localAlbumGroupsOffset, localAlbumFilter, loadLocalAlbumGroups, loadMoreLocalAlbumGroups, fetchLocalAlbumTracks, type LocalAlbumGroup } from "../stores/localLibrary"
  import { playerState } from "../stores/player"
  import TrackRow from "./TrackRow.svelte"
  import Duplicates from "./Duplicates.svelte"
  import type { Track } from "../stores/player"
  import type { LocalTrack } from "../stores/localLibrary"
  import { serverFetch, artworkUrl, connected, isReconnecting, localBase } from "../utils/api"
  import { wsSend } from "../stores/ws"
  import { onMount } from "svelte"
  import { activeTab, localSubTab, selectedAlbum, pushNav, type LibTab } from "../stores/ui"
  import { get, derived } from "svelte/store"
  import { recordRecentAlbum } from "../stores/recentAlbums"
  import { createVirtualizer } from "@tanstack/svelte-virtual"

  const currentTrackId = derived(playerState, $s => $s.trackId);

  // ─── Album detail: filter & sort ────────────────────────────────────────────
  let albumFilter = ""
  let trackListEl: HTMLDivElement
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

  // ─── Album group types (unified for local and remote display) ───────────────

  interface AlbumGroup {
    key: string
    name: string
    artist: string
    trackCount: number
    firstTrackId: string  // for remote album art
    isLocal?: boolean
    firstLocalPath?: string // for local art
  }

  const UNORGANIZED_KEY = "__unorganized__"

  // ─── Album detail state (on-demand loaded tracks) ───────────────────────────

  let currentAlbumGroup: AlbumGroup | null = null
  let albumDetailTracks: Track[] = []
  let albumDetailLoading = false

  // Convert local album groups to AlbumGroup shape
  function localGroupsAsAlbumGroups(groups: LocalAlbumGroup[]): AlbumGroup[] {
    return (groups ?? []).map(g => ({
      key: g.key,
      name: g.name,
      artist: g.artist,
      trackCount: g.track_count,
      firstTrackId: "",
      isLocal: true,
      firstLocalPath: g.first_track_path,
    }))
  }

  // Convert remote album groups (from /albumgroups endpoint) to AlbumGroup shape
  function remoteGroupsAsAlbumGroups(groups: RemoteAlbumGroup[]): AlbumGroup[] {
    return (groups ?? []).map(g => ({
      key: g.key,
      name: g.name || "Unknown Album",
      artist: g.artist || "Unknown Artist",
      trackCount: g.track_count,
      firstTrackId: g.first_track_id,
      isLocal: false,
      firstLocalPath: "",
    }))
  }

  // Convert a LocalTrack to the Track shape used by TrackRow
  function localTrackToTrack(t: LocalTrack): Track {
    return {
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
    }
  }

  // ─── Reactive derivations ──────────────────────────────────────────────────

  $: displayedGroups = $activeTab === "library"
    ? remoteGroupsAsAlbumGroups($remoteAlbumGroups)
    : localGroupsAsAlbumGroups($localAlbumGroups)
  $: isLoading = $activeTab === "library" ? $loading : $localLoading
  $: localDupeCount = $localDuplicates.length
  $: currentTotal = $activeTab === "library" ? $remoteAlbumGroupsTotal : $localAlbumGroupsTotal
  $: currentOffset = $activeTab === "library" ? $remoteAlbumGroupsOffset : $localAlbumGroupsOffset
  $: hasMore = displayedGroups.length < currentTotal

  // Load the selected album's tracks on demand
  $: if ($selectedAlbum && !albumDetailLoading) {
    const group = displayedGroups.find(g => g.key === $selectedAlbum) ?? null
    if (group && (!currentAlbumGroup || currentAlbumGroup.key !== group.key)) {
      loadAlbumDetail(group)
    }
  }

  // Clear album detail when deselecting
  $: if (!$selectedAlbum) {
    currentAlbumGroup = null
    albumDetailTracks = []
    albumFilter = ""
    albumSortField = "default"
    albumSortDir = "asc"
  }

  async function loadAlbumDetail(group: AlbumGroup) {
    albumDetailLoading = true
    currentAlbumGroup = group
    albumFilter = ""
    albumSortField = "default"
    albumSortDir = "asc"
    try {
      if (group.isLocal) {
        // Parse albumName and albumArtist from the key
        let albumName: string, albumArtist: string
        if (group.key === UNORGANIZED_KEY) {
          albumName = ""
          albumArtist = ""
        } else {
          const parts = group.key.split("|||")
          albumName = parts[0] ?? ""
          albumArtist = parts[1] ?? ""
        }
        const locals = await fetchLocalAlbumTracks(albumName, albumArtist)
        albumDetailTracks = locals.map(localTrackToTrack)
        // Update track count with the actual number
        if (currentAlbumGroup) {
          currentAlbumGroup = { ...currentAlbumGroup, trackCount: albumDetailTracks.length }
        }
      } else {
        // Remote: fetch tracks for this album by album_name + album_artist
        let albumName: string, albumArtist: string
        if (group.key === UNORGANIZED_KEY) {
          albumName = ""
          albumArtist = ""
        } else {
          const parts = group.key.split("|||")
          albumName = parts[0] ?? ""
          albumArtist = parts[1] ?? ""
        }
        const params = new URLSearchParams()
        params.set("album_name", albumName)
        if (albumArtist) params.set("album_artist", albumArtist)
        const r = await serverFetch(`/api/library/tracks?${params}`)
        const data = await r.json()
        const fetched: Track[] = Array.isArray(data) ? data : (data.tracks ?? [])
        albumDetailTracks = fetched
          .sort((a, b) => (a.disc_number ?? 0) - (b.disc_number ?? 0) || (a.track_number ?? 0) - (b.track_number ?? 0))
        if (currentAlbumGroup) {
          currentAlbumGroup = { ...currentAlbumGroup, trackCount: albumDetailTracks.length }
        }
      }
    } catch (e) {
      console.warn("Failed to load album detail:", e)
      albumDetailTracks = []
    } finally {
      albumDetailLoading = false
    }
  }

  // Filtered + sorted tracks for album detail (client-side on the small album track set)
  $: filteredAlbumDetailTracks = (() => {
    if (!currentAlbumGroup) return []
    const f = albumFilter.toLowerCase()
    let result = albumDetailTracks
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

  // Virtualized track list for album detail view
  $: virtualizer = createVirtualizer<HTMLDivElement, HTMLDivElement>({
    count: filteredAlbumDetailTracks.length,
    getScrollElement: () => trackListEl,
    estimateSize: () => 38,
    overscan: 5,
  })

  // On mount, load album groups and optionally check duplicates.
  // Library is destroyed/re-created on every view switch (App.svelte uses {#if}),
  // so onMount fires each time the user navigates back. Only do a full
  // scanLocalFolders() on the first ever mount (store still empty); on return
  // visits just refresh the paginated view from the already-populated SQLite
  // table, which is instant and doesn't re-read the file system.
  onMount(() => {
    if ($activeTab === "library") {
      loadRemoteAlbumGroupsPage(0)
    }
    if ($localFolders.length > 0) {
      if (get(localAlbumGroups).length === 0) {
        // First mount — populate the SQLite cache with a full scan.
        scanLocalFolders()
        if ($autoDupeCheck) checkLocalDuplicates()
      } else {
        // Returning from another view — data is in memory, just reload the page.
        loadLocalAlbumGroups(0, get(localAlbumFilter))
      }
    }
  })

  // ─── Debounced album grid filter ───────────────────────────────────────────

  let gridFilterDebounce: ReturnType<typeof setTimeout>

  function onAlbumGridFilterInput() {
    clearTimeout(gridFilterDebounce)
    gridFilterDebounce = setTimeout(() => {
      const q = albumGridFilter.trim()
      if ($activeTab === "library") {
        loadRemoteAlbumGroupsPage(0, q)
      } else {
        localAlbumFilter.set(q)
        loadLocalAlbumGroups(0, q)
      }
    }, 300)
  }

  function clearAlbumGridFilter() {
    albumGridFilter = ""
    if ($activeTab === "library") {
      loadRemoteAlbumGroupsPage(0)
    } else {
      localAlbumFilter.set("")
      loadLocalAlbumGroups(0, "")
    }
  }

  // ─── Infinite scroll for album grid ────────────────────────────────────────

  let gridScrollEl: HTMLDivElement
  let loadingMore = false

  function handleGridScroll() {
    if (loadingMore || !hasMore || !gridScrollEl) return
    const { scrollTop, scrollHeight, clientHeight } = gridScrollEl
    if (scrollTop + clientHeight >= scrollHeight - 200) {
      loadMorePage()
    }
  }

  async function loadMorePage() {
    loadingMore = true
    try {
      if ($activeTab === "library") {
        await loadMoreRemoteAlbumGroups(albumGridFilter.trim())
      } else {
        await loadMoreLocalAlbumGroups(get(localAlbumFilter))
      }
    } finally {
      loadingMore = false
    }
  }

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

  function switchTab(tab: LibTab) {
    albumGridFilter = ""
    pushNav({ tab, albumKey: null, subTab: "albums" })
    // Reload album groups for the new tab
    if (tab === "library") {
      loadRemoteAlbumGroupsPage(0)
    } else if (tab === "local") {
      loadLocalAlbumGroups(0, "")
    }
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

  function handlePlay(e: CustomEvent<Track>) {
    playTrack(e.detail, filteredAlbumDetailTracks)
  }

  function handleQueue(e: CustomEvent<Track>) {
    addToQueue(e.detail)
  }

  let isScrolling = false
  let scrollTimer: ReturnType<typeof setTimeout>

  function handleScroll() {
    isScrolling = true
    clearTimeout(scrollTimer)
    scrollTimer = setTimeout(() => { isScrolling = false }, 150)
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
      <div class="album-detail-view">
        <div class="album-detail-header">
          <div class="album-art-hero">
            {#if selectedAlbumArtUrl}
                <img src={selectedAlbumArtUrl} alt={currentAlbumGroup.name} on:error={hideImgOnError} loading="lazy" decoding="async" />
            {/if}
            <div class="album-art-hero-placeholder">♫</div>
          </div>
          <div class="album-detail-info">
            <h2 class="album-detail-title">{currentAlbumGroup.name}</h2>
      <p class="album-meta text-2">{currentAlbumGroup.artist} · {currentAlbumGroup.trackCount} tracks</p>
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

        <div class="track-header album-detail-cols">
          <span>#</span>
          <button class="col-sort" on:click={() => toggleSort("title")}>Title{sortIndicator("title")}</button>
          <button class="col-sort" on:click={() => toggleSort("artist")}>Artist{sortIndicator("artist")}</button>
          <button class="col-sort" on:click={() => toggleSort("duration")}>Duration{sortIndicator("duration")}</button>
        </div>

        {#if albumFilter && filteredAlbumDetailTracks.length === 0}
          <p class="no-results text-3">No songs match "{albumFilter}"</p>
        {:else}
          <div class="track-list" class:scrolling={isScrolling} bind:this={trackListEl} on:scroll={handleScroll}>
            <div style="position: relative; width: 100%; height: {$virtualizer.getTotalSize()}px;">
              {#each $virtualizer.getVirtualItems() as row (row.index)}
                <div
                  class="virtual-row"
                  style="height: {row.size}px; transform: translateY({row.start}px);"
                >
                  <TrackRow
                    track={filteredAlbumDetailTracks[row.index]}
                    hideAlbum={true}
                    active={$currentTrackId === filteredAlbumDetailTracks[row.index]?.id}
                    on:play={handlePlay}
                    on:select={() => {}}
                    on:addToQueue={handleQueue}
                  />
                </div>
              {/each}
            </div>
          </div>
        {/if}
      </div>
  {:else}
    <!-- Album grid view -->
    <div class="grid-scroll-wrapper" bind:this={gridScrollEl} on:scroll={handleGridScroll}>
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
        on:input={onAlbumGridFilterInput}
      />
      {#if albumGridFilter}
        <button class="grid-filter-clear" on:click={clearAlbumGridFilter}>×</button>
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
        <p class="offline-title">{$isReconnecting ? "Reconnecting to server..." : "Not connected to a server"}</p>
        <p class="offline-sub">{$isReconnecting ? "Your library will appear once the connection is restored." : "Open Settings to connect to your pneuma server."}</p>
      </div>
    {:else if isLoading}
      <p class="text-3">Loading…</p>
    {:else if displayedGroups.length === 0}
      {#if $activeTab === "local"}
        <p class="text-3">No local music. Click "Add Folder" to add a music directory.</p>
      {:else}
        <p class="text-3">No tracks found. Add a watch folder in Settings and scan.</p>
      {/if}
    {:else}
      {#if displayedGroups.length === 0}
        <p class="text-3">No albums match "{albumGridFilter}"</p>
      {:else}
      <div class="album-grid">
        {#each displayedGroups as album (album.key)}
          <button
            class="album-card"
            class:unorganized={album.key === UNORGANIZED_KEY}
            on:click={() => openAlbum(album)}
          >
            <div class="album-art" class:unorg-art={album.key === UNORGANIZED_KEY}>
              {#if album.key !== UNORGANIZED_KEY}
                  <img src={getArtUrl(album)} alt={album.name} on:error={hideImgOnError} loading="lazy"/>
              {/if}
              <div class="album-art-placeholder">{album.key === UNORGANIZED_KEY ? "📂" : "♫"}</div>
            </div>
            <p class="album-title truncate" class:unorg-title={album.key === UNORGANIZED_KEY}>{album.name}</p>
            <p class="album-artist truncate text-3">{album.artist} · {album.trackCount} tracks</p>
          </button>
        {/each}
      </div>
      {#if hasMore}
        <p class="text-3" style="text-align:center;padding:12px;">Loading more…</p>
      {/if}
    {/if}
  {/if}

  </div> <!-- /.grid-scroll-wrapper -->
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

  /* Scrollable body — flex container for album detail or grid */
  .scroll-body {
    flex: 1;
    min-height: 0;
    overflow: hidden;
    display: flex;
    flex-direction: column;
    padding: 16px 16px 0 0;
  }

  /* Album detail view — flex column so track-list fills remaining space */
  .album-detail-view {
    flex: 1;
    display: flex;
    flex-direction: column;
    min-height: 0;
    overflow: hidden;
  }

  /* Grid scroll wrapper — owns vertical scroll for album grid view */
  .grid-scroll-wrapper {
    flex: 1;
    min-height: 0;
    overflow-y: auto;
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

  /* Virtualized track list within album detail */
  .track-list {
    flex: 1;
    min-height: 0;
    overflow-y: auto;
  }

  .virtual-row {
    position: absolute;
    top: 0;
    left: 0;
    width: 100%;
    contain: layout paint;
  }

  .track-list.scrolling .virtual-row {
    pointer-events: none;
  }

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
