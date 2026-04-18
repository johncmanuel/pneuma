<script lang="ts">
  import { createVirtualizer } from "@tanstack/svelte-virtual";
  import { derived } from "svelte/store";
  import {
    favoritesPlaylistId,
    selectedPlaylist,
    selectedPlaylistItems,
    playlistsLoading,
    loadPlaylists,
    selectPlaylist,
    removePlaylistItem,
    itemToTrack,
    toggleFavoriteTrack,
    setPlayingPlaylistContext
  } from "../lib/stores/playlists";
  import { selectedPlaylistView, pushNav } from "../lib/stores/ui";
  import { playerState } from "../lib/stores/playback";
  import { loadRecent, recordRecentPlaylist } from "../lib/stores/recent";
  import { wsSend } from "../lib/ws";
  import { Heart } from "@lucide/svelte";
  import TrackRow from "../components/TrackRow.svelte";
  import { SortButton } from "@pneuma/ui";
  import "@pneuma/ui/css/track-list.css";
  import { totalDuration, addToast, type PlaylistItem } from "@pneuma/shared";

  const currentTrackId = derived(playerState, ($s) => $s.trackId);

  let trackListEl: HTMLDivElement | undefined = $state();
  let filter = $state("");

  type SortField = "default" | "title" | "added_at" | "duration";
  let sortField: SortField = $state("default");
  let sortDir: "asc" | "desc" = $state("asc");
  let syncingFavorites = $state(false);

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
          cmp = (a.added_at || "").localeCompare(b.added_at || "");
        else if (sortField === "duration")
          cmp = (a.ref_duration_ms || 0) - (b.ref_duration_ms || 0);

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

  function handlePlay(item: PlaylistItem) {
    const tracks = $selectedPlaylistItems.map(itemToTrack);
    const idx = $selectedPlaylistItems.findIndex(
      (i) => i.position === item.position
    );
    const queueIds = tracks.map((track) => track.id);

    if ($selectedPlaylist?.id) {
      recordRecentPlaylist({
        playlist_id: $selectedPlaylist.id,
        name: $selectedPlaylist.name,
        artwork_path: $selectedPlaylist.artwork_path
      });

      setPlayingPlaylistContext($selectedPlaylist.id);
    }

    playerState.update((s) => ({
      ...s,
      trackId: tracks[idx >= 0 ? idx : 0].id,
      track: tracks[idx >= 0 ? idx : 0],
      queue: queueIds,
      queueIndex: idx >= 0 ? idx : 0,
      positionMs: 0,
      paused: false
    }));

    wsSend("playback.queue", {
      track_ids: queueIds,
      start_index: idx >= 0 ? idx : 0
    });
    wsSend("playback.play", {
      track_id: tracks[idx >= 0 ? idx : 0].id,
      position_ms: 0
    });
  }

  async function handleRemove(item: PlaylistItem) {
    if (!$favoritesPlaylistId) return;
    await removePlaylistItem($favoritesPlaylistId, item.position);
  }

  async function handleForceSync() {
    syncingFavorites = true;
    try {
      await loadPlaylists();
      if ($favoritesPlaylistId) {
        await selectPlaylist($favoritesPlaylistId);
      }
      await loadRecent();
      addToast("Favorites synchronized", "success");
    } catch (e) {
      addToast(`Failed to sync favorites: ${e}`, "error");
    } finally {
      syncingFavorites = false;
    }
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
  <div class="header">
    <div class="hero">
      <div class="cover" aria-hidden="true">
        <Heart size={28} />
      </div>
      <div class="meta">
        <h1>Favorites</h1>
        <p class="text-3">
          {$selectedPlaylistItems.length} songs
          {#if $selectedPlaylist?.total_duration_ms}
            &middot; {totalDuration($selectedPlaylist.total_duration_ms)}
          {/if}
        </p>
      </div>
    </div>

    <div class="actions">
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
      <button
        class="action-btn"
        onclick={handleForceSync}
        disabled={syncingFavorites}
      >
        {syncingFavorites ? "Syncing..." : "Force Sync"}
      </button>
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
            class="virtual-row"
            class:missing={item.missing}
            style="height: {row.size}px; transform: translateY({row.start}px);"
          >
            <TrackRow
              {track}
              active={$currentTrackId === track.id}
              dateAdded={formatDate(item.added_at)}
              showRemove={true}
              isLocal={item.source === "local_ref"}
              hideFavoriteIcon={true}
              onPlay={() => handlePlay(item)}
              onSelect={() => {}}
              onAddToQueue={() => {}}
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

  .header {
    margin-bottom: 16px;
  }

  .hero {
    display: flex;
    gap: 20px;
    margin-bottom: 16px;
  }

  .cover {
    width: 160px;
    height: 160px;
    border-radius: var(--r-md);
    background: var(--surface);
    border: 1px solid var(--border);
    display: flex;
    align-items: center;
    justify-content: center;
    color: var(--accent);
    flex-shrink: 0;
  }

  .meta {
    display: flex;
    flex-direction: column;
    justify-content: flex-end;
  }

  h1 {
    margin: 0;
    font-size: 28px;
    font-weight: 700;
  }

  .actions {
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

  .virtual-row.missing {
    opacity: 0.45;
  }

  .virtual-row.missing :global(.track-row) {
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

  @media (max-width: 980px) {
    .header {
      margin-bottom: 12px;
    }

    .hero {
      gap: 12px;
      margin-bottom: 10px;
    }

    .cover {
      width: 104px;
      height: 104px;
      border-radius: 10px;
    }

    h1 {
      font-size: 20px;
    }

    .actions {
      flex-wrap: wrap;
      gap: 6px;
      margin-top: 8px;
    }

    .action-btn {
      padding: 7px 12px;
      font-size: 12px;
    }

    .filter-spacer {
      display: none;
    }

    .filter-input {
      width: 100%;
      flex-basis: 100%;
      margin-top: 2px;
    }

    .track-headers {
      display: none;
    }

    .track-list :global(.track-row) {
      grid-template-columns: 30px minmax(0, 1fr) 68px;
      gap: 0 8px;
      min-height: 40px;
      padding: 8px 8px;
    }

    .track-list :global(.track-row .artist),
    .track-list :global(.track-row .album) {
      display: none;
    }

    .track-list :global(.track-row .title) {
      font-size: 13px;
    }
  }

  @media (max-width: 620px) {
    .hero {
      align-items: center;
    }

    .cover {
      width: 84px;
      height: 84px;
    }

    h1 {
      font-size: 18px;
    }
  }
</style>
