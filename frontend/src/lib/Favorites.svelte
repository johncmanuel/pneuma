<script lang="ts">
  import { createVirtualizer } from "@tanstack/svelte-virtual";
  import { derived } from "svelte/store";
  import {
    favoritesPlaylistId,
    selectedPlaylistId,
    selectedPlaylist,
    selectedPlaylistItems,
    playlistsLoading,
    loadPlaylists,
    selectPlaylist,
    playPlaylist,
    removePlaylistItem,
    favoriteTrackIDs,
    toggleFavoriteTrack,
    syncFavoritesFromServer,
    setPlayingPlaylistContext,
    type PlaylistItem
  } from "../stores/playlists";
  import { selectedPlaylistView, pushNav } from "../stores/ui";
  import { playerState } from "../stores/player";
  import { connected } from "../utils/api";
  import { addToast, totalDuration } from "@pneuma/shared";
  import { Heart } from "@lucide/svelte";
  import TrackRow from "./TrackRow.svelte";
  import { SortButton } from "@pneuma/ui";
  import "@pneuma/ui/css/track-list.css";

  const currentTrackId = derived(playerState, ($s) => $s.trackId);

  let trackListEl: HTMLDivElement | undefined = $state();
  let filter = $state("");
  let syncingFavorites = $state(false);

  type SortField = "default" | "title" | "added_at" | "duration";
  let sortField: SortField = $state("default");
  let sortDir: "asc" | "desc" = $state("asc");

  $effect(() => {
    if ($selectedPlaylistView || !$favoritesPlaylistId) return;

    pushNav({
      view: "favorites",
      playlistId: $favoritesPlaylistId,
      albumKey: null
    });
  });

  $effect(() => {
    if (!$favoritesPlaylistId) return;

    if ($selectedPlaylistView !== $favoritesPlaylistId) {
      selectedPlaylistView.set($favoritesPlaylistId);
    }

    selectPlaylist($favoritesPlaylistId);
  });

  let filteredItems = $derived(
    $selectedPlaylistItems
      .filter((item) => {
        if (!filter) return true;
        const q = filter.toLowerCase();
        return (
          item.ref_title.toLowerCase().includes(q) ||
          item.ref_album.toLowerCase().includes(q) ||
          item.ref_album_artist.toLowerCase().includes(q)
        );
      })
      .sort((a, b) => {
        if (sortField === "default") return a.position - b.position;

        let cmp = 0;

        if (sortField === "title") cmp = a.ref_title.localeCompare(b.ref_title);
        else if (sortField === "added_at")
          cmp = a.added_at.localeCompare(b.added_at);
        else if (sortField === "duration")
          cmp = a.ref_duration_ms - b.ref_duration_ms;

        return sortDir === "desc" ? -cmp : cmp;
      })
  );

  let virtualizer = $derived(
    createVirtualizer<HTMLDivElement, HTMLDivElement>({
      count: filteredItems.length,
      getScrollElement: () => trackListEl as HTMLDivElement,
      estimateSize: () => 38,
      overscan: 5
    })
  );

  async function handlePlay(item: PlaylistItem) {
    const index = $selectedPlaylistItems.findIndex(
      (entry) => entry.position === item.position
    );

    setPlayingPlaylistContext($selectedPlaylistId ?? $favoritesPlaylistId);

    await playPlaylist(
      $selectedPlaylistItems,
      index >= 0 ? index : 0,
      $selectedPlaylistId ?? $favoritesPlaylistId ?? undefined
    );
  }

  async function handleRemove(item: PlaylistItem) {
    if (!$favoritesPlaylistId) return;

    await removePlaylistItem($favoritesPlaylistId, item.position);
  }

  async function handleForceSync() {
    if (!$connected) return;

    syncingFavorites = true;
    try {
      await syncFavoritesFromServer();
      await loadPlaylists();
      if ($favoritesPlaylistId) {
        await selectPlaylist($favoritesPlaylistId);
      }
      addToast("Favorites synchronized", "success");
    } catch (e: any) {
      addToast(`Failed to sync favorites: ${e}`, "error");
    } finally {
      syncingFavorites = false;
    }
  }

  function itemToTrack(item: PlaylistItem) {
    return {
      id:
        item.source === "local_ref" && item.local_path
          ? item.local_path
          : item.track_id || `missing-${item.position}`,
      path: item.local_path || "",
      title: item.ref_title || "Unknown",
      artist_id: "",
      album_id: "",
      artist_name: item.ref_album_artist || "",
      album_artist: item.ref_album_artist || "",
      album_name: item.ref_album || "",
      genre: "",
      year: 0,
      track_number: item.position + 1,
      disc_number: 0,
      duration_ms: item.ref_duration_ms,
      bitrate_kbps: 0,
      artwork_id: ""
    };
  }

  function formatDate(iso: string): string {
    if (!iso) return "-";

    const date = new Date(iso);
    return date.toLocaleDateString(undefined, {
      year: "numeric",
      month: "short",
      day: "numeric"
    });
  }
</script>

<div class="favorites-view">
  <div class="detail-header">
    <div class="detail-hero">
      <div class="detail-art favorites-art" aria-hidden="true">
        <span class="art-placeholder"><Heart size={24} /></span>
      </div>
      <div class="detail-meta">
        <h1 class="detail-name">Favorites</h1>
        <p class="detail-info text-3">
          {$selectedPlaylistItems.length} songs
          {#if $selectedPlaylist?.total_duration_ms}
            &middot; {totalDuration($selectedPlaylist.total_duration_ms)}
          {/if}
        </p>
      </div>
    </div>

    <div class="detail-actions">
      <button
        class="action-btn primary"
        onclick={() => {
          if ($selectedPlaylistItems.length > 0)
            handlePlay($selectedPlaylistItems[0]);
        }}
        disabled={$selectedPlaylistItems.length === 0}
      >
        Play
      </button>
      {#if $connected}
        <button
          class="action-btn"
          onclick={handleForceSync}
          disabled={syncingFavorites}
        >
          {syncingFavorites ? "Syncing..." : "Force Sync"}
        </button>
      {/if}
      <div class="filter-spacer"></div>
      <input
        type="text"
        class="filter-input"
        placeholder="Filter tracks..."
        bind:value={filter}
      />
    </div>
  </div>

  <div class="track-headers">
    <span class="num">#</span>
    <SortButton
      class="sortable"
      bind:currentField={sortField}
      bind:sortDir
      field="title">Title</SortButton
    >
    <span>Album</span>
    <SortButton
      class="sortable"
      bind:currentField={sortField}
      bind:sortDir
      field="added_at">Date Added</SortButton
    >
    <SortButton
      class="sortable"
      bind:currentField={sortField}
      bind:sortDir
      field="duration">Duration</SortButton
    >
  </div>

  <div class="track-list" bind:this={trackListEl}>
    {#if $playlistsLoading}
      <p class="text-3 loading-msg">Loading...</p>
    {:else if filteredItems.length === 0}
      <p class="text-3 empty-msg">
        {filter ? "No matching tracks." : "No favorite tracks yet."}
      </p>
    {:else}
      <div
        style="position: relative; width: 100%; height: {$virtualizer.getTotalSize()}px;"
      >
        {#each $virtualizer.getVirtualItems() as row (row.index)}
          {@const item = filteredItems[row.index]}
          {@const track = itemToTrack(item)}
          <div
            class="virtual-row playlist-row"
            class:missing={item.missing}
            style="height: {row.size}px; transform: translateY({row.start}px);"
          >
            <TrackRow
              {track}
              active={$currentTrackId === track.id}
              dateAdded={formatDate(item.added_at)}
              showRemove={true}
              isLocal={item.source === "local_ref"}
              isFavorite={$favoriteTrackIDs.has(track.id)}
              hideFavoriteIcon={true}
              disableLocal={false}
              onPlay={() => handlePlay(item)}
              onAddToQueue={(t) => {}}
              onRemove={() => handleRemove(item)}
              onToggleFavorite={toggleFavoriteTrack}
            />
          </div>
        {/each}
      </div>
    {/if}
  </div>
</div>

<style>
  .favorites-view {
    display: flex;
    flex-direction: column;
    height: 100%;
  }

  .detail-header {
    margin-bottom: 16px;
  }

  .detail-hero {
    display: flex;
    gap: 20px;
    margin-bottom: 16px;
  }

  .detail-art {
    width: 160px;
    height: 160px;
    flex-shrink: 0;
    border-radius: var(--r-md);
    background: var(--surface);
    display: flex;
    align-items: center;
    justify-content: center;
    border: 1px solid var(--border);
  }

  .detail-art.favorites-art {
    color: var(--accent);
  }

  .art-placeholder {
    font-size: 48px;
    color: inherit;
  }

  .detail-meta {
    display: flex;
    flex-direction: column;
    justify-content: flex-end;
  }

  .detail-name {
    margin: 0;
    font-size: 28px;
    font-weight: 700;
  }

  .detail-info {
    margin: 8px 0 0;
    font-size: 12px;
  }

  .detail-actions {
    display: flex;
    align-items: center;
    gap: 8px;
    margin-top: 12px;
  }

  .filter-spacer {
    flex: 1;
  }

  .action-btn {
    padding: 8px 18px;
    border-radius: 20px;
    font-size: 13px;
    font-weight: 600;
    cursor: pointer;
    background: var(--surface);
    color: var(--text-1);
    border: 1px solid var(--border);
    transition: background 0.1s;
  }

  .action-btn:hover {
    background: var(--surface-hover);
  }

  .action-btn.primary {
    background: var(--accent-dim);
    color: var(--on-accent-dim);
    border: none;
  }

  .action-btn.primary:hover {
    filter: brightness(1.1);
  }

  .action-btn:disabled {
    opacity: 0.4;
    cursor: default;
  }

  .filter-input {
    width: 200px;
    padding: 6px 10px;
    border-radius: var(--r-sm);
    border: 1px solid var(--border);
    background: var(--bg);
    color: var(--text-1);
    font-size: 12px;
  }

  .virtual-row {
    position: absolute;
    top: 0;
    left: 0;
    width: 100%;
  }

  .playlist-row :global(.track-row) {
    flex: 1;
  }

  .playlist-row.missing {
    opacity: 0.45;
  }

  .playlist-row.missing :global(.track-row) {
    pointer-events: none;
  }

  .empty-msg {
    padding: 40px 12px;
    text-align: center;
  }

  .loading-msg {
    padding: 20px 12px;
    text-align: center;
  }
</style>
