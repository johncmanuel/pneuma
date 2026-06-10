<script lang="ts">
  import { onMount } from "svelte";
  import { apiFetch } from "../api";
  import { addToast } from "@pneuma/shared";
  import { formatBytes } from "../utils";

  interface TelemetryStat {
    track_id: string;
    track_title: string;
    track_artist: string;
    track_codec: string;
    track_file_size: number;
    track_duration_ms: number;
    stream_quality: string;
    sample_count: number;
    avg_latency_ms: number;
    min_latency_ms: number;
    max_latency_ms: number;
    total_stutters: number;
    avg_stutter_duration_ms: number;
  }

  let stats: TelemetryStat[] = $state([]);
  let loading = $state(false);
  let clearing = $state(false);
  let sortKey: keyof TelemetryStat = $state("avg_latency_ms");
  let sortAsc = $state(false);

  let sorted = $derived.by(() => {
    const copy = [...stats];
    copy.sort((a, b) => {
      const aVal = a[sortKey];
      const bVal = b[sortKey];
      if (typeof aVal === "number" && typeof bVal === "number") {
        return sortAsc ? aVal - bVal : bVal - aVal;
      }
      const aStr = String(aVal);
      const bStr = String(bVal);
      return sortAsc ? aStr.localeCompare(bStr) : bStr.localeCompare(aStr);
    });
    return copy;
  });

  onMount(loadStats);

  async function loadStats() {
    loading = true;
    try {
      const r = await apiFetch("/api/admin/telemetry/stream");
      if (r.ok) stats = await r.json();
    } catch {
      console.warn("Failed to load telemetry stats");
    } finally {
      loading = false;
    }
  }

  async function clearData() {
    if (!confirm("Clear all stream telemetry data? This cannot be undone."))
      return;
    clearing = true;
    try {
      const r = await apiFetch("/api/admin/telemetry/stream", {
        method: "DELETE"
      });
      if (r.ok) {
        stats = [];
        addToast("Telemetry data cleared", "success");
      } else {
        addToast("Failed to clear telemetry", "error");
      }
    } catch {
      addToast("Failed to clear telemetry", "error");
    } finally {
      clearing = false;
    }
  }

  function toggleSort(key: keyof TelemetryStat) {
    if (sortKey === key) {
      sortAsc = !sortAsc;
    } else {
      sortKey = key;
      sortAsc = false;
    }
  }

  function sortIndicator(key: keyof TelemetryStat): string {
    if (sortKey !== key) return "";
    return sortAsc ? " ↑" : " ↓";
  }

  function formatMs(ms: number): string {
    if (ms < 1000) return `${ms}ms`;
    return `${(ms / 1000).toFixed(2)}s`;
  }

  function latencyClass(ms: number): string {
    if (ms <= 300) return "good";
    if (ms <= 1000) return "ok";
    return "bad";
  }
</script>

<div class="panel">
  <div class="header">
    <h2>Stream Telemetry</h2>
    <div class="actions">
      <button class="refresh-btn" onclick={loadStats} disabled={loading}>
        {loading ? "Loading..." : "Refresh"}
      </button>
      <button
        class="danger-btn"
        onclick={clearData}
        disabled={clearing || stats.length === 0}
      >
        {clearing ? "Clearing..." : "Clear Data"}
      </button>
    </div>
  </div>

  {#if loading && stats.length === 0}
    <p class="text-3">Loading telemetry data...</p>
  {:else if stats.length === 0}
    <div class="empty-state">
      <p>No telemetry data yet.</p>
      <p class="text-3">Play tracks in the web player to collect metrics.</p>
    </div>
  {:else}
    <div class="summary">
      <div class="stat-card">
        <span class="stat-label">Tracks Measured</span>
        <span class="stat-value"
          >{new Set(stats.map((s) => s.track_id)).size}</span
        >
      </div>
      <div class="stat-card">
        <span class="stat-label">Total Samples</span>
        <span class="stat-value"
          >{stats.reduce((n, s) => n + s.sample_count, 0)}</span
        >
      </div>
      <div class="stat-card">
        <span class="stat-label">Avg Latency</span>
        <span class="stat-value">
          {formatMs(
            Math.round(
              stats.reduce((n, s) => n + s.avg_latency_ms, 0) / stats.length
            )
          )}
        </span>
      </div>
      <div class="stat-card">
        <span class="stat-label">Total Stutters</span>
        <span class="stat-value"
          >{stats.reduce((n, s) => n + s.total_stutters, 0)}</span
        >
      </div>
    </div>

    <div class="table-wrap">
      <table>
        <thead>
          <tr>
            <th
              class="sortable"
              onclick={() => toggleSort("track_title")}
              aria-sort={sortKey === "track_title"
                ? sortAsc
                  ? "ascending"
                  : "descending"
                : "none"}
            >
              Track{sortIndicator("track_title")}
            </th>
            <th
              class="sortable"
              onclick={() => toggleSort("track_codec")}
              aria-sort={sortKey === "track_codec"
                ? sortAsc
                  ? "ascending"
                  : "descending"
                : "none"}
            >
              Codec{sortIndicator("track_codec")}
            </th>
            <th
              class="sortable"
              onclick={() => toggleSort("track_file_size")}
              aria-sort={sortKey === "track_file_size"
                ? sortAsc
                  ? "ascending"
                  : "descending"
                : "none"}
            >
              Size{sortIndicator("track_file_size")}
            </th>
            <th
              class="sortable"
              onclick={() => toggleSort("stream_quality")}
              aria-sort={sortKey === "stream_quality"
                ? sortAsc
                  ? "ascending"
                  : "descending"
                : "none"}
            >
              Quality{sortIndicator("stream_quality")}
            </th>
            <th
              class="sortable num"
              onclick={() => toggleSort("sample_count")}
              aria-sort={sortKey === "sample_count"
                ? sortAsc
                  ? "ascending"
                  : "descending"
                : "none"}
            >
              Samples{sortIndicator("sample_count")}
            </th>
            <th
              class="sortable num"
              onclick={() => toggleSort("avg_latency_ms")}
              aria-sort={sortKey === "avg_latency_ms"
                ? sortAsc
                  ? "ascending"
                  : "descending"
                : "none"}
            >
              Avg Latency{sortIndicator("avg_latency_ms")}
            </th>
            <th class="num">Min</th>
            <th class="num">Max</th>
            <th
              class="sortable num"
              onclick={() => toggleSort("total_stutters")}
              aria-sort={sortKey === "total_stutters"
                ? sortAsc
                  ? "ascending"
                  : "descending"
                : "none"}
            >
              Stutters{sortIndicator("total_stutters")}
            </th>
            <th
              class="sortable num"
              onclick={() => toggleSort("avg_stutter_duration_ms")}
              aria-sort={sortKey === "avg_stutter_duration_ms"
                ? sortAsc
                  ? "ascending"
                  : "descending"
                : "none"}
            >
              Avg Stutter{sortIndicator("avg_stutter_duration_ms")}
            </th>
          </tr>
        </thead>
        <tbody>
          {#each sorted as row (row.track_id + row.stream_quality)}
            <tr>
              <td class="track-cell">
                <span class="track-title"
                  >{row.track_title || "Unknown Track"}</span
                >
                <span class="track-artist"
                  >{row.track_artist || "Unknown Artist"}</span
                >
              </td>
              <td>
                <span class="badge codec">{row.track_codec || "—"}</span>
              </td>
              <td>
                {row.track_file_size > 0
                  ? formatBytes(row.track_file_size)
                  : "—"}
              </td>
              <td>
                <span class="badge quality">{row.stream_quality}</span>
              </td>
              <td class="num">{row.sample_count}</td>
              <td class="num">
                <span class="latency {latencyClass(row.avg_latency_ms)}">
                  {formatMs(row.avg_latency_ms)}
                </span>
              </td>
              <td class="num text-3">{formatMs(row.min_latency_ms)}</td>
              <td class="num text-3">{formatMs(row.max_latency_ms)}</td>
              <td class="num">
                {#if row.total_stutters > 0}
                  <span class="stutter-count">{row.total_stutters}</span>
                {:else}
                  <span class="text-3">0</span>
                {/if}
              </td>
              <td class="num text-3">
                {row.avg_stutter_duration_ms > 0
                  ? formatMs(row.avg_stutter_duration_ms)
                  : "—"}
              </td>
            </tr>
          {/each}
        </tbody>
      </table>
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

  .actions {
    display: flex;
    gap: 8px;
  }

  .refresh-btn {
    padding: 6px 14px;
    border-radius: var(--r-md);
    background: var(--surface-2);
    border: 1px solid var(--border);
    color: var(--text-1);
    font-size: 13px;
    cursor: pointer;
  }
  .refresh-btn:hover:not(:disabled) {
    background: var(--surface-3);
  }
  .refresh-btn:disabled {
    opacity: 0.4;
    cursor: not-allowed;
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

  .empty-state {
    display: flex;
    flex-direction: column;
    align-items: center;
    gap: 4px;
    padding: 48px 16px;
    color: var(--text-2);
    font-size: 14px;
  }
  .empty-state p {
    margin: 0;
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

  .table-wrap {
    overflow-x: auto;
  }

  table {
    width: 100%;
    border-collapse: collapse;
    font-size: 13px;
  }

  th,
  td {
    padding: 10px 12px;
    text-align: left;
    border-bottom: 1px solid var(--border);
    white-space: nowrap;
  }

  th {
    font-size: 11px;
    text-transform: uppercase;
    letter-spacing: 0.04em;
    color: var(--text-3);
    font-weight: 600;
    position: sticky;
    top: 0;
    background: var(--surface-1);
    z-index: 1;
  }

  th.sortable {
    cursor: pointer;
    user-select: none;
  }
  th.sortable:hover {
    color: var(--text-1);
  }

  th.num,
  td.num {
    text-align: right;
  }

  .track-cell {
    display: flex;
    flex-direction: column;
    gap: 2px;
  }

  .track-title {
    font-weight: 600;
    color: var(--text-1);
  }

  .track-artist {
    font-size: 12px;
    color: var(--text-3);
  }

  .badge {
    display: inline-block;
    padding: 2px 8px;
    border-radius: 9999px;
    font-size: 11px;
    font-weight: 600;
    text-transform: uppercase;
    letter-spacing: 0.03em;
  }

  .badge.codec {
    background: var(--surface-3);
    color: var(--text-2);
  }

  .badge.quality {
    background: var(--accent-soft, rgba(99, 102, 241, 0.15));
    color: var(--accent);
  }

  .latency {
    font-weight: 600;
    font-variant-numeric: tabular-nums;
  }

  .latency.good {
    color: #10b981;
  }
  .latency.ok {
    color: #f59e0b;
  }
  .latency.bad {
    color: #ef4444;
  }

  .stutter-count {
    color: #ef4444;
    font-weight: 600;
  }

  tr:hover td {
    background: var(--surface-2);
  }
</style>
