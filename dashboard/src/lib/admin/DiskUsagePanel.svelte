<script lang="ts">
  import { onMount } from "svelte";
  import { apiFetch } from "../api";
  import { addToast } from "@pneuma/shared";
  import { formatBytes, formatDate } from "../utils";

  interface DiskUsageData {
    total_bytes: number;
    free_bytes: number;
    tracks_bytes: number;
    db_bytes: number;
    transcode_cache_bytes: number;
    artwork_cache_bytes: number;
    playlist_art_bytes: number;
    recorded_at: string;
  }

  let data: DiskUsageData | null = $state(null);
  let loading = $state(false);
  let clearing = $state(false);

  onMount(loadDiskUsage);

  async function loadDiskUsage() {
    loading = true;
    try {
      const r = await apiFetch("/api/admin/metrics/disk");
      if (r.ok) data = await r.json();
    } catch {
      console.warn("Failed to load disk usage");
    } finally {
      loading = false;
    }
  }

  async function clearCache() {
    if (
      !confirm(
        "Clear transcode and artwork caches? Cached files will be regenerated on demand."
      )
    )
      return;
    clearing = true;
    try {
      const r = await apiFetch("/api/admin/cache", { method: "DELETE" });
      if (r.ok) {
        data = await r.json();
        addToast("Caches cleared", "success");
      } else {
        addToast("Failed to clear caches: " + (await r.text()), "error");
      }
    } catch {
      addToast("Failed to clear caches", "error");
    } finally {
      clearing = false;
    }
  }

  // returns the percentage of total that each part represents
  function pct(part: number, total: number): number {
    return total > 0 ? (part / total) * 100 : 0;
  }

  type Segment = { label: string; bytes: number; color: string };

  let segments: Segment[] = $derived.by(() => {
    if (!data) return [];
    return [
      { label: "Tracks", bytes: data.tracks_bytes, color: "var(--accent)" },
      { label: "Database", bytes: data.db_bytes, color: "#7c5cfc" },
      {
        label: "Transcode Cache",
        bytes: data.transcode_cache_bytes,
        color: "#f59e0b"
      },
      {
        label: "Artwork Cache",
        bytes: data.artwork_cache_bytes,
        color: "#10b981"
      },
      {
        label: "Playlist Art",
        bytes: data.playlist_art_bytes,
        color: "#ec4899"
      }
    ];
  });

  let otherUsed = $derived.by(() => {
    if (!data) return 0;
    const pneumaTotal =
      data.tracks_bytes +
      data.db_bytes +
      data.transcode_cache_bytes +
      data.artwork_cache_bytes +
      data.playlist_art_bytes;
    const totalUsed = data.total_bytes - data.free_bytes;
    return Math.max(0, totalUsed - pneumaTotal);
  });

  let pneumaTotal = $derived.by(() => {
    if (!data) return 0;
    return (
      data.tracks_bytes +
      data.db_bytes +
      data.transcode_cache_bytes +
      data.artwork_cache_bytes +
      data.playlist_art_bytes
    );
  });

  let cacheTotal = $derived.by(() => {
    if (!data) return 0;
    return (
      data.transcode_cache_bytes +
      data.artwork_cache_bytes +
      data.playlist_art_bytes
    );
  });
</script>

<div class="panel">
  <div class="header">
    <div style="display: flex; align-items: baseline; gap: 12px;">
      <h2>Disk Usage</h2>
      {#if data}
        <span class="text-3" style="font-size: 12px;"
          >Last updated: {formatDate(data.recorded_at)}</span
        >
      {/if}
    </div>
    <button
      class="danger-btn"
      onclick={clearCache}
      disabled={clearing || cacheTotal === 0}
    >
      {clearing ? "Clearing..." : "Clear Cache"}
    </button>
  </div>

  {#if loading}
    <p class="text-3">Loading...</p>
  {:else if !data}
    <p class="text-3">No disk usage data available.</p>
  {:else}
    <div class="bar-container">
      <div class="bar-track">
        {#each segments as seg}
          {#if seg.bytes > 0}
            <div
              class="bar-segment"
              style="width: {pct(
                seg.bytes,
                data.total_bytes
              )}%; background: {seg.color};"
              title="{seg.label}: {formatBytes(seg.bytes)}"
            ></div>
          {/if}
        {/each}
        {#if otherUsed > 0}
          <div
            class="bar-segment"
            style="width: {pct(
              otherUsed,
              data.total_bytes
            )}%; background: var(--text-3);"
            title="Other: {formatBytes(otherUsed)}"
          ></div>
        {/if}
      </div>
    </div>

    <div class="legend">
      {#each segments as seg}
        <div class="legend-item">
          <span class="legend-dot" style="background: {seg.color};"></span>
          <span class="legend-label">{seg.label}</span>
          <span class="legend-value">{formatBytes(seg.bytes)}</span>
        </div>
      {/each}
      <div class="legend-item">
        <span class="legend-dot" style="background: var(--text-3);"></span>
        <span class="legend-label">Other (OS)</span>
        <span class="legend-value">{formatBytes(otherUsed)}</span>
      </div>
      <div class="legend-item">
        <span
          class="legend-dot"
          style="background: transparent; border: 1px solid var(--border);"
        ></span>
        <span class="legend-label">Free</span>
        <span class="legend-value">{formatBytes(data.free_bytes)}</span>
      </div>
    </div>

    <div class="summary">
      <div class="stat-card">
        <span class="stat-label">Total Used</span>
        <span class="stat-value">{formatBytes(pneumaTotal)}</span>
      </div>
      <div class="stat-card">
        <span class="stat-label">Disk Total</span>
        <span class="stat-value">{formatBytes(data.total_bytes)}</span>
      </div>
      <div class="stat-card">
        <span class="stat-label">Free Space</span>
        <span class="stat-value">{formatBytes(data.free_bytes)}</span>
      </div>
      <div class="stat-card">
        <span class="stat-label">Cache</span>
        <span class="stat-value">{formatBytes(cacheTotal)}</span>
      </div>
    </div>
  {/if}
</div>

<style>
  .panel {
    display: flex;
    flex-direction: column;
    gap: 16px;
  }

  .header {
    display: flex;
    justify-content: space-between;
    align-items: center;
  }

  .header h2 {
    margin: 0;
    font-size: 16px;
  }

  .danger-btn {
    padding: 6px 14px;
    border-radius: var(--r-md);
    background: var(--surface-2);
    border: 1px solid var(--danger);
    color: var(--danger);
    font-size: 13px;
    cursor: pointer;
  }
  .danger-btn:hover:not(:disabled) {
    background: var(--danger-soft);
  }
  .danger-btn:disabled {
    opacity: 0.4;
    cursor: not-allowed;
  }

  .bar-container {
    padding: 4px 0;
  }

  .bar-track {
    display: flex;
    height: 28px;
    border-radius: var(--r-md);
    overflow: hidden;
    background: var(--surface-2);
  }

  .bar-segment {
    height: 100%;
    min-width: 2px;
    transition: width 0.4s ease;
  }

  .legend {
    display: flex;
    flex-wrap: wrap;
    gap: 12px 24px;
  }

  .legend-item {
    display: flex;
    align-items: center;
    gap: 6px;
    font-size: 13px;
  }

  .legend-dot {
    width: 10px;
    height: 10px;
    border-radius: 50%;
    flex-shrink: 0;
  }

  .legend-label {
    color: var(--text-2);
  }

  .legend-value {
    color: var(--text-1);
    font-weight: 600;
    font-variant-numeric: tabular-nums;
  }

  .summary {
    display: grid;
    grid-template-columns: repeat(auto-fit, minmax(140px, 1fr));
    gap: 12px;
  }

  .stat-card {
    display: flex;
    flex-direction: column;
    gap: 4px;
    padding: 14px 16px;
    background: var(--surface-2);
    border: 1px solid var(--border);
    border-radius: var(--r-md);
  }

  .stat-label {
    font-size: 11px;
    text-transform: uppercase;
    letter-spacing: 0.05em;
    color: var(--text-3);
  }

  .stat-value {
    font-size: 18px;
    font-weight: 700;
    color: var(--text-1);
    font-variant-numeric: tabular-nums;
  }
</style>
