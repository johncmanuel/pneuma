<script lang="ts">
  import { onMount } from "svelte";
  import { derived, get } from "svelte/store";
  import {
    albumGroups,
    albumGroupsTotal,
    loading,
    loadAlbumGroupsPage,
    loadMoreAlbumGroups,
    fetchAlbumTracks
  } from "../lib/stores/library";
  import { playerState, appendTrackToQueue } from "../lib/stores/playback";
  import { selectedAlbum, pushNav } from "../lib/stores/ui";
  import {
    visiblePlaylistsForAddMenu,
    handleAddTracksToPlaylist,
    playlists as playlistsStore,
    toggleFavoriteTrack,
    setPlayingPlaylistContext
  } from "../lib/stores/playlists";
  import { artworkUrl } from "../lib/api";
  import { wsSend } from "../lib/ws";
  import { recordRecentAlbum } from "../lib/stores/recent";
  import { portal, shuffle, type Track, type AlbumGroup } from "@pneuma/shared";
  import {
    markMissingTrackArtID,
    missingTrackArtIDs
  } from "../lib/stores/missing-art";
  import TrackRow from "../components/TrackRow.svelte";
  import { AlbumGrid, AlbumDetail } from "@pneuma/ui";
  import { ChevronRight } from "@lucide/svelte";

  const currentTrackId = derived(playerState, ($s) => $s.trackId);

  let albumGridFilter = $state("");
  let currentAlbumGroup: AlbumGroup | null = $state(null);
  let albumDetailTracks: Track[] = $state([]);
  let albumDetailLoading = $state(false);

  let hasMore = $derived($albumGroups.length < $albumGroupsTotal);

  // Load the selected album's tracks when the selected album changes
  $effect(() => {
    if ($selectedAlbum && !albumDetailLoading) {
      const group = $albumGroups.find((g) => g.key === $selectedAlbum) ?? null;
      if (
        group &&
        (!currentAlbumGroup || currentAlbumGroup.key !== group.key)
      ) {
        loadAlbumDetail(group);
      }
    }
  });

  // Clear album detail when deselecting
  $effect(() => {
    if (!$selectedAlbum) {
      currentAlbumGroup = null;
      albumDetailTracks = [];
    }
  });

  async function loadAlbumDetail(group: AlbumGroup) {
    albumDetailLoading = true;
    currentAlbumGroup = group;

    try {
      const tracks = await fetchAlbumTracks(group.name, group.artist);
      albumDetailTracks = tracks;
    } catch (e) {
      console.warn("Failed to load album detail:", e);
      albumDetailTracks = [];
    } finally {
      albumDetailLoading = false;
    }
  }

  onMount(() => {
    if ($albumGroups.length === 0) {
      loadAlbumGroupsPage(0);
    }
  });

  function handleSearch(query: string) {
    loadAlbumGroupsPage(0, query);
  }

  async function loadMorePage() {
    await loadMoreAlbumGroups(albumGridFilter.trim());
  }

  function handleSelectAlbum(album: AlbumGroup) {
    pushNav({ view: "library", albumKey: album.key });
  }

  function handleGetArtUrl(album: AlbumGroup): string {
    if ($missingTrackArtIDs[album.first_track_id]) return "";
    return artworkUrl(album.first_track_id);
  }

  function handleImgError(e: Event, trackID?: string) {
    const img = e.currentTarget as HTMLImageElement;
    if (img) img.style.display = "none";
    if (trackID) {
      markMissingTrackArtID(trackID);
    }
  }

  function handleHideImgOnError(e: Event) {
    const img = e.currentTarget as HTMLImageElement;
    if (img) img.style.display = "none";
  }

  async function playTrack(track: Track) {
    const queueIds = albumDetailTracks.map((t) => t.id);

    if (currentAlbumGroup) {
      recordRecentAlbum({
        album_artist: currentAlbumGroup.artist,
        album_name: currentAlbumGroup.name,
        first_track_id: currentAlbumGroup.first_track_id
      });
    }

    const currentShuffle = get(playerState).shuffle;
    const finalQueue =
      currentShuffle && queueIds.length > 1
        ? [track.id, ...shuffle(queueIds.filter((id) => id !== track.id))]
        : queueIds;

    setPlayingPlaylistContext(null);

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

  let albumCtxMenu: { album: AlbumGroup; x: number; y: number } | null =
    $state(null);
  let albumCtxPlaylistSub = $state(false);

  function handleContextMenu(event: MouseEvent, album: AlbumGroup) {
    event.preventDefault();
    albumCtxMenu = { album, x: event.clientX, y: event.clientY };
    albumCtxPlaylistSub = false;

    const close = () => {
      albumCtxMenu = null;
      window.removeEventListener("click", close);
    };
    window.addEventListener("click", close);
  }

  async function addAlbumToPlaylist(playlistId: string, album: AlbumGroup) {
    albumCtxMenu = null;
    const parts =
      album.key === "__unorganized__" ? ["", ""] : album.key.split("|||");
    const tracks = await fetchAlbumTracks(parts[0] ?? "", parts[1] ?? "");
    await handleAddTracksToPlaylist(tracks, playlistId);
  }
</script>

<section>
  <div class="scroll-body">
    {#if currentAlbumGroup}
      <!-- TODO: work on types -->
      <AlbumDetail
        album={currentAlbumGroup}
        tracks={albumDetailTracks}
        loading={albumDetailLoading}
        currentTrackId={$currentTrackId}
        getArtUrl={() =>
          currentAlbumGroup &&
          !$missingTrackArtIDs[currentAlbumGroup.first_track_id]
            ? artworkUrl(currentAlbumGroup.first_track_id)
            : ""}
        hideImgOnError={(e: Event) =>
          handleImgError(e, currentAlbumGroup?.first_track_id)}
        trackRowComponent={TrackRow}
        onPlayTrack={(t: any) => playTrack(t)}
        onAddToQueue={(t: any) => appendTrackToQueue(t)}
        onToggleFavorite={toggleFavoriteTrack}
      />
    {:else}
      <AlbumGrid
        albums={$albumGroups}
        {hasMore}
        loading={$loading}
        title="Library"
        searchPlaceholder="Search albums..."
        onSearch={handleSearch}
        onLoadMore={loadMorePage}
        onSelectAlbum={handleSelectAlbum}
        onContextMenu={handleContextMenu}
        getArtUrl={handleGetArtUrl}
        hideImgOnError={(e: Event) => handleHideImgOnError(e)}
      >
        {#snippet emptyState()}
          <p class="text-3">
            No tracks found. Add music to the server and scan.
          </p>
        {/snippet}
      </AlbumGrid>
    {/if}
  </div>

  {#if albumCtxMenu}
    {@const album = albumCtxMenu.album}
    <div
      class="album-ctx-menu"
      use:portal
      style="left:{albumCtxMenu.x}px;top:{albumCtxMenu.y}px"
    >
      {#if $playlistsStore.length > 0}
        <!-- svelte-ignore a11y_no_static_element_interactions -->

        <div
          class="album-ctx-sub-wrap"
          onmouseenter={() => (albumCtxPlaylistSub = true)}
          onmouseleave={() => (albumCtxPlaylistSub = false)}
        >
          <button class="has-sub"
            >Add all to playlist <ChevronRight size={12} /></button
          >
          {#if albumCtxPlaylistSub}
            <div class="album-ctx-submenu">
              {#each visiblePlaylistsForAddMenu($playlistsStore) as playlist (playlist.id)}
                <button onclick={() => addAlbumToPlaylist(playlist.id, album)}
                  >{playlist.name}</button
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

  .scroll-body {
    flex: 1;
    min-height: 0;
    overflow: hidden;
    display: flex;
    flex-direction: column;
    padding: 16px 8px 0 0;
  }

  .album-ctx-menu {
    position: fixed;
    z-index: 9999;
    background: var(--surface-2);
    border: 1px solid var(--border);
    border-radius: var(--r-md);
    padding: 4px 0;
    box-shadow: var(--shadow-pop);
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
    box-shadow: var(--shadow-pop);
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

  @media (max-width: 980px) {
    .scroll-body {
      padding: 8px 0 0;
    }
  }
</style>
