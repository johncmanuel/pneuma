<script lang="ts">
  interface Props {
    selectedCount: number;
    saving: boolean;
    onSave: (patch: Record<string, string | number>) => void;
    onClose: () => void;
  }

  let { selectedCount, saving, onSave, onClose }: Props = $props();

  let applyArtist = $state(false);
  let applyAlbum = $state(false);
  let artist = $state("");
  let album = $state("");

  let hasChanges = $derived(applyArtist || applyAlbum);

  function handleSubmit(e: SubmitEvent) {
    e.preventDefault();
    const patch: Record<string, string | number> = {};

    if (applyArtist) patch.album_artist = artist;
    if (applyAlbum) patch.album_name = album;

    onSave(patch);
  }

  function handleBackdrop(e: MouseEvent) {
    if (e.target === e.currentTarget) onClose();
  }

  function handleKeydown(e: KeyboardEvent) {
    if (e.key === "Escape") onClose();
  }
</script>

<svelte:window onkeydown={handleKeydown} />

<!-- svelte-ignore a11y_click_events_have_key_events -->
<!-- svelte-ignore a11y_no_static_element_interactions -->
<div class="backdrop" onclick={handleBackdrop}>
  <div class="modal" role="dialog" aria-label="Bulk edit tracks">
    <h3>Edit {selectedCount} track{selectedCount !== 1 ? "s" : ""}</h3>
    <p class="hint">
      Enable a field to apply the value to all selected tracks.
    </p>

    <form onsubmit={handleSubmit}>
      <label class="field-row">
        <input type="checkbox" bind:checked={applyArtist} />
        <span class="label-text">Album Artist</span>
        <input
          type="text"
          bind:value={artist}
          disabled={!applyArtist}
          placeholder="Album artist…"
          class="field-input"
        />
      </label>

      <label class="field-row">
        <input type="checkbox" bind:checked={applyAlbum} />
        <span class="label-text">Album Name</span>
        <input
          type="text"
          bind:value={album}
          disabled={!applyAlbum}
          placeholder="Album name…"
          class="field-input"
        />
      </label>

      <div class="modal-actions">
        <button
          type="button"
          class="btn cancel-btn"
          onclick={onClose}
          disabled={saving}
        >
          Cancel
        </button>
        <button
          type="submit"
          class="btn save-btn"
          disabled={!hasChanges || saving}
        >
          {saving ? "Saving…" : "Apply Changes"}
        </button>
      </div>
    </form>
  </div>
</div>

<style>
  .backdrop {
    position: fixed;
    inset: 0;
    background: rgba(0, 0, 0, 0.55);
    display: flex;
    align-items: center;
    justify-content: center;
    z-index: 200;
  }
  .modal {
    background: var(--surface);
    border: 1px solid var(--border);
    border-radius: var(--r-lg, 12px);
    padding: 24px;
    width: min(420px, 90vw);
    box-shadow: 0 8px 32px rgba(0, 0, 0, 0.35);
  }
  h3 {
    margin: 0 0 4px;
    font-size: 16px;
  }
  .hint {
    margin: 0 0 16px;
    font-size: 12px;
    color: var(--text-3);
  }
  .field-row {
    display: flex;
    align-items: center;
    gap: 8px;
    margin-bottom: 10px;
  }
  .field-row input[type="checkbox"] {
    width: 15px;
    height: 15px;
    accent-color: var(--accent);
    cursor: pointer;
    flex-shrink: 0;
  }
  .label-text {
    width: 100px;
    flex-shrink: 0;
    font-size: 13px;
    color: var(--text-2);
  }
  .field-input {
    flex: 1;
    min-width: 0;
  }
  .field-input:disabled {
    opacity: 0.4;
  }
  .modal-actions {
    display: flex;
    gap: 8px;
    justify-content: flex-end;
    margin-top: 18px;
  }
  .btn {
    padding: 6px 16px;
    border-radius: var(--r-md);
    font-size: 13px;
    border: 1px solid var(--border);
    cursor: pointer;
  }
  .btn:disabled {
    opacity: 0.4;
    cursor: not-allowed;
  }
  .cancel-btn {
    background: var(--surface-2);
  }
  .cancel-btn:hover:not(:disabled) {
    background: var(--surface-hover);
  }
  .save-btn {
    background: var(--accent);
    color: var(--on-accent, #fff);
    border-color: var(--accent);
  }
  .save-btn:hover:not(:disabled) {
    filter: brightness(1.1);
  }
</style>
