<script lang="ts">
  import { onMount } from "svelte";
  import { createVirtualizer } from "@tanstack/svelte-virtual";
  import { derived } from "svelte/store";
  import {
    isFavoritesPlaylist,
    playlists,
    selectedPlaylist,
    selectedPlaylistItems,
    playlistsLoading,
    loadPlaylists,
    createPlaylist,
    deletePlaylist,
    selectPlaylist,
    removePlaylistItem,
    itemToTrack,
    toggleFavoriteTrack,
    updatePlaylist,
    handleAddToPlaylist
  } from "../lib/stores/playlists";
  import { selectedPlaylistView, pushNav } from "../lib/stores/ui";
  import { playerState } from "../lib/stores/playback";
  import { playlistArtUrl, uploadPlaylistArtwork } from "../lib/api";
  import { wsSend } from "../lib/ws";
  import {
    recordRecentPlaylist,
    removeRecentPlaylist
  } from "../lib/stores/recent";
  import { Heart, Music, SquarePen } from "@lucide/svelte";
  import TrackRow from "../components/TrackRow.svelte";
  import {
    totalDuration,
    type PlaylistItem,
    type PlaylistSummary
  } from "@pneuma/shared";
  import { generateRandomPlaylist } from "../lib/api";
  import { SortButton } from "@pneuma/ui";

  type SortField = "default" | "title" | "added_at" | "duration";

  const currentTrackId = derived(playerState, ($s) => $s.trackId);

  let showNewDialog = $state(false);
  let newName = $state("");
  let newDesc = $state("");

  let showGenerateDialog = $state(false);
  let generateName = $state("");
  let generateDesc = $state("");
  let generateDuration = $state(30);
  let generating = $state(false);

  let editingId: string | null = $state(null);
  let editName = $state("");
  let editDesc = $state("");
  let trackListEl: HTMLDivElement | undefined = $state();
  let artInput: HTMLInputElement | undefined = $state();
  let uploadingArt = $state(false);

  async function handleArtUpload(e: Event) {
    const input = e.target as HTMLInputElement;
    const file = input.files?.[0];
    if (!file || !$selectedPlaylistView) return;

    uploadingArt = true;
    try {
      await uploadPlaylistArtwork($selectedPlaylistView, file);
      await selectPlaylist($selectedPlaylistView);
      await loadPlaylists();
    } finally {
      uploadingArt = false;
      input.value = "";
    }
  }

  let selectedIsFavorites = $derived(isFavoritesPlaylist($selectedPlaylist));

  function triggerArtUpload() {
    if (selectedIsFavorites) return;
    artInput?.click();
  }

  $effect(() => {
    if ($selectedPlaylistView) {
      selectPlaylist($selectedPlaylistView);
    } else {
      selectedPlaylistView.set(null);
      selectedPlaylist.set(null);
      selectedPlaylistItems.set([]);
    }
  });

  let filter = $state("");
  let sortField: SortField = $state("default");
  let sortDir: "asc" | "desc" = $state("asc");

  let filteredItems = $derived(
    $selectedPlaylistItems
      .filter((i) => {
        if (!filter) return true;
        const q = filter.toLowerCase();
        return i.ref_album_artist.toLowerCase().includes(q);
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

  let visiblePlaylists = $derived(
    $playlists.filter((pl) => !isFavoritesPlaylist(pl))
  );

  let playlistDurationMS = $derived(
    $selectedPlaylistItems.reduce(
      (sum, item) => sum + (item.ref_duration_ms || 0),
      0
    )
  );

  async function handleCreate() {
    if (!newName.trim()) return;
    const id = await createPlaylist(newName.trim(), newDesc.trim());
    newName = "";
    newDesc = "";
    showNewDialog = false;
    if (id) {
      pushNav({ view: "playlists", playlistId: id });
    }
  }

  async function handleGenerate() {
    if (!generateName.trim() || generateDuration < 1) return;
    generating = true;
    try {
      const pl = await generateRandomPlaylist(
        generateName.trim(),
        generateDesc.trim(),
        generateDuration
      );
      handleCancelGenerate();
      if (pl) {
        await loadPlaylists();
        pushNav({ view: "playlists", playlistId: pl.id });
      }
    } finally {
      generating = false;
    }
  }

  function handleCancelGenerate() {
    generateName = "";
    generateDesc = "";
    generateDuration = 30;
    showGenerateDialog = false;
  }

  function openPlaylist(pl: PlaylistSummary) {
    pushNav({ view: "playlists", playlistId: pl.id });
  }

  function handlePlay(item: PlaylistItem) {
    const tracks = $selectedPlaylistItems.map(itemToTrack);
    const idx = $selectedPlaylistItems.findIndex(
      (i) => i.position === item.position
    );
    const queueIds = tracks.map((t) => t.id);

    if ($selectedPlaylist) {
      recordRecentPlaylist({
        playlist_id: $selectedPlaylist.id
      });
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
    if ($selectedPlaylistView) {
      await removePlaylistItem($selectedPlaylistView, item.position);
    }
  }

  function formatDate(iso: string): string {
    if (!iso) return "—";
    const d = new Date(iso);
    return d.toLocaleDateString(undefined, {
      year: "numeric",
      month: "short",
      day: "numeric"
    });
  }

  function startEdit(pl: PlaylistSummary) {
    editingId = pl.id;
    editName = pl.name;
    editDesc = pl.description;
  }

  async function saveEdit() {
    if (editingId && editName.trim()) {
      await updatePlaylist(editingId, editName.trim(), editDesc.trim());
      editingId = null;
    }
  }

  function cancelEdit() {
    editingId = null;
  }

  function handleCancelCreate() {
    showNewDialog = false;
    newName = "";
    newDesc = "";
  }

  function hideImg(e: Event) {
    const img = e.currentTarget as HTMLImageElement;
    if (img) img.style.display = "none";
  }

  onMount(() => {
    loadPlaylists();
  });
</script>

{#if $selectedPlaylistView && $selectedPlaylist}
  <div class="playlist-detail">
    <div class="detail-header">
      <div class="detail-hero">
        <input
          type="file"
          accept="image/*"
          bind:this={artInput}
          onchange={handleArtUpload}
          style="display:none"
        />
        <button
          class="detail-art"
          class:favorites-art={selectedIsFavorites}
          onclick={triggerArtUpload}
          disabled={uploadingArt || selectedIsFavorites}
          title={selectedIsFavorites ? "Favorites" : "Change artwork"}
        >
          {#if !selectedIsFavorites && $selectedPlaylist.artwork_path}
            <img
              src={playlistArtUrl(
                $selectedPlaylist.id,
                $selectedPlaylist.updated_at
              )}
              alt=""
              onerror={hideImg}
            />
          {/if}
          <span class="art-placeholder"
            >{#if selectedIsFavorites}<Heart size={24} />{:else}<Music
                size={24}
              />{/if}</span
          >
          {#if !selectedIsFavorites}
            <div class="art-overlay">
              <SquarePen size={24} />
            </div>
          {/if}
        </button>
        <div class="detail-meta">
          {#if editingId === $selectedPlaylist.id}
            <input
              class="edit-input title-input"
              bind:value={editName}
              onkeydown={(e) => e.key === "Enter" && saveEdit()}
            />
            <input
              class="edit-input desc-input"
              bind:value={editDesc}
              placeholder="Description"
              onkeydown={(e) => e.key === "Enter" && saveEdit()}
            />
            <div class="edit-actions">
              <button class="small-btn" onclick={saveEdit}>Save</button>
              <button class="small-btn secondary" onclick={cancelEdit}
                >Cancel</button
              >
            </div>
          {:else}
            <h1 class="detail-name">{$selectedPlaylist.name}</h1>
            {#if !selectedIsFavorites && $selectedPlaylist.description}
              <p class="detail-desc">{$selectedPlaylist.description}</p>
            {/if}
            <p class="detail-info text-3">
              {$selectedPlaylistItems.length} songs
              {#if playlistDurationMS > 0}
                &middot; {totalDuration(playlistDurationMS)}
              {/if}
            </p>
          {/if}
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
        {#if !selectedIsFavorites}
          <button
            class="action-btn"
            onclick={() => startEdit($selectedPlaylist)}>Edit</button
          >
          <button
            class="action-btn danger"
            onclick={() => {
              if (confirm("Delete this playlist?") && $selectedPlaylist) {
                deletePlaylist($selectedPlaylist.id);
                removeRecentPlaylist($selectedPlaylist.id);
              }
            }}
          >
            Delete
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
          {filter ? "No matching tracks." : "This playlist is empty."}
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
                hideFavoriteIcon={selectedIsFavorites}
                playlists={$playlists}
                onplay={() => handlePlay(item)}
                onselect={() => {}}
                onaddtoqueue={() => {}}
                onremove={() => handleRemove(item)}
                onaddtoplaylist={(t, id) => handleAddToPlaylist(t, id)}
                onToggleFavorite={toggleFavoriteTrack}
              />
            </div>
          {/each}
        </div>
      {/if}
    </div>
  </div>
{:else}
  <div class="playlist-list">
    <div class="list-header">
      <h2>Playlists</h2>
      <div class="list-header-actions">
        <button class="new-btn" onclick={() => (showGenerateDialog = true)}>
          Generate Random
        </button>
        <button class="new-btn" onclick={() => (showNewDialog = true)}>
          + New Playlist
        </button>
      </div>
    </div>

    {#if showNewDialog}
      <div class="new-dialog">
        <input
          class="new-input"
          placeholder="Playlist name"
          bind:value={newName}
          onkeydown={(e) => e.key === "Enter" && handleCreate()}
        />
        <input
          class="new-input"
          placeholder="Description (optional)"
          bind:value={newDesc}
          onkeydown={(e) => e.key === "Enter" && handleCreate()}
        />
        <div class="new-actions">
          <button class="small-btn" onclick={handleCreate}>Create</button>
          <button class="small-btn secondary" onclick={handleCancelCreate}>
            Cancel
          </button>
        </div>
      </div>
    {/if}

    {#if showGenerateDialog}
      <div class="new-dialog">
        <input
          class="new-input"
          placeholder="Playlist name"
          bind:value={generateName}
          onkeydown={(e) => e.key === "Enter" && handleGenerate()}
        />
        <input
          class="new-input"
          placeholder="Description (optional)"
          bind:value={generateDesc}
          onkeydown={(e) => e.key === "Enter" && handleGenerate()}
        />
        <div class="generate-duration-row">
          <label class="generate-label" for="gen-duration">Duration</label>
          <input
            id="gen-duration"
            type="number"
            class="new-input gen-duration-input"
            min="1"
            max="999"
            bind:value={generateDuration}
          />
          <span class="generate-unit">minutes</span>
        </div>
        <div class="new-actions">
          <button
            class="small-btn"
            onclick={handleGenerate}
            disabled={generating ||
              !generateName.trim() ||
              generateDuration < 1}
          >
            {generating ? "Generating..." : "Generate"}
          </button>
          <button class="small-btn secondary" onclick={handleCancelGenerate}>
            Cancel
          </button>
        </div>
      </div>
    {/if}

    {#if visiblePlaylists.length === 0 && !showNewDialog}
      <p class="text-3 empty-msg">
        No playlists yet. Create one to get started.
      </p>
    {:else}
      <div class="pl-grid">
        {#each visiblePlaylists as pl (pl.id)}
          <button class="pl-card" onclick={() => openPlaylist(pl)}>
            <div class="pl-art">
              {#if pl.artwork_path}
                <img
                  src={playlistArtUrl(pl.id, pl.updated_at)}
                  alt=""
                  onerror={hideImg}
                />
              {/if}
              <span class="art-placeholder"><Music size={40} /></span>
            </div>
            <div class="pl-info">
              <span class="pl-name truncate">{pl.name}</span>
              <span class="pl-meta text-3">
                {pl.item_count} song{pl.item_count !== 1 ? "s" : ""}
                {#if pl.total_duration_ms}
                  &middot; {totalDuration(pl.total_duration_ms)}
                {/if}
              </span>
            </div>
          </button>
        {/each}
      </div>
    {/if}
  </div>
{/if}

<style>
  .playlist-list {
    padding: 0;
  }

  .list-header {
    display: flex;
    align-items: center;
    justify-content: space-between;
    margin-bottom: 20px;
  }
  .list-header h2 {
    margin: 0;
    font-size: 20px;
    font-weight: 700;
  }

  .list-header-actions {
    display: flex;
    gap: 8px;
  }

  .new-btn {
    padding: 6px 14px;
    border-radius: var(--r-md);
    background: var(--accent-dim);
    color: #fff;
    font-size: 13px;
    font-weight: 600;
    cursor: pointer;
  }

  .new-btn:hover {
    filter: brightness(1.1);
  }

  .new-dialog {
    background: var(--surface);
    border: 1px solid var(--border);
    border-radius: var(--r-md);
    padding: 16px;
    margin-bottom: 20px;
    display: flex;
    flex-direction: column;
    gap: 8px;
    max-width: 400px;
  }
  .new-input {
    padding: 8px 12px;
    border-radius: var(--r-sm);
    border: 1px solid var(--border);
    background: var(--bg);
    color: var(--text-1);
    font-size: 13px;
  }
  .new-actions {
    display: flex;
    gap: 8px;
  }

  .generate-duration-row {
    display: flex;
    align-items: center;
    gap: 8px;
    margin: 12px 0;
  }
  .generate-label {
    font-size: 12px;
    color: var(--text-2);
    text-transform: uppercase;
    font-weight: 600;
  }
  .gen-duration-input {
    width: 80px;
  }
  .generate-unit {
    font-size: 12px;
    color: var(--text-3);
  }

  .pl-grid {
    display: grid;
    grid-template-columns: repeat(auto-fill, minmax(200px, 1fr));
    gap: 16px;
  }

  .pl-card {
    display: flex;
    flex-direction: column;
    background: var(--surface);
    border-radius: var(--r-md);
    overflow: hidden;
    cursor: pointer;
    transition: background 0.1s;
    text-align: left;
    border: 1px solid var(--border);
  }
  .pl-card:hover {
    background: var(--surface-hover);
  }

  .pl-art {
    aspect-ratio: 1;
    background: var(--surface-2);
    display: flex;
    align-items: center;
    justify-content: center;
    font-size: 48px;
    color: var(--text-3);
    position: relative;
    overflow: hidden;
  }
  .pl-art img {
    position: absolute;
    inset: 0;
    width: 100%;
    height: 100%;
    object-fit: cover;
  }

  .pl-info {
    padding: 12px;
  }
  .pl-name {
    display: block;
    font-size: 14px;
    font-weight: 600;
    color: var(--text-1);
  }
  .pl-meta {
    display: block;
    font-size: 12px;
    margin-top: 4px;
  }

  .playlist-detail {
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
    position: relative;
    overflow: hidden;
    cursor: pointer;
    padding: 0;
    border: none;
  }
  .detail-art:disabled {
    cursor: default !important;
  }
  .detail-art.favorites-art {
    cursor: default !important;
  }
  .detail-art img {
    position: absolute;
    inset: 0;
    width: 100%;
    height: 100%;
    object-fit: cover;
  }
  .art-overlay {
    position: absolute;
    inset: 0;
    display: flex;
    align-items: center;
    justify-content: center;
    background: rgba(0, 0, 0, 0.5);
    opacity: 0;
    transition: opacity 0.15s;
    color: var(--text-1);
    z-index: 2;
  }
  .detail-art:hover .art-overlay {
    opacity: 1;
  }
  .detail-art:disabled:hover .art-overlay {
    opacity: 0;
  }
  .detail-art.favorites-art:hover .art-overlay {
    opacity: 0;
  }

  .art-placeholder {
    font-size: 48px;
    color: var(--text-3);
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
  .detail-desc {
    margin: 4px 0 0;
    font-size: 13px;
    color: var(--text-2);
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
    color: #fff;
    border: none;
  }

  .action-btn.primary:hover {
    filter: brightness(1.1);
  }

  .action-btn.danger {
    color: var(--danger);
  }

  .action-btn.danger:hover {
    background: rgba(231, 76, 60, 0.1);
  }

  .action-btn:disabled {
    opacity: 0.4;
    cursor: default;
  }

  .edit-input {
    padding: 6px 10px;
    border-radius: var(--r-sm);
    border: 1px solid var(--border);
    background: var(--bg);
    color: var(--text-1);
    font-size: 13px;
    width: 300px;
  }
  .title-input {
    font-size: 20px;
    font-weight: 700;
  }
  .edit-actions {
    display: flex;
    gap: 6px;
    margin-top: 6px;
  }

  .small-btn {
    padding: 5px 12px;
    border-radius: var(--r-sm);
    font-size: 12px;
    font-weight: 600;
    cursor: pointer;
    background: var(--accent-dim);
    color: #fff;
    border: none;
  }

  .small-btn.secondary {
    background: var(--surface);
    color: var(--text-1);
    border: 1px solid var(--border);
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

  .track-headers {
    display: grid;
    grid-template-columns: 32px 2fr 1fr 1fr 76px;
    gap: 0 12px;
    padding: 4px 12px;
    font-size: 11px;
    font-weight: 600;
    text-transform: uppercase;
    letter-spacing: 0.05em;
    color: var(--text-3);
    border-bottom: 1px solid var(--border);
  }

  .track-headers .num {
    text-align: right;
  }

  .track-list {
    flex: 1;
    min-height: 0;
    overflow-y: auto;
  }
</style>
