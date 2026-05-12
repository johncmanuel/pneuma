<script lang="ts">
  import { formatDuration } from "@pneuma/shared";
  import { AUDIO_ACCEPT } from "./uploader";
  import type { Track } from "./types";

  interface Props {
    track: Track | null;
    canEdit: boolean;
    canUpload: boolean;
    saving: boolean;
    onClose: () => void;
    onSave: (id: string, patch: Record<string, string | number>) => void;
    onReplaceTrack: (file: File, track: Track) => Promise<boolean>;
  }

  let {
    track,
    canEdit,
    canUpload,
    saving,
    onClose,
    onSave,
    onReplaceTrack
  }: Props = $props();

  let editTitle = $state("");
  let editArtist = $state("");
  let editAlbum = $state("");
  let editTrackNumber = $state(0);
  let editDiscNumber = $state(0);

  let replaceInput: HTMLInputElement | undefined = $state();

  $effect(() => {
    if (!track) return;
    editTitle = track.title || "";
    editArtist = track.album_artist || "";
    editAlbum = track.album_name || "";
    editTrackNumber = track.track_number ?? 0;
    editDiscNumber = track.disc_number ?? 0;
  });

  let isDirty = $derived.by(() => {
    if (!track) return false;
    return (
      editTitle.trim() !== (track.title || "") ||
      editArtist.trim() !== (track.album_artist || "") ||
      editAlbum.trim() !== (track.album_name || "") ||
      editTrackNumber !== (track.track_number ?? 0) ||
      editDiscNumber !== (track.disc_number ?? 0)
    );
  });

  function buildPatch(): Record<string, string | number> {
    if (!track) return {};
    const patch: Record<string, string | number> = {};
    const nextTitle = editTitle.trim();
    const nextArtist = editArtist.trim();
    const nextAlbum = editAlbum.trim();

    if (nextTitle !== (track.title || "")) patch.title = nextTitle;
    if (nextArtist !== (track.album_artist || ""))
      patch.album_artist = nextArtist;
    if (nextAlbum !== (track.album_name || "")) patch.album_name = nextAlbum;
    if (editTrackNumber !== (track.track_number ?? 0))
      patch.track_number = editTrackNumber;
    if (editDiscNumber !== (track.disc_number ?? 0))
      patch.disc_number = editDiscNumber;

    return patch;
  }

  function handleSave() {
    if (!track) return;
    const patch = buildPatch();
    if (Object.keys(patch).length === 0) return;
    onSave(track.id, patch);
  }

  function requestClose() {
    if (saving) return;
    if (isDirty && !confirm("Discard unsaved changes?")) return;
    onClose();
  }

  function handleOverlayClick(event: MouseEvent) {
    if (event.target === event.currentTarget) requestClose();
  }

  function handleKeydown(event: KeyboardEvent) {
    if (!track) return;
    if (event.key === "Escape") {
      event.preventDefault();
      requestClose();
    }
  }

  async function handleReplaceInput() {
    const file = replaceInput?.files?.[0];
    if (file && track) {
      const ok = await onReplaceTrack(file, track);
      if (ok) requestClose();
    }
    if (replaceInput) replaceInput.value = "";
  }
</script>

<svelte:window onkeydown={handleKeydown} />

{#if track}
  <div class="drawer-overlay" role="presentation" onclick={handleOverlayClick}>
    <div class="drawer" role="dialog" aria-modal="true" aria-label="Edit track">
      <header class="drawer-header">
        <div class="drawer-title">
          <h3>Edit Track</h3>
          <p>{track.title || "Untitled"}</p>
        </div>
        <button class="icon-btn" onclick={requestClose}> Close </button>
      </header>

      <div class="drawer-body">
        <div class="status-row">
          {#if isDirty}
            <span class="unsaved">Unsaved changes</span>
          {:else}
            <span class="text-3">All changes saved</span>
          {/if}
          <span class="text-3"
            >Duration: {formatDuration(track.duration_ms)}</span
          >
        </div>

        <section>
          <h4>Core Info</h4>
          <div class="field-grid">
            <label class="field">
              <span class="field-label">Title</span>
              <input
                type="text"
                bind:value={editTitle}
                class="field-input"
                disabled={!canEdit || saving}
                // autofocus
              />
            </label>
            <label class="field">
              <span class="field-label">Artist (Album Artist)</span>
              <input
                type="text"
                bind:value={editArtist}
                class="field-input"
                disabled={!canEdit || saving}
              />
            </label>
            <label class="field">
              <span class="field-label">Album</span>
              <input
                type="text"
                bind:value={editAlbum}
                class="field-input"
                disabled={!canEdit || saving}
              />
            </label>
          </div>
        </section>

        <section>
          <h4>Track Position</h4>
          <div class="field-grid two-col">
            <label class="field">
              <span class="field-label">Track #</span>
              <input
                type="number"
                min="0"
                step="1"
                bind:value={editTrackNumber}
                class="field-input"
                disabled={!canEdit || saving}
                inputmode="numeric"
              />
            </label>
            <label class="field">
              <span class="field-label">Disc #</span>
              <input
                type="number"
                min="0"
                step="1"
                bind:value={editDiscNumber}
                class="field-input"
                disabled={!canEdit || saving}
                inputmode="numeric"
              />
            </label>
          </div>
        </section>

        <section>
          <h4>Media</h4>
          {#if canUpload}
            <input
              class="hidden-input"
              type="file"
              accept={AUDIO_ACCEPT}
              bind:this={replaceInput}
              onchange={handleReplaceInput}
            />
            <button
              class="btn secondary"
              onclick={() => replaceInput?.click()}
              disabled={saving}
            >
              Replace audio
            </button>
          {:else}
            <p class="text-3">You don't have permission to replace audio.</p>
          {/if}
        </section>
      </div>

      <footer class="drawer-footer">
        <button class="btn cancel" onclick={requestClose} disabled={saving}>
          Discard
        </button>
        <button
          class="btn save"
          onclick={handleSave}
          disabled={!isDirty || saving || !canEdit}
        >
          {saving ? "Saving..." : "Save changes"}
        </button>
      </footer>
    </div>
  </div>
{/if}

<style>
  .drawer-overlay {
    position: fixed;
    top: 48px;
    left: var(--sidebar-w);
    right: 0;
    bottom: 0;
    background: var(--overlay-strong);
    display: flex;
    justify-content: flex-end;
    z-index: 120;
  }

  .drawer {
    width: min(480px, 100%);
    height: 100%;
    background: var(--surface);
    border-left: 1px solid var(--border);
    display: flex;
    flex-direction: column;
    box-shadow: var(--shadow-pop);
  }

  .drawer-header {
    display: flex;
    align-items: center;
    justify-content: space-between;
    gap: 12px;
    padding: 16px 20px;
    border-bottom: 1px solid var(--border);
  }

  .drawer-title {
    display: flex;
    flex-direction: column;
    gap: 2px;
  }

  .drawer-title h3 {
    margin: 0;
    font-size: 16px;
  }

  .drawer-title p {
    margin: 0;
    font-size: 12px;
    color: var(--text-3);
  }

  .icon-btn {
    padding: 6px 10px;
    border-radius: var(--r-sm);
    font-size: 12px;
    border: 1px solid var(--border);
    background: var(--surface-2);
  }

  .icon-btn:hover {
    background: var(--surface-hover);
  }

  .drawer-body {
    flex: 1;
    overflow-y: auto;
    padding: 16px 20px 24px;
    display: flex;
    flex-direction: column;
    gap: 18px;
  }

  .status-row {
    display: flex;
    justify-content: space-between;
    align-items: center;
    gap: 12px;
    font-size: 12px;
  }

  .unsaved {
    color: var(--warning);
    font-weight: 600;
  }

  section h4 {
    margin: 0 0 8px;
    font-size: 11px;
    text-transform: uppercase;
    letter-spacing: 0.08em;
    color: var(--text-3);
  }

  .field-grid {
    display: grid;
    gap: 12px;
  }

  .field-grid.two-col {
    grid-template-columns: repeat(2, minmax(0, 1fr));
  }

  .field {
    display: flex;
    flex-direction: column;
    gap: 6px;
  }

  .field-label {
    font-size: 12px;
    color: var(--text-2);
  }

  .field-input {
    padding: 6px 10px;
    border-radius: var(--r-sm);
    border: 1px solid var(--border);
    background: var(--surface-2);
    color: var(--text-1);
  }

  .field-input:disabled {
    opacity: 0.6;
  }

  .hidden-input {
    display: none;
  }

  .btn {
    padding: 6px 14px;
    border-radius: var(--r-md);
    font-size: 13px;
    border: 1px solid var(--border);
    cursor: pointer;
  }

  .btn.secondary {
    background: var(--surface-2);
  }

  .btn.secondary:hover:not(:disabled) {
    background: var(--surface-hover);
  }

  .btn.cancel {
    background: var(--surface-2);
  }

  .btn.cancel:hover:not(:disabled) {
    background: var(--surface-hover);
  }

  .btn.save {
    background: var(--accent);
    color: var(--on-accent, #fff);
    border-color: var(--accent);
  }

  .btn.save:hover:not(:disabled) {
    filter: brightness(1.08);
  }

  .btn:disabled {
    opacity: 0.4;
    cursor: not-allowed;
  }

  .drawer-footer {
    padding: 12px 20px;
    border-top: 1px solid var(--border);
    display: flex;
    justify-content: flex-end;
    gap: 8px;
    background: var(--surface);
  }

  @media (max-width: 900px) {
    .drawer-overlay {
      top: 0;
      left: 0;
    }
  }

  @media (max-width: 600px) {
    .field-grid.two-col {
      grid-template-columns: 1fr;
    }
  }
</style>
