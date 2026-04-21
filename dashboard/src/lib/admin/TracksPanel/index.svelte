<script lang="ts">
  import { onMount, onDestroy } from "svelte";
  import { apiFetch, currentUser } from "../../api";
  import { libraryDelta, scanRunning, scanResult } from "../../ws";
  import { storageKeys, addToast } from "@pneuma/shared";
  import type { Track, SortKey, SortDir } from "./types";
  import type { UploadItem } from "./uploader";
  import {
    processUploadItem,
    isAudioFile,
    collectFilesFromEntries
  } from "./uploader";

  import Toolbar from "./Toolbar.svelte";
  import UploadStatBar from "./UploadStatBar.svelte";
  import TrackTable from "./TrackTable.svelte";

  let tracks: Track[] = $state([]);
  let loading = $state(false);
  let searchQuery = $state("");
  let sortKey: SortKey = $state("title");
  let sortDir: SortDir = $state("asc");
  let concurrencyLimit: number = $state(0);

  let selectedIds = $state(new Set<string>());
  let bulkDeleting = $state(false);

  let canUpload = $derived(
    !!($currentUser?.is_admin || $currentUser?.can_upload)
  );
  let canEdit = $derived(!!($currentUser?.is_admin || $currentUser?.can_edit));
  let canDelete = $derived(
    !!($currentUser?.is_admin || $currentUser?.can_delete)
  );

  const STORAGE_KEY = storageKeys.adminTracksPanel;

  function loadPersistedState() {
    try {
      const raw = localStorage.getItem(STORAGE_KEY);
      if (!raw) return;

      const s = JSON.parse(raw) as {
        sortKey?: SortKey;
        sortDir?: SortDir;
        searchQuery?: string;
        concurrencyLimit?: number;
      };
      if (!s) return;

      if (s.sortKey) sortKey = s.sortKey;
      if (s.sortDir) sortDir = s.sortDir;
      if (typeof s.searchQuery === "string") searchQuery = s.searchQuery;
      if (typeof s.concurrencyLimit === "number")
        concurrencyLimit = s.concurrencyLimit;
    } catch (e) {
      console.error("Failed to load persisted tracks panel state", e);
    }
  }

  function persistState() {
    try {
      localStorage.setItem(
        STORAGE_KEY,
        JSON.stringify({ sortKey, sortDir, searchQuery, concurrencyLimit })
      );
    } catch (e) {
      console.error("Failed to persist tracks panel state", e);
    }
  }

  // persist state to local storage
  $effect(() => {
    sortKey;
    sortDir;
    searchQuery;
    concurrencyLimit;
    persistState();
  });

  let filteredTracks = $derived(
    searchQuery.trim()
      ? tracks.filter(
          (t) =>
            t.title?.toLowerCase().includes(searchQuery.toLowerCase()) ||
            t.album_artist?.toLowerCase().includes(searchQuery.toLowerCase()) ||
            t.album_name?.toLowerCase().includes(searchQuery.toLowerCase())
        )
      : tracks
  );

  let sortedTracks = $derived(
    [...filteredTracks].sort((a, b) => {
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
    })
  );

  // load persisted state and tracks on mount
  onMount(() => {
    loadPersistedState();
    void loadTracks();
  });

  // normalize track payload from API response
  function normalizeTrackPayload(payload: unknown): Track[] {
    if (Array.isArray(payload)) return payload as Track[];

    if (payload && typeof payload === "object") {
      const obj = payload as { tracks?: Track[] };
      return Array.isArray(obj.tracks) ? obj.tracks : [];
    }

    return [];
  }

  async function loadTracks() {
    loading = true;
    try {
      const r = await apiFetch(`/api/library/tracks?view=admin-list`);
      if (!r.ok) return;
      const payload = await r.json();
      tracks = normalizeTrackPayload(payload);
      selectedIds = new Set(
        [...selectedIds].filter((id) => tracks.some((track) => track.id === id))
      );
    } finally {
      loading = false;
    }
  }

  async function fetchTracksByIDs(ids: string[]): Promise<Track[]> {
    if (ids.length === 0) return [];
    const r = await apiFetch(
      `/api/library/tracks?view=admin-list&ids=${encodeURIComponent(ids.join(","))}`
    );
    if (!r.ok) return [];
    const payload = await r.json();
    return Array.isArray(payload) ? (payload as Track[]) : [];
  }

  // "library delta" means that the library has transformed in some way
  // (e.g. a track was added, removed, or updated)
  async function applyLibraryDelta(type: string, id: string | null) {
    if (uploadActive || $scanRunning) return;
    if (type === "library.deduped") {
      await loadTracks();
      return;
    }
    if (!id) return;

    if (type === "track.removed") {
      tracks = tracks.filter((track) => track.id !== id);
      selectedIds = new Set(
        [...selectedIds].filter((selected) => selected !== id)
      );
      return;
    }

    const updated = await fetchTracksByIDs([id]);
    const next = updated[0];
    if (!next) return;

    const index = tracks.findIndex((track) => track.id === id);
    if (index >= 0)
      tracks = [...tracks.slice(0, index), next, ...tracks.slice(index + 1)];
    else tracks = [...tracks, next];
  }

  let _prevDeltaSeq: number | undefined = $state();
  $effect(() => {
    const delta = $libraryDelta;
    if (!delta) return;
    if (_prevDeltaSeq === delta.seq) return;
    _prevDeltaSeq = delta.seq;
    void applyLibraryDelta(delta.type, delta.id);
  });

  $effect(() => {
    if ($scanResult) void loadTracks();
  });

  async function triggerScan() {
    await apiFetch("/api/library/scan", { method: "POST" });
  }

  const timeoutMs = 8000;
  let scanResultTimer: ReturnType<typeof setTimeout> | undefined;
  $effect(() => {
    if ($scanResult) {
      if (scanResultTimer) clearTimeout(scanResultTimer);
      scanResultTimer = setTimeout(() => scanResult.set(null), timeoutMs);
    }
  });

  onDestroy(() => {
    if (scanResultTimer) clearTimeout(scanResultTimer);
  });

  let editingTrackId: string | null = $state(null);

  async function saveEdit(
    id: string,
    title: string,
    artist: string,
    album: string
  ) {
    const r = await apiFetch(`/api/library/tracks/${id}`, {
      method: "PATCH",
      body: JSON.stringify({ title, album_artist: artist, album_name: album })
    });
    if (r.ok) {
      addToast("Track updated successfully", "success");
      const updated = await fetchTracksByIDs([id]);
      if (updated.length === 1) {
        const idx = tracks.findIndex((t) => t.id === id);
        if (idx >= 0) {
          tracks = [
            ...tracks.slice(0, idx),
            updated[0],
            ...tracks.slice(idx + 1)
          ];
        }
      }
      editingTrackId = null;
    }
  }

  async function deleteTrack(id: string) {
    if (!confirm("Delete this track?")) return;
    const r = await apiFetch(`/api/library/tracks/${id}`, { method: "DELETE" });
    if (!r.ok) return;
    tracks = tracks.filter((t) => t.id !== id);
    selectedIds = new Set(
      [...selectedIds].filter((selected) => selected !== id)
    );
  }

  let allSelected = $derived(
    sortedTracks.length > 0 && sortedTracks.every((t) => selectedIds.has(t.id))
  );

  function toggleSelectAll() {
    if (allSelected) selectedIds = new Set();
    else selectedIds = new Set(sortedTracks.map((t) => t.id));
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
    const deletedIDs = new Set(ids);
    selectedIds = new Set();
    tracks = tracks.filter((track) => !deletedIDs.has(track.id));
    await loadTracks();
    if (fail > 0) alert(`Deleted ${ok}, failed ${fail}`);
  }

  let uploadQueue: UploadItem[] = $state([]);
  let uploadActive = $state(false);
  let uploadCancelled = $state(false);
  let uploadDone = $state(false);

  let replacingTrack: Track | null = $state(null);
  let toolbarRef: ReturnType<typeof Toolbar> | undefined;

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
    const concurrency =
      concurrencyLimit > 0
        ? concurrencyLimit
        : getConcurrencyLevel(pendingCount);

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
      uploadQueue = uploadQueue;

      const updatedItem = await processUploadItem(uploadQueue[idx]);
      uploadQueue[idx] = updatedItem;
      uploadQueue = uploadQueue;
    }
  }

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

  let dragOver = $state(false);

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

    const files = Array.from(e.dataTransfer.files).filter(
      (f) => f.type.startsWith("audio/") || isAudioFile(f.name)
    );
    if (files.length) enqueueFiles(files);
  }

  async function handleReplaceTrack(file: File, track: Track) {
    const form = new FormData();
    form.append("file", file);
    try {
      const r = await apiFetch(`/api/library/tracks/${track.id}/file`, {
        method: "PUT",
        body: form
      });
      if (!r.ok) {
        const text = await r.text().catch(() => "Unknown error");
        addToast("Failed to replace file: " + text, "error");
      } else {
        addToast("File replacement queued successfully", "success");
      }
    } catch (e: any) {
      addToast("Upload error: " + (e.message || "Network error"), "error");
    }
  }

  function handleKeydown(e: KeyboardEvent) {
    if (e.key === "/" && !editingTrackId) {
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

<svelte:window onkeydown={handleKeydown} />

<div
  class="panel"
  class:drag-over={dragOver}
  ondragover={handleDragOver}
  ondragleave={handleDragLeave}
  ondrop={handleDrop}
  role="region"
>
  <Toolbar
    bind:this={toolbarRef}
    bind:searchQuery
    bind:concurrencyLimit
    {canUpload}
    {canDelete}
    selectedCount={selectedIds.size}
    {bulkDeleting}
    {uploadActive}
    bind:replacingTrack
    onUploadFiles={enqueueFiles}
    onReplaceTrack={handleReplaceTrack}
    onTriggerScan={triggerScan}
    onBulkDelete={bulkDelete}
  />

  {#if $scanResult}
    <p class="scan-result">
      Scan complete: {$scanResult.added} added, {$scanResult.updated} updated, {$scanResult.removed}
      removed
    </p>
  {/if}

  {#if uploadQueue.length > 0}
    <UploadStatBar
      {uploadQueue}
      {uploadActive}
      {uploadDone}
      onCancelUpload={cancelUpload}
      onClearUploadQueue={clearUploadQueue}
    />
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
    <TrackTable
      {sortedTracks}
      {searchQuery}
      totalTracksCount={tracks.length}
      filteredTracksCount={filteredTracks.length}
      bind:sortKey
      bind:sortDir
      {canEdit}
      {canDelete}
      {canUpload}
      {allSelected}
      {selectedIds}
      {editingTrackId}
      onToggleSelectAll={toggleSelectAll}
      onToggleSelect={toggleSelect}
      onStartEdit={(t) => (editingTrackId = t.id)}
      onSaveEdit={saveEdit}
      onCancelEdit={() => (editingTrackId = null)}
      onDeleteTrack={deleteTrack}
      onClickReplace={(t) => {
        replacingTrack = t;
        (toolbarRef as any)?.clickReplaceInput();
      }}
    />
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

  .scan-result {
    font-size: 13px;
    color: var(--accent);
    margin: -4px 0 0;
  }

  .drop-overlay {
    position: absolute;
    inset: 0;
    background: var(--drop-overlay-bg);
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
</style>
