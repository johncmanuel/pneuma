<script lang="ts">
  import { onMount } from "svelte"
  import { apiFetch, currentUser, artworkUrl } from "../api"

  interface Track {
    id: string
    title: string
    album_artist: string
    album_name: string
    duration_ms: number
    uploaded_by_user_id: string
    created_at: string
  }

  let tracks: Track[] = []
  let loading = false
  let searchQuery = ""

  $: canUpload = $currentUser?.is_admin || $currentUser?.can_upload
  $: canEdit = $currentUser?.is_admin || $currentUser?.can_edit
  $: canDelete = $currentUser?.is_admin || $currentUser?.can_delete

  $: filteredTracks = searchQuery.trim()
    ? tracks.filter(
        (t) =>
          t.title?.toLowerCase().includes(searchQuery.toLowerCase()) ||
          t.album_artist?.toLowerCase().includes(searchQuery.toLowerCase()) ||
          t.album_name?.toLowerCase().includes(searchQuery.toLowerCase()),
      )
    : tracks

  onMount(loadTracks)

  async function loadTracks() {
    loading = true
    try {
      const r = await apiFetch("/api/library/tracks")
      if (r.ok) tracks = await r.json()
    } finally {
      loading = false
    }
  }

  let editingTrack: Track | null = null
  let editTitle = ""
  let editArtist = ""
  let editAlbum = ""

  function startEdit(t: Track) {
    editingTrack = t
    editTitle = t.title
    editArtist = t.album_artist
    editAlbum = t.album_name
  }

  async function saveEdit() {
    if (!editingTrack) return
    const r = await apiFetch(`/api/library/tracks/${editingTrack.id}`, {
      method: "PATCH",
      body: JSON.stringify({
        title: editTitle,
        album_artist: editArtist,
        album_name: editAlbum,
      }),
    })
    if (r.ok) {
      await loadTracks()
      editingTrack = null
    }
  }

  async function deleteTrack(id: string) {
    if (!confirm("Delete this track?")) return
    const r = await apiFetch(`/api/library/tracks/${id}`, { method: "DELETE" })
    if (r.ok) await loadTracks()
  }

  // Upload
  let fileInput: HTMLInputElement
  let uploading = false

  async function handleUpload() {
    if (!fileInput?.files?.length) return
    uploading = true
    try {
      for (const file of fileInput.files) {
        const form = new FormData()
        form.append("file", file)
        await apiFetch("/api/library/tracks/upload", {
          method: "POST",
          body: form,
          headers: {}, // let browser set Content-Type with boundary
        })
      }
      await loadTracks()
    } finally {
      uploading = false
      if (fileInput) fileInput.value = ""
    }
  }

  async function triggerScan() {
    await apiFetch("/api/library/scan", { method: "POST" })
  }

  function formatDuration(ms: number): string {
    const s = Math.floor(ms / 1000)
    const m = Math.floor(s / 60)
    return `${m}:${String(s % 60).padStart(2, "0")}`
  }
</script>

<div class="panel">
  <div class="actions-bar">
    <input
      type="text"
      placeholder="Search tracks…"
      bind:value={searchQuery}
      class="search-input"
    />
    {#if canUpload}
      <input
        type="file"
        accept="audio/*"
        multiple
        bind:this={fileInput}
        on:change={handleUpload}
        style="display:none"
      />
      <button class="action-btn" on:click={() => fileInput?.click()} disabled={uploading}>
        {uploading ? "Uploading…" : "↑ Upload"}
      </button>
    {/if}
    {#if $currentUser?.is_admin}
      <button class="action-btn" on:click={triggerScan}>↺ Scan</button>
    {/if}
  </div>

  {#if loading}
    <p class="text-3">Loading…</p>
  {:else}
    <div class="table-wrap">
      <table>
        <thead>
          <tr>
            <th>Title</th>
            <th>Artist</th>
            <th>Album</th>
            <th>Duration</th>
            {#if canEdit || canDelete}
              <th>Actions</th>
            {/if}
          </tr>
        </thead>
        <tbody>
          {#each filteredTracks as t (t.id)}
            <tr>
              {#if editingTrack?.id === t.id}
                <td><input type="text" bind:value={editTitle} class="inline-input" /></td>
                <td><input type="text" bind:value={editArtist} class="inline-input" /></td>
                <td><input type="text" bind:value={editAlbum} class="inline-input" /></td>
                <td>{formatDuration(t.duration_ms)}</td>
                <td class="action-cell">
                  <button class="sm-btn save" on:click={saveEdit}>Save</button>
                  <button class="sm-btn" on:click={() => { editingTrack = null }}>Cancel</button>
                </td>
              {:else}
                <td class="truncate">{t.title}</td>
                <td class="truncate text-2">{t.album_artist || "–"}</td>
                <td class="truncate text-2">{t.album_name || "–"}</td>
                <td class="text-3">{formatDuration(t.duration_ms)}</td>
                {#if canEdit || canDelete}
                  <td class="action-cell">
                    {#if canEdit}
                      <button class="sm-btn" on:click={() => startEdit(t)}>Edit</button>
                    {/if}
                    {#if canDelete}
                      <button class="sm-btn danger" on:click={() => deleteTrack(t.id)}>Delete</button>
                    {/if}
                  </td>
                {/if}
              {/if}
            </tr>
          {/each}
        </tbody>
      </table>
    </div>
  {/if}
</div>

<style>
  .panel { display: flex; flex-direction: column; gap: 12px; }

  .actions-bar {
    display: flex;
    gap: 8px;
    align-items: center;
    flex-wrap: wrap;
  }

  .search-input {
    max-width: 280px;
  }

  .action-btn {
    padding: 6px 14px;
    border-radius: var(--r-md);
    background: var(--surface-2);
    border: 1px solid var(--border);
    font-size: 13px;
    white-space: nowrap;
  }
  .action-btn:hover { background: var(--surface-hover); }

  .table-wrap { overflow-x: auto; }

  table {
    width: 100%;
    border-collapse: collapse;
    font-size: 13px;
  }

  th {
    text-align: left;
    padding: 8px 12px;
    font-size: 11px;
    text-transform: uppercase;
    letter-spacing: 0.05em;
    color: var(--text-3);
    border-bottom: 1px solid var(--border);
  }

  td {
    padding: 6px 12px;
    border-bottom: 1px solid var(--border);
    max-width: 200px;
  }

  tr:hover { background: var(--surface-hover); }

  .action-cell {
    display: flex;
    gap: 6px;
    white-space: nowrap;
  }

  .sm-btn {
    padding: 3px 10px;
    border-radius: var(--r-sm);
    font-size: 12px;
    background: var(--surface-2);
    border: 1px solid var(--border);
  }
  .sm-btn:hover { background: var(--surface-hover); }
  .sm-btn.save { color: var(--accent); border-color: var(--accent-dim); }
  .sm-btn.danger { color: var(--danger); }
  .sm-btn.danger:hover { background: rgba(248, 113, 113, 0.1); }

  .inline-input {
    padding: 2px 6px;
    font-size: 13px;
    width: 100%;
  }
</style>
