<script lang="ts">
  import { formatDuration } from "@pneuma/shared";
  import type { Track } from "./types";
  import { untrack } from "svelte";

  interface Props {
    track: Track;
    canDelete: boolean;
    isSelected: boolean;
    onSave: (
      trackId: string,
      title: string,
      artist: string,
      album: string
    ) => void;
    onCancel: () => void;
    onToggleSelect: () => void;
  }

  let {
    track,
    canDelete,
    isSelected,
    onSave,
    onCancel,
    onToggleSelect
  }: Props = $props();

  let editTitle = $state(untrack(() => track.title || ""));
  let editArtist = $state(untrack(() => track.album_artist || ""));
  let editAlbum = $state(untrack(() => track.album_name || ""));

  // reset local state when track changes
  $effect(() => {
    if (track) {
      editTitle = track.title || "";
      editArtist = track.album_artist || "";
      editAlbum = track.album_name || "";
    }
  });

  function handleKeydown(e: KeyboardEvent) {
    if (e.key === "Enter") {
      e.preventDefault();
      onSave(track.id, editTitle, editArtist, editAlbum);
    }
    if (e.key === "Escape") {
      e.preventDefault();
      onCancel();
    }
  }
</script>

<tr class="selected" onkeydown={handleKeydown}>
  {#if canDelete}
    <td class="col-check">
      <input type="checkbox" checked={isSelected} onchange={onToggleSelect} />
    </td>
  {/if}
  <td>
    <!-- svelte-ignore a11y_autofocus -->
    <input type="text" bind:value={editTitle} class="inline-input" autofocus />
  </td>
  <td>
    <input type="text" bind:value={editArtist} class="inline-input" />
  </td>
  <td>
    <input type="text" bind:value={editAlbum} class="inline-input" />
  </td>
  <td>{formatDuration(track.duration_ms)}</td>
  <td class="action-cell">
    <button
      class="sm-btn save"
      onclick={() => onSave(track.id, editTitle, editArtist, editAlbum)}
      >Save</button
    >
    <button class="sm-btn" onclick={onCancel}>Cancel</button>
  </td>
</tr>

<style>
  .col-check {
    width: 32px;
    text-align: center;
  }
  td {
    padding: 6px 12px;
    border-bottom: 1px solid var(--border);
    max-width: 200px;
  }
  tr.selected {
    background: var(--success-soft);
  }

  .inline-input {
    padding: 2px 6px;
    font-size: 13px;
    width: 100%;
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
  .sm-btn.save {
    color: var(--accent);
    border-color: var(--accent-dim);
  }

  input[type="checkbox"] {
    width: 15px;
    height: 15px;
    accent-color: var(--accent);
    cursor: pointer;
  }
</style>
