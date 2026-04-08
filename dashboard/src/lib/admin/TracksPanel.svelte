<script lang="ts">
  import { onMount, onDestroy } from "svelte";
  import { apiFetch, currentUser } from "../api";
  import { libraryVersion, scanRunning, scanResult } from "../ws";
  import { formatDuration, storageKeys } from "@pneuma/shared";

  interface Track {
    id: string;
    title: string;
    album_artist: string;
    album_name: string;
    duration_ms: number;
    uploaded_by_user_id: string;
    created_at: string;
  }

  type SortKey = "title" | "album_artist" | "album_name" | "duration_ms";
  type SortDir = "asc" | "desc";

  interface UploadItem {
    file: File;
    status:
      | "pending"
      | "uploading"
      | "done"
      | "duplicate"
      | "unsupported"
      | "error";
    error?: string;
  }

  const AUDIO_EXTS = new Set([
    ".mp3",
    ".flac",
    ".ogg",
    ".opus",
    ".m4a",
    ".aac",
    ".wav",
    ".aiff",
    ".wma",
    ".alac",
    ".ape",
    ".wv"
  ]);

  function isAudioFile(name: string): boolean {
    const dot = name.lastIndexOf(".");
    if (dot < 0) return false;
    return AUDIO_EXTS.has(name.slice(dot).toLowerCase());
  }

  let tracks: Track[] = [];
  let loading = false;
  let searchQuery = "";
  let sortKey: SortKey = "title";
  let sortDir: SortDir = "asc";

  let selectedIds = new Set<string>();
  let bulkDeleting = false;

  $: canUpload = $currentUser?.is_admin || $currentUser?.can_upload;
  $: canEdit = $currentUser?.is_admin || $currentUser?.can_edit;
  $: canDelete = $currentUser?.is_admin || $currentUser?.can_delete;

  const STORAGE_KEY = storageKeys.adminTracksPanel;

  function loadPersistedState() {
    try {
      const raw = localStorage.getItem(STORAGE_KEY);
      if (!raw) return;

      const s = JSON.parse(raw) as {
        sortKey?: SortKey;
        sortDir?: SortDir;
        searchQuery?: string;
      };
      if (!s) return;

      if (s.sortKey) sortKey = s.sortKey;
      if (s.sortDir) sortDir = s.sortDir;
      if (typeof s.searchQuery === "string") searchQuery = s.searchQuery;
    } catch (e) {
      console.error("Failed to load persisted tracks panel state", e);
    }
  }

  function persistState() {
    try {
      localStorage.setItem(
        STORAGE_KEY,
        JSON.stringify({ sortKey, sortDir, searchQuery })
      );
    } catch (e) {
      console.error("Failed to persist tracks panel state", e);
    }
  }

  $: (sortKey, sortDir, searchQuery, persistState());

  // Filter tracks based on searchQuery, sortKey, and sortDir.
  $: filteredTracks = searchQuery.trim()
    ? tracks.filter(
        (t) =>
          t.title?.toLowerCase().includes(searchQuery.toLowerCase()) ||
          t.album_artist?.toLowerCase().includes(searchQuery.toLowerCase()) ||
          t.album_name?.toLowerCase().includes(searchQuery.toLowerCase())
      )
    : tracks;

  // Sort the filtered tracks based on sortKey and sortDir.
  $: sortedTracks = [...filteredTracks].sort((a, b) => {
    let cmp = 0;
    switch (sortKey) {
      case "title":
        cmp = (a.title || "").localeCompare(b.title || "", undefined, {
          sensitivity: "base"
        });
        break;
      case "album_artist":
        cmp = (a.album_artist || "").localeCompare(
          b.album_artist || "",
          undefined,
          { sensitivity: "base" }
        );
        break;
      case "album_name":
        cmp = (a.album_name || "").localeCompare(
          b.album_name || "",
          undefined,
          { sensitivity: "base" }
        );
        break;
      case "duration_ms":
        cmp = (a.duration_ms ?? 0) - (b.duration_ms ?? 0);
        break;
    }
    return sortDir === "desc" ? -cmp : cmp;
  });

  function toggleSort(key: SortKey) {
    if (sortKey === key) {
      sortDir = sortDir === "asc" ? "desc" : "asc";
    } else {
      sortKey = key;
      sortDir = "asc";
    }
  }

  function sortIndicator(key: SortKey): string {
    if (sortKey !== key) return "";
    return sortDir === "asc" ? " ↑" : " ↓";
  }

  onMount(() => {
    loadPersistedState();
    loadTracks();
  });

  let _prevLibVer: number | undefined;
  $: {
    const v = $libraryVersion;

    // prevent repetitive reloads when libraryVersion changes rapidly (e.g. during bulk upload)
    if (_prevLibVer !== undefined && v !== _prevLibVer && !uploadActive)
      loadTracks();

    _prevLibVer = v;
  }

  async function loadTracks() {
    loading = true;
    try {
      const r = await apiFetch("/api/library/tracks");
      if (r.ok) tracks = await r.json();
    } finally {
      loading = false;
    }
  }

  let editingTrack: Track | null = null;
  let editTitle = "";
  let editArtist = "";
  let editAlbum = "";

  function startEdit(t: Track) {
    editingTrack = t;
    editTitle = t.title;
    editArtist = t.album_artist;
    editAlbum = t.album_name;
  }

  async function saveEdit() {
    if (!editingTrack) return;
    const r = await apiFetch(`/api/library/tracks/${editingTrack.id}`, {
      method: "PATCH",
      body: JSON.stringify({
        title: editTitle,
        album_artist: editArtist,
        album_name: editAlbum
      })
    });
    if (r.ok) {
      await loadTracks();
      editingTrack = null;
    }
  }

  function cancelEdit() {
    editingTrack = null;
  }

  async function deleteTrack(id: string) {
    if (!confirm("Delete this track?")) return;
    const r = await apiFetch(`/api/library/tracks/${id}`, { method: "DELETE" });
    if (r.ok) await loadTracks();
  }

  $: allSelected =
    sortedTracks.length > 0 && sortedTracks.every((t) => selectedIds.has(t.id));

  function toggleSelectAll() {
    if (allSelected) {
      selectedIds = new Set();
    } else {
      selectedIds = new Set(sortedTracks.map((t) => t.id));
    }
  }

  function toggleSelect(id: string) {
    const next = new Set(selectedIds);
    if (next.has(id)) next.delete(id);
    else next.add(id);
    selectedIds = next;
  }

  async function bulkDelete() {
    const ids = [...selectedIds];
    if (!ids.length) return;
    if (!confirm(`Delete ${ids.length} track${ids.length > 1 ? "s" : ""}?`))
      return;
    bulkDeleting = true;
    let ok = 0;
    let fail = 0;
    for (const id of ids) {
      const r = await apiFetch(`/api/library/tracks/${id}`, {
        method: "DELETE"
      });
      if (r.ok) ok++;
      else fail++;
    }
    bulkDeleting = false;
    selectedIds = new Set();
    await loadTracks();
    if (fail > 0) alert(`Deleted ${ok}, failed ${fail}`);
  }

  let fileInput: HTMLInputElement;
  let folderInput: HTMLInputElement;
  let dragOver = false;

  let uploadQueue: UploadItem[] = [];
  let uploadActive = false;
  let uploadCancelled = false;
  let uploadDone = false;

  // show upload stats
  $: uploadStats = {
    total: uploadQueue.length,
    done: uploadQueue.filter((i) => i.status === "done").length,
    duplicate: uploadQueue.filter((i) => i.status === "duplicate").length,
    unsupported: uploadQueue.filter((i) => i.status === "unsupported").length,
    error: uploadQueue.filter((i) => i.status === "error").length,
    pending: uploadQueue.filter(
      (i) => i.status === "pending" || i.status === "uploading"
    ).length
  };

  function handleFileInput() {
    if (fileInput?.files?.length) {
      enqueueFiles(Array.from(fileInput.files));
      fileInput.value = "";
    }
  }

  function handleFolderInput() {
    if (folderInput?.files?.length) {
      enqueueFiles(Array.from(folderInput.files));
      folderInput.value = "";
    }
  }

  function handleDragOver(e: DragEvent) {
    e.preventDefault();
    if (canUpload) dragOver = true;
  }

  function handleDragLeave() {
    dragOver = false;
  }

  async function handleDrop(e: DragEvent) {
    e.preventDefault();
    dragOver = false;

    if (!canUpload || !e.dataTransfer) return;

    const items = e.dataTransfer.items;
    if (
      items &&
      items.length > 0 &&
      typeof items[0].webkitGetAsEntry === "function"
    ) {
      const entries: FileSystemEntry[] = [];

      for (let i = 0; i < items.length; i++) {
        const entry = items[i].webkitGetAsEntry();
        if (entry) entries.push(entry);
      }

      const files = await collectFilesFromEntries(entries);
      if (files.length) enqueueFiles(files);
      return;
    }

    // if we can't get entries, fall back to just accepting files
    const files = Array.from(e.dataTransfer.files).filter(
      (f) => f.type.startsWith("audio/") || isAudioFile(f.name)
    );
    if (files.length) enqueueFiles(files);
  }

  async function collectFilesFromEntries(
    entries: FileSystemEntry[]
  ): Promise<File[]> {
    const results = await Promise.all(
      entries.map(async (entry) => {
        if (entry.isFile) {
          return [await readFileEntry(entry as FileSystemFileEntry)];
        }
        if (entry.isDirectory) {
          return await readDirectoryEntries(entry as FileSystemDirectoryEntry);
        }
        return [];
      })
    );
    return results.flat();
  }

  function readFileEntry(entry: FileSystemFileEntry): Promise<File> {
    return new Promise((resolve) => {
      entry.file(resolve, () => resolve(null as unknown as File));
    });
  }

  async function readDirectoryEntries(
    entry: FileSystemDirectoryEntry
  ): Promise<File[]> {
    const files: File[] = [];
    const reader = entry.createReader();

    while (true) {
      const batch = await readEntriesBatch(reader);
      if (batch.length === 0) break;
      const subFiles = await collectFilesFromEntries(batch);
      files.push(...subFiles);
    }

    return files;
  }

  // Read a batch of entries from the directory
  // NOTE: readEntries() returns first 100 entries
  // https://developer.mozilla.org/en-US/docs/Web/API/FileSystemDirectoryReader/readEntries#browser_compatibility
  function readEntriesBatch(
    reader: FileSystemDirectoryReader
  ): Promise<FileSystemEntry[]> {
    return new Promise((resolve) => {
      reader.readEntries(resolve, () => resolve([]));
    });
  }

  // Add new files to the upload queue, avoiding duplicates. Start upload if not active.
  function enqueueFiles(files: File[]) {
    if (!uploadActive && uploadDone) {
      uploadQueue = [];
      uploadDone = false;
    }

    const existing = new Set(
      uploadQueue.map((i) => `${i.file.name}::${i.file.size}`)
    );
    const newItems: UploadItem[] = [];

    for (const f of files) {
      const key = `${f.name}::${f.size}`;
      if (existing.has(key)) continue;
      existing.add(key);
      if (!isAudioFile(f.name)) {
        newItems.push({
          file: f,
          status: "unsupported",
          error: "Not an audio file"
        });
      } else {
        newItems.push({ file: f, status: "pending" });
      }
    }

    uploadQueue = [...uploadQueue, ...newItems];
    uploadDone = false;
    if (!uploadActive) startUpload();
  }

  // Determine concurrency level based on number of file uploads
  // It's hardcoded to 1 for small uploads to avoid unnecessary
  // overhead, and goes up to 3 for large uploads to speed up the process.
  // TODO: may want to make this more dynamic in the future
  function getConcurrencyLevel(pendingCount: number): number {
    if (pendingCount <= 5) return 1;
    if (pendingCount <= 30) return 2;
    return 3;
  }

  async function startUpload() {
    uploadActive = true;
    uploadCancelled = false;

    const pendingCount = uploadQueue.filter(
      (i) => i.status === "pending"
    ).length;
    const concurrency = getConcurrencyLevel(pendingCount);

    const workers = Array.from({ length: concurrency }, () => uploadWorker());
    await Promise.all(workers);

    uploadActive = false;
    uploadDone = true;
    await loadTracks();
  }

  async function uploadWorker() {
    while (!uploadCancelled) {
      const idx = uploadQueue.findIndex((i) => i.status === "pending");
      if (idx < 0) break;

      uploadQueue[idx].status = "uploading";
      uploadQueue = uploadQueue; // trigger reactivity

      const item = uploadQueue[idx];
      try {
        const form = new FormData();
        form.append("file", item.file);
        const r = await apiFetch("/api/library/tracks/upload", {
          method: "POST",
          body: form,
          headers: {} // let browser set Content-Type
        });
        if (r.status === 409) {
          uploadQueue[idx].status = "duplicate";
          uploadQueue[idx].error = "Duplicate file";
        } else if (r.ok) {
          uploadQueue[idx].status = "done";
        } else {
          const body = await r.text().catch(() => "Upload failed");
          uploadQueue[idx].status = "error";
          uploadQueue[idx].error = body.slice(0, 120);
        }
      } catch (e: any) {
        uploadQueue[idx].status = "error";
        uploadQueue[idx].error = e.message ?? "Network error";
      }
      uploadQueue = uploadQueue; // trigger reactivity
    }
  }

  // Cancel ongoing uploads and mark pending items as cancelled.
  // Note that we can't actually abort in-flight fetch requests, so some uploads
  // may still complete after cancellation, but we'll ignore their results.
  function cancelUpload() {
    uploadCancelled = true;
    uploadQueue = uploadQueue.map((i) =>
      i.status === "pending"
        ? { ...i, status: "error" as const, error: "Cancelled" }
        : i
    );
  }

  function clearUploadQueue() {
    uploadQueue = [];
    uploadDone = false;
  }

  async function triggerScan() {
    await apiFetch("/api/library/scan", { method: "POST" });
  }

  // Auto-clear scan result after 8 seconds
  const timeoutMs = 8000;
  let scanResultTimer: ReturnType<typeof setTimeout> | undefined;
  $: if ($scanResult) {
    if (scanResultTimer) clearTimeout(scanResultTimer);
    scanResultTimer = setTimeout(() => scanResult.set(null), timeoutMs);
  }

  onDestroy(() => {
    if (scanResultTimer) clearTimeout(scanResultTimer);
  });

  function handleKeydown(e: KeyboardEvent) {
    // Enter -> save edit
    if (e.key === "Enter" && editingTrack) {
      e.preventDefault();
      saveEdit();
      return;
    }
    // Escape -> cancel edit
    if (e.key === "Escape" && editingTrack) {
      e.preventDefault();
      cancelEdit();
      return;
    }
    // "/" -> focus search (only when not in an input/textarea)
    if (e.key === "/" && !editingTrack) {
      const tag = (e.target as HTMLElement)?.tagName;
      if (tag !== "INPUT" && tag !== "TEXTAREA" && tag !== "SELECT") {
        e.preventDefault();
        const el = document.querySelector<HTMLInputElement>(
          ".admin-tracks-search"
        );
        el?.focus();
      }
    }
  }
</script>

<svelte:window on:keydown={handleKeydown} />

<div
  class="panel"
  class:drag-over={dragOver}
  on:dragover={handleDragOver}
  on:dragleave={handleDragLeave}
  on:drop={handleDrop}
  role="region"
>
  <div class="actions-bar">
    <input
      type="text"
      placeholder="Search tracks... (press /)"
      bind:value={searchQuery}
      class="search-input admin-tracks-search"
    />

    {#if canUpload}
      <input
        type="file"
        accept="audio/*"
        multiple
        bind:this={fileInput}
        on:change={handleFileInput}
        style="display:none"
      />
      <button
        class="action-btn"
        on:click={() => fileInput?.click()}
        disabled={uploadActive}
      >
        Upload Files
      </button>

      <input
        type="file"
        webkitdirectory
        multiple
        bind:this={folderInput}
        on:change={handleFolderInput}
        style="display:none"
      />
      <button
        class="action-btn"
        on:click={() => folderInput?.click()}
        disabled={uploadActive}
      >
        Upload Folder
      </button>
    {/if}

    {#if $currentUser?.is_admin}
      <button class="action-btn" on:click={triggerScan} disabled={$scanRunning}>
        {$scanRunning ? "Scanning..." : "Scan"}
      </button>
    {/if}

    {#if canDelete && selectedIds.size > 0}
      <button
        class="action-btn danger-btn"
        on:click={bulkDelete}
        disabled={bulkDeleting}
      >
        {bulkDeleting ? "Deleting…" : `Delete ${selectedIds.size}`}
      </button>
    {/if}
  </div>

  {#if $scanResult}
    <p class="scan-result">
      Scan complete: {$scanResult.added} added, {$scanResult.updated} updated, {$scanResult.removed}
      removed
    </p>
  {/if}

  {#if uploadQueue.length > 0}
    <div class="upload-panel">
      <div class="upload-header">
        <span class="upload-title">
          {#if uploadActive}
            Uploading {uploadStats.done +
              uploadStats.duplicate +
              uploadStats.error +
              uploadStats.unsupported}/{uploadStats.total}…
          {:else if uploadDone}
            Upload complete
          {:else}
            Upload queue
          {/if}
        </span>
        <div class="upload-actions">
          {#if uploadActive}
            <button class="sm-btn danger" on:click={cancelUpload}>Cancel</button
            >
          {/if}
          {#if !uploadActive}
            <button class="sm-btn" on:click={clearUploadQueue}>Clear</button>
          {/if}
        </div>
      </div>

      {#if uploadStats.total > 0}
        <div class="progress-bar-track">
          <div
            class="progress-bar-fill"
            style="width: {(
              ((uploadStats.done +
                uploadStats.duplicate +
                uploadStats.error +
                uploadStats.unsupported) /
                uploadStats.total) *
              100
            ).toFixed(1)}%"
          ></div>
        </div>
      {/if}

      <div class="upload-badges">
        {#if uploadStats.done > 0}
          <span class="badge success">{uploadStats.done} uploaded</span>
        {/if}
        {#if uploadStats.duplicate > 0}
          <span class="badge warn"
            >{uploadStats.duplicate} duplicate{uploadStats.duplicate > 1
              ? "s"
              : ""}</span
          >
        {/if}
        {#if uploadStats.unsupported > 0}
          <span class="badge warn">{uploadStats.unsupported} unsupported</span>
        {/if}
        {#if uploadStats.error > 0}
          <span class="badge err">{uploadStats.error} failed</span>
        {/if}
      </div>

      {#if uploadQueue.some((i) => i.status === "error" || i.status === "unsupported" || i.status === "duplicate")}
        <details class="upload-details">
          <summary class="text-3"
            >Show details ({uploadStats.duplicate +
              uploadStats.unsupported +
              uploadStats.error} items)</summary
          >
          <ul class="upload-detail-list">
            {#each uploadQueue.filter((i) => i.status === "error" || i.status === "unsupported" || i.status === "duplicate") as item}
              <li>
                <span
                  class="detail-status"
                  class:dup={item.status === "duplicate"}
                  class:err={item.status === "error"}
                  class:warn={item.status === "unsupported"}
                >
                  {item.status}
                </span>
                <span class="truncate">{item.file.name}</span>
                {#if item.error}
                  <span class="text-3">— {item.error}</span>
                {/if}
              </li>
            {/each}
          </ul>
        </details>
      {/if}
    </div>
  {/if}

  {#if dragOver && canUpload}
    <div class="drop-overlay">
      <div class="drop-message">
        <span class="drop-icon">↓</span>
        <p>Drop audio files or folders to upload</p>
      </div>
    </div>
  {/if}

  {#if loading}
    <p class="text-3">Loading…</p>
  {:else}
    <div class="table-wrap">
      <table>
        <thead>
          <tr>
            {#if canDelete}
              <th class="col-check">
                <input
                  type="checkbox"
                  checked={allSelected}
                  on:change={toggleSelectAll}
                  title="Select all"
                />
              </th>
            {/if}
            <th class="sortable" on:click={() => toggleSort("title")}
              >Title{sortIndicator("title")}</th
            >
            <th class="sortable" on:click={() => toggleSort("album_artist")}
              >Artist{sortIndicator("album_artist")}</th
            >
            <th class="sortable" on:click={() => toggleSort("album_name")}
              >Album{sortIndicator("album_name")}</th
            >
            <th
              class="sortable col-dur"
              on:click={() => toggleSort("duration_ms")}
              >Duration{sortIndicator("duration_ms")}</th
            >
            {#if canEdit || canDelete}
              <th>Actions</th>
            {/if}
          </tr>
        </thead>
        <tbody>
          {#each sortedTracks as t (t.id)}
            <tr class:selected={selectedIds.has(t.id)}>
              {#if canDelete}
                <td class="col-check">
                  <input
                    type="checkbox"
                    checked={selectedIds.has(t.id)}
                    on:change={() => toggleSelect(t.id)}
                  />
                </td>
              {/if}
              {#if editingTrack?.id === t.id}
                <td
                  ><input
                    type="text"
                    bind:value={editTitle}
                    class="inline-input"
                  /></td
                >
                <td
                  ><input
                    type="text"
                    bind:value={editArtist}
                    class="inline-input"
                  /></td
                >
                <td
                  ><input
                    type="text"
                    bind:value={editAlbum}
                    class="inline-input"
                  /></td
                >
                <td>{formatDuration(t.duration_ms)}</td>
                <td class="action-cell">
                  <button class="sm-btn save" on:click={saveEdit}>Save</button>
                  <button class="sm-btn" on:click={cancelEdit}>Cancel</button>
                </td>
              {:else}
                <td class="truncate">{t.title}</td>
                <td class="truncate text-2">{t.album_artist || "–"}</td>
                <td class="truncate text-2">{t.album_name || "–"}</td>
                <td class="text-3">{formatDuration(t.duration_ms)}</td>
                {#if canEdit || canDelete}
                  <td class="action-cell">
                    {#if canEdit}
                      <button class="sm-btn" on:click={() => startEdit(t)}
                        >Edit</button
                      >
                    {/if}
                    {#if canDelete}
                      <button
                        class="sm-btn danger"
                        on:click={() => deleteTrack(t.id)}>Delete</button
                      >
                    {/if}
                  </td>
                {/if}
              {/if}
            </tr>
          {/each}
          {#if sortedTracks.length === 0}
            <tr>
              <td
                colspan="99"
                class="text-3"
                style="text-align:center; padding: 24px;"
              >
                {searchQuery.trim()
                  ? "No tracks match your search"
                  : "No tracks in library"}
              </td>
            </tr>
          {/if}
        </tbody>
      </table>
    </div>

    {#if sortedTracks.length > 0}
      <p class="track-count text-3">
        {sortedTracks.length}{filteredTracks.length !== tracks.length
          ? ` of ${tracks.length}`
          : ""} track{sortedTracks.length !== 1 ? "s" : ""}
      </p>
    {/if}
  {/if}
</div>

<style>
  .panel {
    display: flex;
    flex-direction: column;
    gap: 12px;
    position: relative;
  }
  .panel.drag-over {
    outline: 2px dashed var(--accent);
    outline-offset: -4px;
    border-radius: 8px;
  }

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
  .action-btn:hover:not(:disabled) {
    background: var(--surface-hover);
  }
  .action-btn:disabled {
    opacity: 0.4;
    cursor: not-allowed;
  }
  .danger-btn {
    color: var(--danger);
    border-color: var(--danger);
  }

  .scan-result {
    font-size: 13px;
    color: var(--accent);
    margin: -4px 0 0;
  }

  .upload-panel {
    background: var(--surface);
    border: 1px solid var(--border);
    border-radius: var(--r-md);
    padding: 12px 16px;
    display: flex;
    flex-direction: column;
    gap: 8px;
  }
  .upload-header {
    display: flex;
    justify-content: space-between;
    align-items: center;
  }
  .upload-title {
    font-size: 13px;
    font-weight: 600;
  }
  .upload-actions {
    display: flex;
    gap: 6px;
  }

  .progress-bar-track {
    height: 4px;
    background: var(--surface-2);
    border-radius: 2px;
    overflow: hidden;
  }
  .progress-bar-fill {
    height: 100%;
    background: var(--accent);
    border-radius: 2px;
    transition: width 0.3s ease;
  }

  .upload-badges {
    display: flex;
    gap: 8px;
    flex-wrap: wrap;
  }
  .badge {
    font-size: 11px;
    padding: 2px 8px;
    border-radius: 999px;
    font-weight: 600;
  }
  .badge.success {
    background: rgba(74, 222, 128, 0.15);
    color: var(--accent);
  }
  .badge.warn {
    background: rgba(250, 204, 21, 0.12);
    color: #facc15;
  }
  .badge.err {
    background: rgba(248, 113, 113, 0.12);
    color: var(--danger);
  }

  .upload-details {
    font-size: 12px;
  }
  .upload-details summary {
    cursor: pointer;
    user-select: none;
  }
  .upload-detail-list {
    list-style: none;
    margin-top: 4px;
    max-height: 180px;
    overflow-y: auto;
    display: flex;
    flex-direction: column;
    gap: 2px;
  }
  .upload-detail-list li {
    display: flex;
    gap: 6px;
    align-items: baseline;
    font-size: 12px;
  }
  .detail-status {
    font-size: 10px;
    text-transform: uppercase;
    font-weight: 700;
    flex-shrink: 0;
  }
  .detail-status.dup {
    color: #facc15;
  }
  .detail-status.err {
    color: var(--danger);
  }
  .detail-status.warn {
    color: #facc15;
  }

  .drop-overlay {
    position: absolute;
    inset: 0;
    background: rgba(15, 15, 15, 0.85);
    display: flex;
    align-items: center;
    justify-content: center;
    z-index: 100;
    border-radius: 8px;
    pointer-events: none;
  }
  .drop-message {
    text-align: center;
    color: var(--accent);
    font-size: 18px;
    font-weight: 600;
  }
  .drop-icon {
    font-size: 48px;
    display: block;
    margin-bottom: 8px;
  }

  .table-wrap {
    overflow-x: auto;
  }

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
    white-space: nowrap;
    user-select: none;
  }
  th.sortable {
    cursor: pointer;
  }
  th.sortable:hover {
    color: var(--text-1);
  }

  .col-check {
    width: 32px;
    text-align: center;
  }
  .col-dur {
    width: 90px;
  }

  td {
    padding: 6px 12px;
    border-bottom: 1px solid var(--border);
    max-width: 200px;
  }

  tr:hover {
    background: var(--surface-hover);
  }
  tr.selected {
    background: rgba(74, 222, 128, 0.06);
  }

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
  .sm-btn:hover {
    background: var(--surface-hover);
  }
  .sm-btn.save {
    color: var(--accent);
    border-color: var(--accent-dim);
  }
  .sm-btn.danger {
    color: var(--danger);
  }
  .sm-btn.danger:hover {
    background: rgba(248, 113, 113, 0.1);
  }

  .inline-input {
    padding: 2px 6px;
    font-size: 13px;
    width: 100%;
  }

  input[type="checkbox"] {
    width: 15px;
    height: 15px;
    accent-color: var(--accent);
    cursor: pointer;
  }

  .track-count {
    font-size: 12px;
    text-align: right;
  }
</style>
