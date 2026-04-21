<script lang="ts">
  import type { UploadItem } from "./uploader";

  interface Props {
    uploadQueue: UploadItem[];
    uploadActive: boolean;
    uploadDone: boolean;
    onCancelUpload: () => void;
    onClearUploadQueue: () => void;
  }

  let {
    uploadQueue,
    uploadActive,
    uploadDone,
    onCancelUpload,
    onClearUploadQueue
  }: Props = $props();

  let uploadStats = $derived({
    total: uploadQueue.length,
    done: uploadQueue.filter((i) => i.status === "done").length,
    duplicate: uploadQueue.filter((i) => i.status === "duplicate").length,
    unsupported: uploadQueue.filter((i) => i.status === "unsupported").length,
    error: uploadQueue.filter((i) => i.status === "error").length,
    pending: uploadQueue.filter(
      (i) => i.status === "pending" || i.status === "uploading"
    ).length
  });
</script>

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
        <button class="sm-btn danger" onclick={onCancelUpload}>Cancel</button>
      {/if}
      {#if !uploadActive}
        <button class="sm-btn" onclick={onClearUploadQueue}>Clear</button>
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

<style>
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
    background: var(--success-soft);
    color: var(--success);
  }
  .badge.warn {
    background: var(--warning-soft);
    color: var(--warning);
  }
  .badge.err {
    background: var(--danger-soft);
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
    color: var(--warning);
  }
  .detail-status.err {
    color: var(--danger);
  }
  .detail-status.warn {
    color: var(--warning);
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
  .sm-btn.danger {
    color: var(--danger);
  }
  .sm-btn.danger:hover {
    background: var(--danger-soft);
  }
</style>
