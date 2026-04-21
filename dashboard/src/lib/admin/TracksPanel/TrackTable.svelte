<script lang="ts">
  import { SortButton } from "@pneuma/ui";
  import { formatDuration } from "@pneuma/shared";
  import type { Track, SortKey, SortDir } from "./types";
  import TrackEditRow from "./TrackEditRow.svelte";

  interface Props {
    sortedTracks: Track[];
    searchQuery: string;
    totalTracksCount: number;
    filteredTracksCount: number;
    sortKey: SortKey;
    sortDir: SortDir;
    canEdit: boolean;
    canDelete: boolean;
    canUpload: boolean;
    allSelected: boolean;
    selectedIds: Set<string>;
    editingTrackId: string | null;
    onToggleSelectAll: () => void;
    onToggleSelect: (id: string) => void;
    onStartEdit: (t: Track) => void;
    onSaveEdit: (
      id: string,
      title: string,
      artist: string,
      album: string
    ) => void;
    onCancelEdit: () => void;
    onDeleteTrack: (id: string) => void;
    onClickReplace: (t: Track) => void;
  }

  let {
    sortedTracks,
    searchQuery,
    totalTracksCount,
    filteredTracksCount,
    sortKey = $bindable(),
    sortDir = $bindable(),
    canEdit,
    canDelete,
    canUpload,
    allSelected,
    selectedIds,
    editingTrackId,
    onToggleSelectAll,
    onToggleSelect,
    onStartEdit,
    onSaveEdit,
    onCancelEdit,
    onDeleteTrack,
    onClickReplace
  }: Props = $props();
</script>

<div class="table-wrap">
  <table>
    <thead>
      <tr>
        {#if canDelete}
          <th class="col-check">
            <input
              type="checkbox"
              checked={allSelected}
              onchange={onToggleSelectAll}
              title="Select all"
            />
          </th>
        {/if}
        <th class="sortable">
          <SortButton bind:currentField={sortKey} bind:sortDir field="title"
            >Title</SortButton
          >
        </th>
        <th class="sortable">
          <SortButton
            bind:currentField={sortKey}
            bind:sortDir
            field="album_artist">Artist</SortButton
          >
        </th>
        <th class="sortable">
          <SortButton
            bind:currentField={sortKey}
            bind:sortDir
            field="album_name">Album</SortButton
          >
        </th>
        <th class="sortable col-dur">
          <SortButton
            bind:currentField={sortKey}
            bind:sortDir
            field="duration_ms">Duration</SortButton
          >
        </th>
        {#if canEdit || canDelete}
          <th>Actions</th>
        {/if}
      </tr>
    </thead>
    <tbody>
      {#each sortedTracks as t, i (t.id || i)}
        {#if editingTrackId === t.id}
          <TrackEditRow
            track={t}
            {canDelete}
            isSelected={selectedIds.has(t.id)}
            onSave={onSaveEdit}
            onCancel={onCancelEdit}
            onToggleSelect={() => onToggleSelect(t.id)}
          />
        {:else}
          <tr class:selected={selectedIds.has(t.id)}>
            {#if canDelete}
              <td class="col-check">
                <input
                  type="checkbox"
                  checked={selectedIds.has(t.id)}
                  onchange={() => onToggleSelect(t.id)}
                />
              </td>
            {/if}
            <td class="truncate">{t.title}</td>
            <td class="truncate text-2">{t.album_artist || "–"}</td>
            <td class="truncate text-2">{t.album_name || "–"}</td>
            <td class="text-3">{formatDuration(t.duration_ms)}</td>
            {#if canEdit || canDelete}
              <td class="action-cell">
                {#if canEdit}
                  <button class="sm-btn" onclick={() => onStartEdit(t)}
                    >Edit</button
                  >
                  {#if canUpload}
                    <button class="sm-btn" onclick={() => onClickReplace(t)}
                      >Replace</button
                    >
                  {/if}
                {/if}
                {#if canDelete}
                  <button
                    class="sm-btn danger"
                    onclick={() => onDeleteTrack(t.id)}>Delete</button
                  >
                {/if}
              </td>
            {/if}
          </tr>
        {/if}
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
    {sortedTracks.length}{filteredTracksCount !== totalTracksCount
      ? ` of ${totalTracksCount}`
      : ""} track{sortedTracks.length !== 1 ? "s" : ""}
  </p>
{/if}

<style>
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
    background: var(--success-soft);
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
    cursor: pointer;
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
