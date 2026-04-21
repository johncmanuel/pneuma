<script lang="ts">
  import { currentUser } from "../../api";
  import { scanRunning } from "../../ws";
  import { AUDIO_ACCEPT } from "./uploader";
  import type { Track } from "./types";

  interface Props {
    searchQuery: string;
    concurrencyLimit: number;
    canUpload: boolean;
    canDelete: boolean;
    selectedCount: number;
    bulkDeleting: boolean;
    uploadActive: boolean;
    onUploadFiles: (files: File[]) => void;
    onReplaceTrack: (file: File, track: Track) => void;
    onTriggerScan: () => void;
    onBulkDelete: () => void;
    replacingTrack: Track | null;
  }

  let {
    searchQuery = $bindable(),
    concurrencyLimit = $bindable(),
    canUpload,
    canDelete,
    selectedCount,
    bulkDeleting,
    uploadActive,
    onUploadFiles,
    onReplaceTrack,
    onTriggerScan,
    onBulkDelete,
    replacingTrack = $bindable()
  }: Props = $props();

  let fileInput: HTMLInputElement | undefined = $state();
  let folderInput: HTMLInputElement | undefined = $state();
  let replaceInput: HTMLInputElement | undefined = $state();

  function handleFileInput() {
    if (fileInput?.files?.length) {
      onUploadFiles(Array.from(fileInput.files));
      fileInput.value = "";
    }
  }

  function handleFolderInput() {
    if (folderInput?.files?.length) {
      onUploadFiles(Array.from(folderInput.files));
      folderInput.value = "";
    }
  }

  function handleReplaceFileInput() {
    const file = replaceInput?.files?.[0];
    const track = replacingTrack;
    if (file && track) {
      onReplaceTrack(file, track);
    }
    if (replaceInput) replaceInput.value = "";
    replacingTrack = null;
  }

  export function clickReplaceInput() {
    replaceInput?.click();
  }
</script>

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
      accept={AUDIO_ACCEPT}
      bind:this={replaceInput}
      onchange={handleReplaceFileInput}
      style="display:none"
    />
    <input
      type="file"
      accept={AUDIO_ACCEPT}
      multiple
      bind:this={fileInput}
      onchange={handleFileInput}
      style="display:none"
    />
    <button
      class="action-btn"
      onclick={() => fileInput?.click()}
      disabled={uploadActive}
    >
      Upload Files
    </button>

    <input
      type="file"
      webkitdirectory
      multiple
      bind:this={folderInput}
      onchange={handleFolderInput}
      style="display:none"
    />
    <button
      class="action-btn"
      onclick={() => folderInput?.click()}
      disabled={uploadActive}
    >
      Upload Folder
    </button>

    <select
      bind:value={concurrencyLimit}
      class="search-input"
      style="width: auto"
      title="Upload Concurrency"
    >
      <option value={0}>Auto Concurrency</option>
      <option value={1}>1 at a time</option>
      <option value={2}>2 at a time</option>
      <option value={3}>3 at a time</option>
      <option value={4}>4 at a time</option>
      <option value={5}>5 at a time</option>
      <option value={10}>10 at a time</option>
    </select>
  {/if}

  {#if $currentUser?.is_admin}
    <button class="action-btn" onclick={onTriggerScan} disabled={$scanRunning}>
      {$scanRunning ? "Scanning..." : "Scan"}
    </button>
  {/if}

  {#if canDelete && selectedCount > 0}
    <button
      class="action-btn danger-btn"
      onclick={onBulkDelete}
      disabled={bulkDeleting}
    >
      {bulkDeleting ? "Deleting…" : `Delete ${selectedCount}`}
    </button>
  {/if}
</div>

<style>
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
    cursor: pointer;
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
</style>
