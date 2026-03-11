<script lang="ts">
  import { onMount } from "svelte"
  import { derived } from "svelte/store"
  import {
    playlists, selectedPlaylistId, selectedPlaylist,
    selectedPlaylistItems, playlistsLoading,
    loadPlaylists, createPlaylist, deletePlaylist,
    selectPlaylist, playPlaylist, removePlaylistItem,
    uploadPlaylist, updatePlaylist,
    pickPlaylistArtwork,
    type PlaylistItem, type PlaylistSummary,
  } from "../stores/playlists"
  import { selectedPlaylistView, pushNav } from "../stores/ui"
  import { playerState, type Track } from "../stores/player"
  import { connected, playlistArtUrl } from "../utils/api"
  import TrackRow from "./TrackRow.svelte"

  const currentTrackId = derived(playerState, $s => $s.trackId)

  // ─── List / detail view state ──────────────────────────────────

  let showNewDialog = false
  let newName = ""
  let newDesc = ""
  let editingId: string | null = null
  let editName = ""
  let editDesc = ""

  // Track the playlist view from ui store
  $: if ($selectedPlaylistView) {
    selectPlaylist($selectedPlaylistView)
  } else {
    selectedPlaylistId.set(null)
    selectedPlaylist.set(null)
    selectedPlaylistItems.set([])
  }

  // ─── Detail: sort & filter ─────────────────────────────────────

  let filter = ""
  type SortField = "default" | "title" | "artist" | "duration"
  let sortField: SortField = "default"
  let sortDir: "asc" | "desc" = "asc"

  function toggleSort(field: SortField) {
    if (sortField === field) {
      sortDir = sortDir === "asc" ? "desc" : "asc"
    } else {
      sortField = field
      sortDir = "asc"
    }
  }

  function sortIndicator(field: SortField): string {
    return sortField === field ? (sortDir === "asc" ? " ↑" : " ↓") : ""
  }

  $: filteredItems =
    $selectedPlaylistItems
      .filter(i => {
        if (!filter) return true
        const q = filter.toLowerCase()
        return i.ref_title.toLowerCase().includes(q) ||
               i.ref_album.toLowerCase().includes(q) ||
               i.ref_album_artist.toLowerCase().includes(q)
      })
      .sort((a, b) => {
        if (sortField === "default") return a.position - b.position
        let cmp = 0
        if (sortField === "title") cmp = a.ref_title.localeCompare(b.ref_title)
        else if (sortField === "artist") cmp = a.ref_album_artist.localeCompare(b.ref_album_artist)
        else if (sortField === "duration") cmp = a.ref_duration_ms - b.ref_duration_ms
        return sortDir === "desc" ? -cmp : cmp
      })

  // Build Track from PlaylistItem for TrackRow
  function itemToTrack(item: PlaylistItem): Track {
    const id = item.source === "local_ref" && item.local_path
      ? item.local_path
      : item.track_id || `missing-${item.position}`
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
      artwork_id: "",
    }
  }

  // ─── Handlers ──────────────────────────────────────────────────

  async function handleCreate() {
    if (!newName.trim()) return
    const id = await createPlaylist(newName.trim(), newDesc.trim())
    newName = ""
    newDesc = ""
    showNewDialog = false
    if (id) {
      pushNav({ view: "playlists", playlistId: id })
    }
  }

  function openPlaylist(pl: PlaylistSummary) {
    pushNav({ view: "playlists", playlistId: pl.id, albumKey: null })
  }

  function handlePlay(item: PlaylistItem) {
    const idx = $selectedPlaylistItems.findIndex(i => i.position === item.position)
    playPlaylist($selectedPlaylistItems, idx >= 0 ? idx : 0, $selectedPlaylistId ?? undefined)
  }

  async function handleRemove(item: PlaylistItem) {
    if ($selectedPlaylistId) {
      await removePlaylistItem($selectedPlaylistId, item.position)
    }
  }

  async function handleUpload() {
    if ($selectedPlaylistId) {
      await uploadPlaylist($selectedPlaylistId)
    }
  }

  function startEdit(pl: PlaylistSummary) {
    editingId = pl.id
    editName = pl.name
    editDesc = pl.description
  }

  async function saveEdit() {
    if (editingId && editName.trim()) {
      await updatePlaylist(editingId, editName.trim(), editDesc.trim())
      editingId = null
    }
  }

  function cancelEdit() {
    editingId = null
  }

  function totalDuration(ms: number): string {
    const totalMin = Math.floor(ms / 60000)
    if (totalMin < 60) return `${totalMin} min`
    const h = Math.floor(totalMin / 60)
    const m = totalMin % 60
    return `${h} hr ${m} min`
  }

  onMount(() => {
    loadPlaylists()
  })
</script>

{#if $selectedPlaylistView && $selectedPlaylist}
  <div class="playlist-detail">
    <div class="detail-header">
      <div class="detail-hero">
        <button class="detail-art" on:click={() => $selectedPlaylistId && pickPlaylistArtwork($selectedPlaylistId)} title="Change artwork">
          {#if $selectedPlaylist.artwork_path}
            <img src={playlistArtUrl($selectedPlaylist.artwork_path)} alt="" on:error={(e) => { (e.currentTarget as HTMLImageElement).style.display = 'none' }} />
          {/if}
          <span class="art-placeholder">♫</span>
          <div class="art-overlay">
            <svg viewBox="0 0 24 24" width="24" height="24" fill="none" stroke="currentColor" stroke-width="2" stroke-linecap="round" stroke-linejoin="round">
              <path d="M12 20h9"/><path d="M16.5 3.5a2.121 2.121 0 0 1 3 3L7 19l-4 1 1-4 12.5-12.5z"/>
            </svg>
          </div>
        </button>
        <div class="detail-meta">
          {#if editingId === $selectedPlaylist.id}
            <input class="edit-input title-input" bind:value={editName} on:keydown={(e) => e.key === 'Enter' && saveEdit()} />
            <input class="edit-input desc-input" bind:value={editDesc} placeholder="Description" on:keydown={(e) => e.key === 'Enter' && saveEdit()} />
            <div class="edit-actions">
              <button class="small-btn" on:click={saveEdit}>Save</button>
              <button class="small-btn secondary" on:click={cancelEdit}>Cancel</button>
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
        <button class="action-btn primary" on:click={() => playPlaylist($selectedPlaylistItems, 0, $selectedPlaylistId ?? undefined)} disabled={$selectedPlaylistItems.length === 0}>
          Play
        </button>
        <button class="action-btn" on:click={() => startEdit($selectedPlaylist)}>Edit</button>
        {#if $connected && !$selectedPlaylist.remote_playlist_id}
          <button class="action-btn" on:click={handleUpload}>Upload to Server</button>
        {:else if $connected && $selectedPlaylist.remote_playlist_id}
          <button class="action-btn" on:click={handleUpload}>Sync to Server</button>
        {/if}
        <button class="action-btn danger" on:click={() => { if (confirm('Delete this playlist?')) deletePlaylist($selectedPlaylist.id) }}>
          Delete
        </button>
        <div class="filter-spacer"></div>
        <input type="text" class="filter-input" placeholder="Filter tracks…" bind:value={filter} />
      </div>
    </div>

    <div class="track-headers">
      <span class="num">#</span>
      <button class="sortable" on:click={() => toggleSort("title")}>Title{sortIndicator("title")}</button>
      <button class="sortable" on:click={() => toggleSort("artist")}>Artist{sortIndicator("artist")}</button>
      <span>Album</span>
      <button class="sortable" on:click={() => toggleSort("duration")}>Duration{sortIndicator("duration")}</button>
    </div>

    <div class="track-list">
      {#if $playlistsLoading}
        <p class="text-3 loading-msg">Loading…</p>
      {:else if filteredItems.length === 0}
        <p class="text-3 empty-msg">
          {filter ? "No matching tracks." : "This playlist is empty. Add tracks from the library."}
        </p>
      {:else}
        {#each filteredItems as item (item.position)}
          {@const track = itemToTrack(item)}
          <div class="playlist-row" class:missing={item.missing}>
            <TrackRow
              {track}
              active={$currentTrackId === track.id}
              on:play={() => handlePlay(item)}
              on:addToQueue
            />
            <button class="remove-btn" on:click={() => handleRemove(item)} title="Remove from playlist">×</button>
          </div>
        {/each}
      {/if}
    </div>
  </div>

{:else}
  <div class="playlist-list">
    <div class="list-header">
      <h2>Playlists</h2>
      <button class="new-btn" on:click={() => showNewDialog = true}>+ New Playlist</button>
    </div>

    {#if showNewDialog}
      <div class="new-dialog">
        <!-- svelte-ignore a11y_autofocus -->
        <input class="new-input" placeholder="Playlist name" bind:value={newName} on:keydown={(e) => e.key === 'Enter' && handleCreate()} autofocus />
        <input class="new-input" placeholder="Description (optional)" bind:value={newDesc} on:keydown={(e) => e.key === 'Enter' && handleCreate()} />
        <div class="new-actions">
          <button class="small-btn" on:click={handleCreate}>Create</button>
          <button class="small-btn secondary" on:click={() => { showNewDialog = false; newName = ''; newDesc = '' }}>Cancel</button>
        </div>
      </div>
    {/if}

    {#if $playlists.length === 0}
      <p class="text-3 empty-msg">No playlists yet. Create one to get started.</p>
    {:else}
      <div class="pl-grid">
        {#each $playlists as pl (pl.id)}
          <button class="pl-card" on:click={() => openPlaylist(pl)}>
            <div class="pl-art">
              {#if pl.artwork_path}
                <img src={playlistArtUrl(pl.artwork_path)} alt="" on:error={(e) => { (e.currentTarget as HTMLImageElement).style.display = 'none' }} />
              {/if}
              <span class="art-placeholder">♫</span>
            </div>
            <div class="pl-info">
              <span class="pl-name truncate">{pl.name}</span>
              <span class="pl-meta text-3">
                {pl.item_count} song{pl.item_count !== 1 ? 's' : ''}
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
  .playlist-list { padding: 0; }

  .list-header {
    display: flex;
    align-items: center;
    justify-content: space-between;
    margin-bottom: 20px;
  }
  .list-header h2 { margin: 0; font-size: 20px; font-weight: 700; }

  .new-btn {
    padding: 6px 14px;
    border-radius: var(--r-md);
    background: var(--accent);
    color: #fff;
    font-size: 13px;
    font-weight: 600;
    cursor: pointer;
  }
  .new-btn:hover { filter: brightness(1.1); }

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
  .new-actions { display: flex; gap: 8px; }

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
  .pl-card:hover { background: var(--surface-hover); }

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

  .pl-info { padding: 12px; }
  .pl-name { display: block; font-size: 14px; font-weight: 600; color: var(--text-1); }
  .pl-meta { display: block; font-size: 12px; margin-top: 4px; }

  .playlist-detail { display: flex; flex-direction: column; height: 100%; }

  .detail-header { margin-bottom: 16px; }

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
    background: rgba(0,0,0,0.5);
    display: flex;
    align-items: center;
    justify-content: center;
    opacity: 0;
    transition: opacity 0.15s;
    color: #fff;
  }
  .detail-art:hover .art-overlay { opacity: 1; }

  .art-placeholder { font-size: 48px; color: var(--text-3); }

  .detail-meta { display: flex; flex-direction: column; justify-content: flex-end; }

  .detail-name { margin: 0; font-size: 28px; font-weight: 700; }
  .detail-desc { margin: 4px 0 0; font-size: 13px; color: var(--text-2); }
  .detail-info { margin: 8px 0 0; font-size: 12px; }

  .detail-actions {
    display: flex;
    align-items: center;
    gap: 8px;
    margin-top: 12px;
  }

  .filter-spacer { flex: 1; }

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
  .action-btn:hover { background: var(--surface-hover); }
  .action-btn.primary { background: var(--accent); color: #fff; border: none; }
  .action-btn.primary:hover { filter: brightness(1.1); }
  .action-btn.danger { color: #e74c3c; }
  .action-btn.danger:hover { background: rgba(231, 76, 60, 0.1); }
  .action-btn:disabled { opacity: 0.4; cursor: default; }

  .edit-input {
    padding: 6px 10px;
    border-radius: var(--r-sm);
    border: 1px solid var(--border);
    background: var(--bg);
    color: var(--text-1);
    font-size: 13px;
    width: 300px;
  }
  .title-input { font-size: 20px; font-weight: 700; }
  .edit-actions { display: flex; gap: 6px; margin-top: 6px; }

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
  .small-btn.secondary { background: var(--surface); color: var(--text-1); border: 1px solid var(--border); }

  .track-headers {
    display: grid;
    grid-template-columns: 32px 2fr 1fr 1fr 56px;
    gap: 0 12px;
    padding: 4px 12px;
    font-size: 11px;
    font-weight: 600;
    text-transform: uppercase;
    letter-spacing: 0.05em;
    color: var(--text-3);
    border-bottom: 1px solid var(--border);
  }
  .track-headers .num { text-align: right; }
  .track-headers .sortable {
    cursor: pointer;
    color: var(--text-3);
    text-align: left;
    font-size: inherit;
    font-weight: inherit;
    text-transform: inherit;
    letter-spacing: inherit;
  }
  .track-headers .sortable:hover { color: var(--text-1); }

  .filter-input {
    width: 200px;
    padding: 6px 10px;
    border-radius: var(--r-sm);
    border: 1px solid var(--border);
    background: var(--bg);
    color: var(--text-1);
    font-size: 12px;
  }

  .track-list { flex: 1; overflow-y: auto; }

  .playlist-row {
    display: flex;
    align-items: center;
  }
  .playlist-row :global(.track-row) { flex: 1; }
  .playlist-row.missing { opacity: 0.45; }
  .playlist-row.missing :global(.track-row) { pointer-events: none; }

  .remove-btn {
    width: 28px;
    height: 28px;
    display: flex;
    align-items: center;
    justify-content: center;
    border-radius: 50%;
    font-size: 16px;
    color: var(--text-3);
    opacity: 0;
    transition: opacity 0.1s, background 0.1s;
    cursor: pointer;
    flex-shrink: 0;
  }
  .playlist-row:hover .remove-btn { opacity: 1; }
  .remove-btn:hover { background: var(--surface-hover); color: var(--text-1); }

  .empty-msg { padding: 40px 12px; text-align: center; }
  .loading-msg { padding: 20px 12px; text-align: center; }
</style>
