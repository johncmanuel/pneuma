<script lang="ts">
  import { onMount } from "svelte";
  import { createVirtualizer } from "@tanstack/svelte-virtual";
  import { derived } from "svelte/store";
  import {
    playlists,
    selectedPlaylistId,
    selectedPlaylist,
    selectedPlaylistItems,
    playlistsLoading,
    loadPlaylists,
    createPlaylist,
    deletePlaylist,
    selectPlaylist,
    playPlaylist,
    removePlaylistItem,
    uploadPlaylist,
    updatePlaylist,
    pickPlaylistArtwork,
    type PlaylistItem,
    type PlaylistSummary
  } from "../stores/playlists";
  import { selectedPlaylistView, pushNav } from "../stores/ui";
  import { playerState, type Track } from "../stores/player";
  import { connected, playlistArtUrl } from "../utils/api";
  import { Music, SquarePen } from "@lucide/svelte";
  import TrackRow from "./TrackRow.svelte";
  import SortButton from "./SortButton.svelte";
  import "../assets/css/track-list.css";

  const currentTrackId = derived(playerState, ($s) => $s.trackId);

  let showNewDialog = false;
  let newName = "";
  let newDesc = "";
  let editingId: string | null = null;
  let editName = "";
  let editDesc = "";
  let trackListEl: HTMLDivElement;

  $: if ($selectedPlaylistView) {
    selectPlaylist($selectedPlaylistView);
  } else {
    selectedPlaylistId.set(null);
    selectedPlaylist.set(null);
    selectedPlaylistItems.set([]);
  }

  let filter = "";
  type SortField = "default" | "title" | "added_at" | "duration";
  let sortField: SortField = "default";
  let sortDir: "asc" | "desc" = "asc";

  $: virtualizer = createVirtualizer<HTMLDivElement, HTMLDivElement>({
    count: filteredItems.length,
    getScrollElement: () => trackListEl,
    estimateSize: () => 38,
    overscan: 5
  });

  $: filteredItems = $selectedPlaylistItems
    .filter((i) => {
      if (!filter) return true;
      const q = filter.toLowerCase();
      return (
        i.ref_title.toLowerCase().includes(q) ||
        i.ref_album.toLowerCase().includes(q) ||
        i.ref_album_artist.toLowerCase().includes(q)
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
    });

  function itemToTrack(item: PlaylistItem): Track {
    const id =
      item.source === "local_ref" && item.local_path
        ? item.local_path
        : item.track_id || `missing-${item.position}`;
    return {
      id,
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
      replay_gain_track: 0,
      artwork_id: ""
    };
  }

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

  function openPlaylist(pl: PlaylistSummary) {
    pushNav({ view: "playlists", playlistId: pl.id, albumKey: null });
  }

  function handlePlay(item: PlaylistItem) {
    const idx = $selectedPlaylistItems.findIndex(
      (i) => i.position === item.position
    );
    playPlaylist(
      $selectedPlaylistItems,
      idx >= 0 ? idx : 0,
      $selectedPlaylistId ?? undefined
    );
  }

  async function handleRemove(item: PlaylistItem) {
    if ($selectedPlaylistId) {
      await removePlaylistItem($selectedPlaylistId, item.position);
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

  async function handleUpload() {
    if ($selectedPlaylistId) {
      await uploadPlaylist($selectedPlaylistId);
    }
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

  function totalDuration(ms: number): string {
    const totalMin = Math.floor(ms / 60000);
    if (totalMin < 60) return `${totalMin} min`;

    const h = Math.floor(totalMin / 60);
    const m = totalMin % 60;
    return `${h} hr ${m} min`;
  }

  onMount(() => {
    loadPlaylists();
  });
</script>

{#if $selectedPlaylistView && $selectedPlaylist}
  <div class="playlist-detail">
    <div class="detail-header">
      <div class="detail-hero">
        <button
          class="detail-art"
          on:click={() =>
            $selectedPlaylistId && pickPlaylistArtwork($selectedPlaylistId)}
          title="Change artwork"
        >
          {#if $selectedPlaylist.artwork_path}
            <img
              src={playlistArtUrl($selectedPlaylist.artwork_path)}
              alt=""
              on:error={(e) => {
                (e.currentTarget as HTMLImageElement).style.display = "none";
              }}
            />
          {/if}
          <span class="art-placeholder"><Music size={24} /></span>
          <div class="art-overlay">
            <SquarePen size={24} />
          </div>
        </button>
        <div class="detail-meta">
          {#if editingId === $selectedPlaylist.id}
            <input
              class="edit-input title-input"
              bind:value={editName}
              on:keydown={(e) => e.key === "Enter" && saveEdit()}
            />
            <input
              class="edit-input desc-input"
              bind:value={editDesc}
              placeholder="Description"
              on:keydown={(e) => e.key === "Enter" && saveEdit()}
            />
            <div class="edit-actions">
              <button class="small-btn" on:click={saveEdit}>Save</button>
              <button class="small-btn secondary" on:click={cancelEdit}
                >Cancel</button
              >
            </div>
          {:else}
            <h1 class="detail-name">{$selectedPlaylist.name}</h1>
            {#if $selectedPlaylist.description}
              <p class="detail-desc">{$selectedPlaylist.description}</p>
            {/if}
            <p class="detail-info text-3">
              {$selectedPlaylistItems.length} songs
              {#if $selectedPlaylist.total_duration_ms}
                &middot; {totalDuration($selectedPlaylist.total_duration_ms)}
              {/if}
            </p>
          {/if}
        </div>
      </div>

      <div class="detail-actions">
        <button
          class="action-btn primary"
          on:click={() =>
            playPlaylist(
              $selectedPlaylistItems,
              0,
              $selectedPlaylistId ?? undefined
            )}
          disabled={$selectedPlaylistItems.length === 0}
        >
          Play
        </button>
        <button class="action-btn" on:click={() => startEdit($selectedPlaylist)}
          >Edit</button
        >
        {#if $connected && !$selectedPlaylist.remote_playlist_id}
          <button class="action-btn" on:click={handleUpload}
            >Upload to Server</button
          >
        {:else if $connected && $selectedPlaylist.remote_playlist_id}
          <button class="action-btn" on:click={handleUpload}
            >Sync to Server</button
          >
        {/if}
        <button
          class="action-btn danger"
          on:click={() => {
            if (confirm("Delete this playlist?"))
              deletePlaylist($selectedPlaylist.id);
          }}
        >
          Delete
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
          {filter
            ? "No matching tracks."
            : "This playlist is empty. Add tracks from the library."}
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
                onplay={() => handlePlay(item)}
                onaddtoqueue={(t) => {}}
                onremove={() => handleRemove(item)}
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
      <button class="new-btn" on:click={() => (showNewDialog = true)}
        >+ New Playlist</button
      >
    </div>

    {#if showNewDialog}
      <div class="new-dialog">
        <!-- svelte-ignore a11y_autofocus -->
        <input
          class="new-input"
          placeholder="Playlist name"
          bind:value={newName}
          on:keydown={(e) => e.key === "Enter" && handleCreate()}
          autofocus
        />
        <input
          class="new-input"
          placeholder="Description (optional)"
          bind:value={newDesc}
          on:keydown={(e) => e.key === "Enter" && handleCreate()}
        />
        <div class="new-actions">
          <button class="small-btn" on:click={handleCreate}>Create</button>
          <button
            class="small-btn secondary"
            on:click={() => {
              showNewDialog = false;
              newName = "";
              newDesc = "";
            }}>Cancel</button
          >
        </div>
      </div>
    {/if}

    {#if $playlists.length === 0}
      <p class="text-3 empty-msg">
        No playlists yet. Create one to get started.
      </p>
    {:else}
      <div class="pl-grid">
        {#each $playlists as pl (pl.id)}
          <button class="pl-card" on:click={() => openPlaylist(pl)}>
            <div class="pl-art">
              {#if pl.artwork_path}
                <img
                  src={playlistArtUrl(pl.artwork_path)}
                  alt=""
                  on:error={(e) => {
                    (e.currentTarget as HTMLImageElement).style.display =
                      "none";
                  }}
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
    background: var(--surface-2, var(--surface));
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
    border: none;
    padding: 0;
  }
  .detail-art img {
    position: absolute;
    inset: 0;
    width: 100%;
    height: 100%;
    object-fit: cover;
  }
  .detail-art .art-overlay {
    position: absolute;
    inset: 0;
    background: rgba(0, 0, 0, 0.5);
    display: flex;
    align-items: center;
    justify-content: center;
    opacity: 0;
    transition: opacity 0.15s;
    color: #fff;
  }
  .detail-art:hover .art-overlay {
    opacity: 1;
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
    background: var(--accent);
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
</style>
