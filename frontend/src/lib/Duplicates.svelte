<script lang="ts">
  import {
    localDuplicates,
    dismissDuplicate,
    restoreDuplicate,
    dismissedDuplicates,
    scanningDuplicates,
  } from "../stores/localLibrary"
  import { formatDuration } from "./TrackRow.svelte"

  function fmtPath(p: string): string {
    const parts = p.split("/")
    return parts.length > 2 ? "…/" + parts.slice(-2).join("/") : p
  }

  $: groups = $localDuplicates
  $: dismissed = $dismissedDuplicates
  $: scanning = $scanningDuplicates

  /** Collapsed state per group index. */
  let collapsed: Record<number, boolean> = {}
  function toggle(idx: number) {
    collapsed[idx] = !collapsed[idx]
  }
</script>

<section>
  <h2>Duplicates</h2>

  {#if scanning}
    <div class="scan-banner">
      <span class="spinner" aria-hidden="true"></span>
      <span>Scanning for duplicates — comparing content &amp; acoustic fingerprints…</span>
    </div>
  {/if}

  {#if groups.length === 0}
    <p class="text-3 empty">
      {scanning ? "Results will appear when the scan completes." : "No duplicate local files detected."}
    </p>
  {:else}
    <p class="subtitle text-2">
      {groups.length} duplicate group{groups.length !== 1 ? "s" : ""} found.
      Dismiss copies you don't want to see in your library.
    </p>

    <div class="groups">
      {#each groups as group, idx (group.fingerprint)}
        {@const primary = group.tracks[0]}
        <div class="group">
          <button class="group-header" on:click={() => toggle(idx)}>
            <span class="chevron" class:open={!collapsed[idx]}>▸</span>
            <span class="group-title truncate">
              {primary.title}
              <span class="text-2">— {primary.artist || primary.album_artist || "Unknown"}</span>
            </span>
            <span class="badge" class:exact={group.kind === "exact"} class:acoustic={group.kind === "acoustic"}>
              {group.kind === "exact" ? "Exact copy" : "Acoustic match"}
            </span>
            <span class="count text-3">{group.tracks.length} copies</span>
          </button>

          {#if !collapsed[idx]}
            <ul class="track-list">
              {#each group.tracks as track (track.path)}
                <li class="track-row" class:is-dismissed={dismissed.has(track.path)}>
                  <span class="track-path truncate" title={track.path}>{track.path}</span>
                  <span class="track-dur text-3">{formatDuration(track.duration_ms)}</span>
                  {#if dismissed.has(track.path)}
                    <button class="action restore" on:click={() => restoreDuplicate(track.path)} title="Restore">
                      ↩
                    </button>
                  {:else}
                    <button class="action dismiss" on:click={() => dismissDuplicate(track.path)} title="Dismiss from library">
                      ✕
                    </button>
                  {/if}
                </li>
              {/each}
            </ul>
          {/if}
        </div>
      {/each}
    </div>
  {/if}
</section>

<style>
  section {
    height: 100%;
    display: flex;
    flex-direction: column;
  }

  h2 {
    margin: 0 0 8px;
    font-size: 20px;
    font-weight: 700;
  }

  .scan-banner {
    display: flex;
    align-items: center;
    gap: 10px;
    margin: 0 0 14px;
    padding: 8px 12px;
    background: var(--surface);
    border: 1px solid var(--border);
    border-radius: var(--r-md);
    font-size: 13px;
    color: var(--text-2);
  }

  .spinner {
    flex-shrink: 0;
    display: inline-block;
    width: 14px;
    height: 14px;
    border: 2px solid var(--border);
    border-top-color: var(--accent);
    border-radius: 50%;
    animation: spin 0.8s linear infinite;
  }

  @keyframes spin {
    to { transform: rotate(360deg); }
  }

  .subtitle {
    margin: 0 0 16px;
    font-size: 13px;
  }

  .empty {
    margin-top: 32px;
    text-align: center;
  }

  .groups {
    flex: 1;
    overflow-y: auto;
  }

  .group {
    border: 1px solid var(--border);
    border-radius: var(--r-md);
    margin-bottom: 8px;
    background: var(--surface);
  }

  .group-header {
    display: flex;
    align-items: center;
    gap: 8px;
    width: 100%;
    padding: 10px 12px;
    text-align: left;
    font-size: 13px;
    color: var(--text-1);
    transition: background 0.1s;
  }

  .group-header:hover {
    background: var(--surface-hover);
  }

  .chevron {
    display: inline-block;
    transition: transform 0.15s;
    font-size: 11px;
    color: var(--text-3);
  }
  .chevron.open { transform: rotate(90deg); }

  .group-title {
    flex: 1;
    min-width: 0;
  }

  .badge {
    font-size: 11px;
    padding: 2px 6px;
    border-radius: var(--r-sm);
    white-space: nowrap;
  }
  .badge.exact {
    background: rgba(248, 113, 113, 0.15);
    color: var(--danger);
  }
  .badge.acoustic {
    background: rgba(250, 204, 21, 0.15);
    color: #facc15;
  }

  .count {
    font-size: 12px;
    white-space: nowrap;
  }

  .track-list {
    list-style: none;
    padding: 0;
    margin: 0;
    border-top: 1px solid var(--border);
  }

  .track-row {
    display: flex;
    align-items: center;
    gap: 8px;
    padding: 6px 12px 6px 32px;
    font-size: 12px;
    border-bottom: 1px solid var(--border);
  }
  .track-row:last-child { border-bottom: none; }

  .track-row.is-dismissed {
    opacity: 0.4;
  }

  .track-path {
    flex: 1;
    min-width: 0;
    font-family: monospace;
    font-size: 11px;
    color: var(--text-2);
  }

  .track-dur {
    min-width: 40px;
    text-align: right;
    font-size: 11px;
  }

  .action {
    font-size: 13px;
    padding: 2px 6px;
    border-radius: var(--r-sm);
    transition: background 0.1s;
  }
  .action:hover { background: var(--surface-hover); }

  .dismiss { color: var(--danger); }
  .restore { color: var(--accent); }
</style>
