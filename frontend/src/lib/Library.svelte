<script lang="ts">
  import {
    loading,
    remoteAlbumGroups,
    remoteAlbumGroupsTotal,
    loadRemoteAlbumGroupsPage,
    loadMoreRemoteAlbumGroups,
    type RemoteAlbumGroup,
    UNORGANIZED_KEY
  } from "../stores/library";
  import {
    localLoading,
    localFolders,
    addLocalFolder,
    removeLocalFolder,
    scanLocalFolders,
    localAlbumGroups,
    localAlbumGroupsTotal,
    localAlbumFilter,
    loadLocalAlbumGroups,
    loadMoreLocalAlbumGroups,
    fetchLocalAlbumTracks,
    localChangeSeq,
    localTrackToTrack,
    scanProgress,
    type LocalAlbumGroup
  } from "../stores/localLibrary";
  import { playerState } from "../stores/player";
  import { totalDuration } from "../utils";
  import { shuffle } from "../utils/algos";
  import TrackRow from "./TrackRow.svelte";
  import SortButton from "./SortButton.svelte";
  import "../assets/css/track-list.css";

  import type { Track } from "../stores/player";
  import {
    serverFetch,
    artworkUrl,
    connected,
    isReconnecting,
    localBase
  } from "../utils/api";
  import { wsSend } from "../stores/ws";
  import { onMount } from "svelte";
  import { activeTab, selectedAlbum, pushNav, type LibTab } from "../stores/ui";
  import { get, derived } from "svelte/store";
  import { recordRecentAlbum } from "../stores/recentAlbums";
  import { createVirtualizer } from "@tanstack/svelte-virtual";
  import {
    playlists,
    addTracksToPlaylist,
    type PlaylistSummary
  } from "../stores/playlists";
  import {
    Music,
    FolderOpen,
    RotateCcw,
    X,
    TriangleAlert,
    ChevronRight,
    Search
  } from "@lucide/svelte";
  import { portal } from "../utils/dom";

  const currentTrackId = derived(playerState, ($s) => $s.trackId);

  let albumFilter = "";
  let trackListEl: HTMLDivElement;
  let albumGridFilter = "";

  type SortField = "default" | "title" | "artist" | "duration";
  type SortDir = "asc" | "desc";

  let albumSortField: SortField = "default";
  let albumSortDir: SortDir = "asc";

  // Library UI representation of an album group (local or remote)
  interface AlbumGroup {
    key: string;
    name: string;
    artist: string;
    trackCount: number;
    firstTrackId: string; // for remote album art
    isLocal?: boolean;
    firstLocalPath?: string; // for local art
  }

  let currentAlbumGroup: AlbumGroup | null = null;
  let albumDetailTracks: Track[] = [];
  let albumDetailLoading = false;

  function localGroupsAsAlbumGroups(groups: LocalAlbumGroup[]): AlbumGroup[] {
    return (groups ?? []).map((g) => ({
      key: g.key,
      name: g.name || "Unknown Album",
      artist: g.artist || "Unknown Artist",
      trackCount: g.track_count,
      firstTrackId: "",
      isLocal: true,
      firstLocalPath: g.first_track_path
    }));
  }

  function remoteGroupsAsAlbumGroups(groups: RemoteAlbumGroup[]): AlbumGroup[] {
    return (groups ?? []).map((g) => ({
      key: g.key,
      name: g.name || "Unknown Album",
      artist: g.artist || "Unknown Artist",
      trackCount: g.track_count,
      firstTrackId: g.first_track_id,
      isLocal: false,
      firstLocalPath: ""
    }));
  }

  // Display groups based on the current tab
  $: displayedGroups =
    $activeTab === "library"
      ? remoteGroupsAsAlbumGroups($remoteAlbumGroups)
      : localGroupsAsAlbumGroups($localAlbumGroups);
  $: isLoading = $activeTab === "library" ? $loading : $localLoading;
  $: currentTotal =
    $activeTab === "library" ? $remoteAlbumGroupsTotal : $localAlbumGroupsTotal;
  $: hasMore = displayedGroups.length < currentTotal;

  // Load the selected album's tracks whenever the selected album changes
  $: if ($selectedAlbum && !albumDetailLoading) {
    const group = displayedGroups.find((g) => g.key === $selectedAlbum) ?? null;
    if (group && (!currentAlbumGroup || currentAlbumGroup.key !== group.key)) {
      loadAlbumDetail(group);
    }
  }

  // Clear album detail when deselecting an album
  $: if (!$selectedAlbum) {
    currentAlbumGroup = null;
    albumDetailTracks = [];
    albumFilter = "";
    albumSortField = "default";
    albumSortDir = "asc";
  }

  // Re-fetch the open local album's track list whenever a file change is detected.
  $: if ($localChangeSeq && currentAlbumGroup?.isLocal) {
    refreshCurrentAlbumDetail();
  }

  function getAlbumNameAndArtist(group: AlbumGroup): {
    albumName: string;
    albumArtist: string;
  } {
    if (group.key === UNORGANIZED_KEY) {
      return { albumName: "", albumArtist: "" };
    }
    const parts = group.key.split("|||");
    return { albumName: parts[0] ?? "", albumArtist: parts[1] ?? "" };
  }

  // Refresh the current album's track list to reflect local file changes
  async function refreshCurrentAlbumDetail() {
    if (!currentAlbumGroup?.isLocal) return;

    const { albumName, albumArtist } = getAlbumNameAndArtist(currentAlbumGroup);

    const locals = await fetchLocalAlbumTracks(albumName, albumArtist);
    albumDetailTracks = locals.map(localTrackToTrack);
    if (currentAlbumGroup) {
      currentAlbumGroup = {
        ...currentAlbumGroup,
        trackCount: albumDetailTracks.length
      };
    }
  }

  async function loadAlbumDetail(group: AlbumGroup) {
    albumDetailLoading = true;
    currentAlbumGroup = group;
    albumFilter = "";
    albumSortField = "default";
    albumSortDir = "asc";

    try {
      const { albumName, albumArtist } = getAlbumNameAndArtist(group);

      if (group.isLocal) {
        const locals = await fetchLocalAlbumTracks(albumName, albumArtist);
        albumDetailTracks = locals.map(localTrackToTrack);
      } else {
        // Remote: fetch tracks for this album by album_name + album_artist
        const params = new URLSearchParams();
        params.set("album_name", albumName);

        if (albumArtist) params.set("album_artist", albumArtist);

        const r = await serverFetch(`/api/library/tracks?${params}`);
        const data = await r.json();
        const fetched: Track[] = Array.isArray(data)
          ? data
          : (data.tracks ?? []);

        albumDetailTracks = fetched.sort(
          (a, b) =>
            (a.disc_number ?? 0) - (b.disc_number ?? 0) ||
            (a.track_number ?? 0) - (b.track_number ?? 0)
        );

        if (currentAlbumGroup) {
          currentAlbumGroup = {
            ...currentAlbumGroup,
            trackCount: albumDetailTracks.length
          };
        }
      }
    } catch (e) {
      console.warn("Failed to load album detail:", e);
      albumDetailTracks = [];
    } finally {
      albumDetailLoading = false;
    }
  }

  // Filtered + sorted tracks for album detail (client-side on the small album track set)
  $: filteredAlbumDetailTracks = (() => {
    if (!currentAlbumGroup) return [];

    const f = albumFilter.toLowerCase();
    let result = albumDetailTracks;

    if (f) {
      result = result.filter(
        (t) =>
          (t.title ?? "").toLowerCase().includes(f) ||
          (t.artist_name ?? "").toLowerCase().includes(f)
      );
    }

    // sort priority (from highest to lowest):
    // 1. disc number
    // 2. track number
    // 3. title
    // 4. artist
    // 5. duration
    if (albumSortField !== "default") {
      const dir = albumSortDir === "asc" ? 1 : -1;

      result = [...result].sort((a, b) => {
        if (albumSortField === "title")
          return dir * (a.title ?? "").localeCompare(b.title ?? "");
        if (albumSortField === "artist")
          return dir * (a.artist_name ?? "").localeCompare(b.artist_name ?? "");
        if (albumSortField === "duration")
          return dir * ((a.duration_ms ?? 0) - (b.duration_ms ?? 0));
        return 0;
      });
    }
    return result;
  })();

  // Album artwork for the detail header
  $: selectedAlbumArtUrl = (() => {
    if (!currentAlbumGroup) return "";
    return getArtUrl(currentAlbumGroup);
  })();

  // Virtualized track list for album detail view
  $: virtualizer = createVirtualizer<HTMLDivElement, HTMLDivElement>({
    count: filteredAlbumDetailTracks.length,
    getScrollElement: () => trackListEl,
    estimateSize: () => 38,
    overscan: 5
  });

  // On mount, load album groups.
  // Library is destroyed/re-created on every view switch (App.svelte uses {#if}),
  // so onMount fires each time the user navigates back. Only do a full
  // scanLocalFolders() on the first ever mount (store still empty); on return
  // visits just refresh the paginated view from the already-populated SQLite
  // table, which is instant and doesn't re-read the file system.
  onMount(() => {
    if ($activeTab === "library") {
      loadRemoteAlbumGroupsPage(0);
    }
    if ($localFolders.length > 0) {
      // If the local album groups are empty, scan the local folders, else load the local album groups.
      get(localAlbumGroups).length === 0
        ? scanLocalFolders()
        : loadLocalAlbumGroups(0, get(localAlbumFilter));
    }
  });

  let gridFilterDebounce: ReturnType<typeof setTimeout>;

  // Load the remote album groups page if the active tab is library,
  // else load the local album groups if the active tab is local.
  // Uses a debounce to prevent too many requests.
  function onAlbumGridFilterInput() {
    clearTimeout(gridFilterDebounce);
    gridFilterDebounce = setTimeout(() => {
      const q = albumGridFilter.trim();
      if ($activeTab === "library") {
        loadRemoteAlbumGroupsPage(0, q);
      } else {
        localAlbumFilter.set(q);
        loadLocalAlbumGroups(0, q);
      }
    }, 300);
  }

  function clearAlbumGridFilter() {
    albumGridFilter = "";
    if ($activeTab === "library") {
      loadRemoteAlbumGroupsPage(0);
    } else {
      localAlbumFilter.set("");
      loadLocalAlbumGroups(0, "");
    }
  }

  let gridScrollEl: HTMLDivElement;
  let loadingMore = false;

  // implement infinite scroll
  function handleGridScroll() {
    if (loadingMore || !hasMore || !gridScrollEl) return;
    const { scrollTop, scrollHeight, clientHeight } = gridScrollEl;
    if (scrollTop + clientHeight >= scrollHeight - 200) {
      loadMorePage();
    }
  }

  async function loadMorePage() {
    loadingMore = true;
    try {
      if ($activeTab === "library") {
        await loadMoreRemoteAlbumGroups(albumGridFilter.trim());
      } else {
        await loadMoreLocalAlbumGroups(get(localAlbumFilter));
      }
    } finally {
      loadingMore = false;
    }
  }

  async function playTrack(track: Track, albumTracks: Track[]) {
    const idx = albumTracks.findIndex((t) => t.id === track.id);
    const queueIds = albumTracks.map((t) => t.id);

    if (currentAlbumGroup) {
      recordRecentAlbum({
        key: currentAlbumGroup.key,
        name: currentAlbumGroup.name,
        artist: currentAlbumGroup.artist,
        isLocal: currentAlbumGroup.isLocal ?? false,
        firstTrackId: currentAlbumGroup.firstTrackId,
        firstLocalPath: currentAlbumGroup.firstLocalPath ?? ""
      });
    }

    if (get(activeTab) === "local") {
      playerState.update((s) => {
        const finalQueue =
          s.shuffle && queueIds.length > 1
            ? [track.id, ...shuffle(queueIds.filter((id) => id !== track.id))]
            : queueIds;
        return {
          ...s,
          trackId: track.id,
          track,
          queue: finalQueue,
          baseQueue: queueIds,
          queueIndex: 0,
          positionMs: 0,
          paused: false
        };
      });
      return;
    }

    if (!$connected) return;

    // Update local state immediately, then sync server state
    const currentShuffle = get(playerState).shuffle;
    const finalQueue =
      currentShuffle && queueIds.length > 1
        ? [track.id, ...shuffle(queueIds.filter((id) => id !== track.id))]
        : queueIds;

    playerState.update((s) => ({
      ...s,
      trackId: track.id,
      track,
      queue: finalQueue,
      baseQueue: queueIds,
      queueIndex: 0,
      positionMs: 0,
      paused: false
    }));

    wsSend("playback.queue", {
      track_ids: finalQueue,
      start_index: 0
    });
    wsSend("playback.play", {
      track_id: track.id,
      position_ms: 0
    });
  }

  function addToQueue(track: Track) {
    playerState.update((s) => {
      const insertAt = s.queueIndex + 1;
      const newQueue = [
        ...s.queue.slice(0, insertAt),
        track.id,
        ...s.queue.slice(insertAt)
      ];
      return { ...s, queue: newQueue };
    });
  }

  async function scanLibrary() {
    if (!$connected) return;
    await serverFetch("/api/library/scan", { method: "POST" });
  }

  function switchTab(tab: LibTab) {
    albumGridFilter = "";
    pushNav({ tab, albumKey: null, subTab: "albums" });

    // Reload album groups for the new tab
    if (tab === "library") {
      loadRemoteAlbumGroupsPage(0);
    } else if (tab === "local") {
      loadLocalAlbumGroups(0, "");
    }
  }

  function localArtUrl(track: Track): string {
    const base = localBase();
    if (!base) return "";
    return `${base}/local/art?path=${encodeURIComponent(track.path)}`;
  }

  function getArtUrl(album: AlbumGroup): string {
    if (album.isLocal && album.firstLocalPath) {
      return localArtUrl({ path: album.firstLocalPath } as Track);
    }
    return artworkUrl(album.firstTrackId);
  }

  function openAlbum(album: AlbumGroup) {
    pushNav({ albumKey: album.key });
  }

  let albumCtxMenu: { group: AlbumGroup; x: number; y: number } | null = null;
  let albumCtxPlaylistSub = false;

  // Handle right-click context menu for albums
  function onAlbumContext(e: MouseEvent, album: AlbumGroup) {
    e.preventDefault();
    albumCtxMenu = { group: album, x: e.clientX, y: e.clientY };
    albumCtxPlaylistSub = false;

    const close = (_: MouseEvent) => {
      // Don't close immediately on the right-click that opened it.
      albumCtxMenu = null;
      window.removeEventListener("click", close);
    };
    window.addEventListener("click", close);
  }

  async function addAlbumToPlaylist(pl: PlaylistSummary, group: AlbumGroup) {
    albumCtxMenu = null;
    let tracksToAdd: Track[] = [];
    const parts =
      group.key === UNORGANIZED_KEY ? ["", ""] : group.key.split("|||");

    if (group.isLocal) {
      try {
        const locals = await fetchLocalAlbumTracks(
          parts[0] ?? "",
          parts[1] ?? ""
        );
        tracksToAdd = locals.map(localTrackToTrack);
      } catch (e) {
        console.warn("addAlbumToPlaylist local:", e);
      }
    } else {
      const params = new URLSearchParams();
      params.set("album_name", parts[0] ?? "");
      params.set("album_artist", parts[1] ?? "");
      params.set("limit", "500");

      try {
        const r = await serverFetch(`/api/library/tracks?${params}`);
        if (r.ok) {
          const d = await r.json();
          tracksToAdd = Array.isArray(d) ? d : (d.tracks ?? []);
        }
      } catch (e) {
        console.warn("addAlbumToPlaylist remote:", e);
      }
    }
    await addTracksToPlaylist(pl.id, tracksToAdd, group.isLocal ?? false);
  }

  async function handleAddFolder() {
    await addLocalFolder();
  }

  function hideImgOnError(e: Event) {
    const img = e.currentTarget as HTMLImageElement;
    if (img) img.style.display = "none";
  }

  function handlePlay(track: Track | null) {
    // Always build the queue from the full unfiltered album tracks so that
    // clicking a filtered song still produces the correct album order queue.
    if (track) playTrack(track, albumDetailTracks);
  }

  function handleQueue(track: Track | null) {
    if (track) addToQueue(track);
  }

  let isScrolling = false;
  let scrollTimer: ReturnType<typeof setTimeout>;

  function handleScroll() {
    isScrolling = true;
    clearTimeout(scrollTimer);
    scrollTimer = setTimeout(() => {
      isScrolling = false;
    }, 150);
  }
</script>

<section>
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

  <div class="scroll-body">
    {#if currentAlbumGroup}
      <div class="album-detail-view">
        <div class="album-detail-header">
          <div class="album-art-hero">
            {#if selectedAlbumArtUrl}
              <img
                src={selectedAlbumArtUrl}
                alt={currentAlbumGroup.name}
                on:error={hideImgOnError}
                loading="lazy"
                decoding="async"
              />
            {/if}
            <div class="album-art-hero-placeholder"><Music size={24} /></div>
          </div>
          <div class="album-detail-info">
            <h2 class="album-detail-title">{currentAlbumGroup.name}</h2>
            <p class="album-meta text-2">
              {currentAlbumGroup.artist} · {currentAlbumGroup.trackCount} tracks ·
              {totalDuration(
                albumDetailTracks.reduce(
                  (sum, t) => sum + (t.duration_ms ?? 0),
                  0
                )
              )}
            </p>
            <div class="album-filter-bar">
              <input
                type="search"
                class="album-filter-input"
                placeholder="Filter songs..."
                bind:value={albumFilter}
              />
            </div>
          </div>
        </div>

        <div class="track-headers hide-album">
          <span class="num">#</span>
          <SortButton
            class="sortable"
            bind:currentField={albumSortField}
            bind:sortDir={albumSortDir}
            field="title">Title</SortButton
          >
          <SortButton
            class="sortable"
            bind:currentField={albumSortField}
            bind:sortDir={albumSortDir}
            field="artist">Artist</SortButton
          >
          <SortButton
            class="sortable"
            bind:currentField={albumSortField}
            bind:sortDir={albumSortDir}
            field="duration">Duration</SortButton
          >
        </div>

        {#if albumFilter && filteredAlbumDetailTracks.length === 0}
          <p class="no-results text-3">No songs match "{albumFilter}"</p>
        {:else}
          <div
            class="track-list"
            class:scrolling={isScrolling}
            bind:this={trackListEl}
            on:scroll={handleScroll}
          >
            <div
              style="position: relative; width: 100%; height: {$virtualizer.getTotalSize()}px;"
            >
              {#each $virtualizer.getVirtualItems() as row (row.index)}
                <div
                  class="virtual-row"
                  style="height: {row.size}px; transform: translateY({row.start}px);"
                >
                  <TrackRow
                    track={filteredAlbumDetailTracks[row.index]}
                    hideAlbum={true}
                    isLocal={currentAlbumGroup?.isLocal ?? false}
                    active={$currentTrackId ===
                      filteredAlbumDetailTracks[row.index]?.id}
                    disableLocal={false}
                    onplay={handlePlay}
                    onselect={() => {}}
                    onaddtoqueue={handleQueue}
                  />
                </div>
              {/each}
            </div>
          </div>
        {/if}
      </div>
    {:else}
      <div
        class="grid-scroll-wrapper"
        bind:this={gridScrollEl}
        on:scroll={handleGridScroll}
      >
        <div class="toolbar">
          <h2>{$activeTab === "library" ? "Library" : "Local Albums"}</h2>
          <div class="toolbar-actions">
            {#if $activeTab === "library"}
              <button on:click={scanLibrary} title="Rescan watch folders"
                ><RotateCcw size={14} /> Scan</button
              >
            {:else}
              <button
                on:click={handleAddFolder}
                title="Add a local music folder">+ Add Folder</button
              >
              {#if $localFolders.length > 0}
                <button
                  on:click={() => scanLocalFolders()}
                  title="Rescan local folders"
                  ><RotateCcw size={14} /> Rescan</button
                >
              {/if}
            {/if}
          </div>
        </div>

        <div class="album-grid-search">
          <Search size={14} />
          <input
            type="search"
            class="album-grid-filter"
            placeholder="Search albums..."
            bind:value={albumGridFilter}
            on:input={onAlbumGridFilterInput}
          />
          {#if albumGridFilter}
            <button class="grid-filter-clear" on:click={clearAlbumGridFilter}
              ><X size={14} /></button
            >
          {/if}
        </div>

        {#if $activeTab === "local" && $localFolders.length > 0}
          <div class="folder-chips">
            {#each $localFolders as dir}
              <span class="folder-chip">
                {dir.split("/").pop() || dir}
                <button
                  class="chip-remove"
                  on:click={() => removeLocalFolder(dir)}
                  title="Remove folder"><X size={14} /></button
                >
              </span>
            {/each}
          </div>
        {/if}

        {#if $activeTab === "library" && !$connected}
          <div class="offline-state">
            <span class="offline-icon"><TriangleAlert size={16} /></span>
            <p class="offline-title">
              {$isReconnecting
                ? "Reconnecting to server..."
                : "Not connected to a server"}
            </p>
            <p class="offline-sub">
              {$isReconnecting
                ? "Your library will appear once the connection is restored."
                : "Open Settings to connect to your pneuma server."}
            </p>
          </div>
        {:else if $activeTab === "local" && $scanProgress}
          <div style="text-align: center; padding: 24px;">
            <p class="text-3" style="margin-bottom: 8px;">
              Scanning music files in: {$scanProgress.folder}
            </p>
            <p class="text-3">
              {$scanProgress.done} / {$scanProgress.total} files
            </p>
          </div>
        {:else if isLoading}
          <p class="text-3" style="text-align: center; padding: 24px;">
            Loading...
          </p>
        {:else if displayedGroups.length === 0}
          {#if $activeTab === "local"}
            <p class="text-3">
              No local music. Click "Add Folder" to add a music directory.
            </p>
          {:else}
            <p class="text-3">
              No tracks found. Add a watch folder in Settings and scan.
            </p>
          {/if}
        {:else if displayedGroups.length === 0}
          <p class="text-3">No albums match "{albumGridFilter}"</p>
        {:else}
          <div class="album-grid">
            {#each displayedGroups as album (album.key)}
              <button
                class="album-card"
                class:unorganized={album.key === UNORGANIZED_KEY}
                on:click={() => openAlbum(album)}
                on:contextmenu={(e) => onAlbumContext(e, album)}
              >
                <div
                  class="album-art"
                  class:unorg-art={album.key === UNORGANIZED_KEY}
                >
                  {#if album.key !== UNORGANIZED_KEY}
                    <img
                      src={getArtUrl(album)}
                      alt={album.name}
                      on:error={hideImgOnError}
                      loading="lazy"
                    />
                  {/if}
                  <div class="album-art-placeholder">
                    {#if album.key === UNORGANIZED_KEY}
                      <FolderOpen size={24} />
                    {:else}
                      <Music size={24} />
                    {/if}
                  </div>
                </div>
                <p
                  class="album-title truncate"
                  class:unorg-title={album.key === UNORGANIZED_KEY}
                >
                  {album.name}
                </p>
                <p class="album-artist truncate text-3">
                  {album.artist} · {album.trackCount} tracks
                </p>
              </button>
            {/each}
          </div>
          {#if hasMore}
            <p class="text-3" style="text-align:center;padding:12px;">
              Loading more...
            </p>
          {/if}
        {/if}
      </div>
    {/if}
  </div>

  {#if albumCtxMenu}
    {@const grp = albumCtxMenu.group}
    <div
      class="album-ctx-menu"
      use:portal
      style="left:{albumCtxMenu.x}px;top:{albumCtxMenu.y}px"
    >
      {#if $playlists.length > 0}
        <!-- svelte-ignore a11y_no_static_element_interactions -->
        <div
          class="album-ctx-sub-wrap"
          on:mouseenter={() => (albumCtxPlaylistSub = true)}
          on:mouseleave={() => (albumCtxPlaylistSub = false)}
        >
          <button class="has-sub"
            >Add all to playlist <ChevronRight size={12} /></button
          >
          {#if albumCtxPlaylistSub}
            <div class="album-ctx-submenu">
              {#each $playlists as pl (pl.id)}
                <button on:click={() => addAlbumToPlaylist(pl, grp)}
                  >{pl.name}</button
                >
              {/each}
            </div>
          {/if}
        </div>
      {:else}
        <button disabled style="opacity:0.5">No playlists yet</button>
      {/if}
    </div>
  {/if}
</section>

<style>
  section {
    display: flex;
    flex-direction: column;
    height: 100%;
    overflow: hidden;
  }

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
    transition:
      color 0.12s,
      border-color 0.12s;
  }
  .lib-tab:hover {
    color: var(--text-1);
  }
  .lib-tab.active {
    color: var(--accent);
    border-bottom-color: var(--accent);
  }

  .scroll-body {
    flex: 1;
    min-height: 0;
    overflow: hidden;
    display: flex;
    flex-direction: column;
    padding: 16px 16px 0 0;
  }

  .album-detail-view {
    flex: 1;
    display: flex;
    flex-direction: column;
    min-height: 0;
    overflow: hidden;
  }

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

  h2 {
    margin: 0;
    font-size: 20px;
    font-weight: 700;
  }

  .album-meta {
    font-size: 13px;
    margin: 0;
  }

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
  .chip-remove:hover {
    color: var(--danger);
  }

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
  .album-grid-search:focus-within {
    border-color: var(--accent);
  }

  .album-grid-filter {
    flex: 1;
    background: none;
    border: none;
    color: var(--fg);
    font-size: 13px;
    outline: none;
    padding: 0;
  }
  .album-grid-filter::placeholder {
    color: var(--text-3);
  }
  .album-grid-filter::-webkit-search-cancel-button {
    display: none;
  }

  .grid-filter-clear {
    background: none;
    border: none;
    color: var(--text-3);
    font-size: 16px;
    cursor: pointer;
    padding: 0;
    line-height: 1;
  }
  .grid-filter-clear:hover {
    color: var(--fg);
  }

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
  .album-card:hover .album-art {
    border-color: var(--accent);
  }

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

  .album-title {
    margin: 0;
    font-size: 13px;
    font-weight: 600;
  }
  .album-artist {
    margin: 2px 0 0;
    font-size: 11px;
  }

  .unorganized .album-art,
  .unorg-art {
    border: 2px dashed var(--text-3);
    background: transparent;
  }
  .unorg-title {
    font-style: italic;
  }

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
  .offline-icon {
    font-size: 36px;
    opacity: 0.4;
  }
  .offline-title {
    margin: 0;
    font-size: 15px;
    font-weight: 600;
    color: var(--text-1);
  }
  .offline-sub {
    margin: 0;
    font-size: 13px;
    color: var(--text-3);
  }

  .virtual-row {
    position: absolute;
    top: 0;
    left: 0;
    width: 100%;
  }

  .track-list.scrolling .virtual-row {
    pointer-events: none;
  }

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
  .album-filter-input:focus {
    border-color: var(--accent);
  }
  .album-filter-input::placeholder {
    color: var(--text-3);
  }
  /* hide default search clear button */
  .album-filter-input::-webkit-search-cancel-button {
    display: none;
  }

  .no-results {
    padding: 16px 8px;
    font-size: 13px;
  }

  .album-ctx-menu {
    position: fixed;
    z-index: 9999;
    background: var(--surface-2);
    border: 1px solid var(--border);
    border-radius: var(--r-md);
    padding: 4px 0;
    box-shadow: 0 8px 24px rgba(0, 0, 0, 0.5);
    min-width: 180px;
  }
  .album-ctx-menu button {
    display: block;
    width: 100%;
    text-align: left;
    padding: 8px 14px;
    font-size: 13px;
    color: var(--text-1);
    border-radius: 0;
    cursor: pointer;
  }
  .album-ctx-menu button:hover {
    background: var(--surface-hover);
  }
  .album-ctx-sub-wrap {
    position: relative;
  }
  .album-ctx-sub-wrap .has-sub {
    cursor: default;
  }
  .album-ctx-submenu {
    position: absolute;
    left: 100%;
    top: 0;
    background: var(--surface-2);
    border: 1px solid var(--border);
    border-radius: var(--r-md);
    padding: 4px 0;
    box-shadow: 0 8px 24px rgba(0, 0, 0, 0.5);
    min-width: 160px;
    max-height: 240px;
    overflow-y: auto;
  }
  .album-ctx-submenu button {
    display: block;
    width: 100%;
    text-align: left;
    padding: 8px 14px;
    font-size: 13px;
    color: var(--text-1);
    border-radius: 0;
  }
  .album-ctx-submenu button:hover {
    background: var(--surface-hover);
  }
</style>
