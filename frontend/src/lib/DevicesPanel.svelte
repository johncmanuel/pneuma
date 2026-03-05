<script lang="ts">
  import { onMount } from "svelte"
  import { playerState, type Track } from "../stores/player"
  import { tracks } from "../stores/library"
  import { closePanel } from "../stores/ui"
  import { serverFetch, connected } from "./api"

  interface Session {
    id: string
    device_id: string
    user_id: string
    track_id: string
    position_ms: number
    queue: string[]
    updated_at: string
  }

  let sessions: Session[] = []
  let loading = true
  let transferring: string | null = null
  const currentDevice = "desktop"

  $: trackMap = new Map(($tracks as Track[]).map(t => [t.id, t]))

  onMount(() => {
    fetchSessions()
  })

  async function fetchSessions() {
    if (!$connected) { loading = false; return }
    loading = true
    try {
      const res = await serverFetch("/api/sessions")
      if (res.ok) {
        sessions = (await res.json()) ?? []
      }
    } catch {
      // ignore
    }
    loading = false
  }

  async function transferTo(deviceId: string) {
    if (!$connected) return
    transferring = deviceId
    try {
      await serverFetch("/api/handoff", {
        method: "POST",
        headers: { "Content-Type": "application/json" },
        body: JSON.stringify({
          source_device_id: currentDevice,
          target_device_id: deviceId,
        }),
      })
    } catch {
      // ignore
    }
    transferring = null
  }

  function deviceLabel(id: string): string {
    if (id === currentDevice) return "This device"
    return id.replace(/-/g, " ").replace(/\b\w/g, c => c.toUpperCase())
  }

  function close() {
    closePanel()
  }
</script>

<aside class="devices-panel">
  <div class="panel-header">
    <h3>Devices</h3>
    <button class="close-btn" on:click={close} title="Close">&times;</button>
  </div>

  {#if loading}
    <p class="status text-3">Loading devices…</p>
  {:else if sessions.length === 0}
    <p class="status text-3">No active sessions found.</p>
  {:else}
    <div class="device-list">
      {#each sessions as session (session.device_id)}
        {@const isThis = session.device_id === currentDevice}
        {@const sessionTrack = trackMap.get(session.track_id)}
        <div class="device-item" class:active={isThis}>
          <div class="device-info">
            <span class="device-icon">{isThis ? "💻" : "📡"}</span>
            <div class="device-meta">
              <span class="device-name">{deviceLabel(session.device_id)}</span>
              {#if sessionTrack}
                <span class="device-track truncate text-3">
                  {sessionTrack.title} — {sessionTrack.album_artist || "Unknown"}
                </span>
              {:else if session.track_id}
                <span class="device-track truncate text-3">Playing…</span>
              {:else}
                <span class="device-track text-3">Idle</span>
              {/if}
            </div>
          </div>
          {#if !isThis}
            <button
              class="transfer-btn"
              on:click={() => transferTo(session.device_id)}
              disabled={transferring === session.device_id}
              title="Transfer playback here"
            >
              {transferring === session.device_id ? "…" : "▶"}
            </button>
          {:else}
            <span class="current-badge">Current</span>
          {/if}
        </div>
      {/each}
    </div>
  {/if}

  <div class="panel-footer">
    <button class="refresh-btn" on:click={fetchSessions} title="Refresh">↺ Refresh</button>
  </div>
</aside>

<style>
  .devices-panel {
    display: flex;
    flex-direction: column;
    background: var(--surface);
    border-left: 1px solid var(--border);
    height: 100%;
    overflow: hidden;
  }

  .panel-header {
    display: flex;
    align-items: center;
    justify-content: space-between;
    padding: 16px;
    flex-shrink: 0;
  }

  h3 { margin: 0; font-size: 16px; font-weight: 700; }

  .close-btn {
    font-size: 20px;
    color: var(--text-3);
    padding: 2px 6px;
    line-height: 1;
  }
  .close-btn:hover { color: var(--text-1); }

  .status { padding: 16px; font-size: 13px; }

  .device-list {
    flex: 1;
    overflow-y: auto;
    padding: 0 8px;
  }

  .device-item {
    display: flex;
    align-items: center;
    justify-content: space-between;
    padding: 10px 8px;
    border-radius: 6px;
    gap: 8px;
  }
  .device-item:hover { background: var(--surface-hover); }
  .device-item.active { background: var(--surface-hover); }

  .device-info {
    display: flex;
    align-items: center;
    gap: 10px;
    min-width: 0;
    flex: 1;
  }

  .device-icon { font-size: 20px; flex-shrink: 0; }

  .device-meta {
    display: flex;
    flex-direction: column;
    min-width: 0;
    gap: 2px;
  }

  .device-name { font-size: 13px; font-weight: 600; }
  .device-track { font-size: 11px; }

  .transfer-btn {
    background: var(--accent);
    color: #000;
    border-radius: 50%;
    width: 28px;
    height: 28px;
    font-size: 12px;
    display: flex;
    align-items: center;
    justify-content: center;
    flex-shrink: 0;
  }
  .transfer-btn:hover:not(:disabled) { transform: scale(1.06); }
  .transfer-btn:disabled { opacity: 0.4; cursor: not-allowed; }

  .current-badge {
    font-size: 10px;
    text-transform: uppercase;
    letter-spacing: 0.05em;
    color: var(--accent);
    font-weight: 600;
    flex-shrink: 0;
  }

  .panel-footer {
    padding: 12px 16px;
    border-top: 1px solid var(--border);
    flex-shrink: 0;
  }

  .refresh-btn {
    font-size: 12px;
    color: var(--text-2);
    padding: 4px 8px;
    border-radius: var(--r-sm);
  }
  .refresh-btn:hover { color: var(--text-1); background: var(--surface-hover); }
</style>
