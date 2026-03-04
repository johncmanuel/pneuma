<script lang="ts">
  import { onMount } from "svelte"

  const BASE = () => `http://127.0.0.1:${(window as any).__PNEUMA_PORT__ || 8989}`
  const userId = "default"

  interface Pack {
    track_id: string
    user_id: string
    local_path: string
    size_bytes: number
    downloaded_at: string
  }

  let packs: Pack[] = []
  let loading = true

  onMount(async () => {
    await loadPacks()
  })

  async function loadPacks() {
    loading = true
    const res = await fetch(`${BASE()}/api/offline/${userId}`)
    if (res.ok) packs = await res.json()
    loading = false
  }

  async function removeTrack(trackId: string) {
    await fetch(`${BASE()}/api/offline/${userId}/tracks/${trackId}`, { method: "DELETE" })
    await loadPacks()
  }

  function fmtBytes(b: number): string {
    if (b < 1024) return `${b} B`
    if (b < 1024 * 1024) return `${(b / 1024).toFixed(1)} KB`
    return `${(b / (1024 * 1024)).toFixed(1)} MB`
  }
</script>

<section>
  <h2>Downloads</h2>

  {#if loading}
    <p class="text-3">Loading…</p>
  {:else if packs.length === 0}
    <p class="text-3">No offline downloads. Right-click tracks to download for offline use.</p>
  {:else}
    <ul class="pack-list">
      {#each packs as pack (pack.track_id)}
        <li>
          <span class="track-id">{pack.track_id}</span>
          <span class="size text-3">{fmtBytes(pack.size_bytes)}</span>
          <button on:click={() => removeTrack(pack.track_id)} class="del" title="Remove">✕</button>
        </li>
      {/each}
    </ul>
  {/if}
</section>

<style>
  section { height: 100%; display: flex; flex-direction: column; }
  h2 { margin: 0 0 16px; font-size: 20px; font-weight: 700; }

  .pack-list { list-style: none; padding: 0; margin: 0; overflow-y: auto; flex: 1; }

  li {
    display: flex;
    align-items: center;
    gap: 12px;
    padding: 8px 4px;
    border-bottom: 1px solid var(--border);
    font-size: 13px;
  }

  .track-id { flex: 1; font-family: monospace; font-size: 12px; }
  .size { min-width: 60px; text-align: right; }

  .del {
    background: transparent;
    border: none;
    color: var(--fg-3);
    cursor: pointer;
    font-size: 14px;
    padding: 2px 6px;
  }
  .del:hover { color: #ef4444; }
</style>
