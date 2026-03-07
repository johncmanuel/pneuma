<script lang="ts">
  import { ConnectToServer, DisconnectFromServer, ClearArtworkCache } from "../../wailsjs/go/main/App"
  import { connected, serverURL, authToken, refreshConnection, saveSession, clearSession, isReconnecting, stopAutoReconnect } from "../utils/api"
  import { loadTracks } from "../stores/library"
  import { autoDupeCheck, scanProgress, localLoading } from "../stores/localLibrary"

  // Connect form state
  let connectURL = "http://127.0.0.1:8989"
  let connectUser = ""
  let connectPass = ""
  let connectErr = ""
  let connecting = false

  // Scan state
  let scanMsg = ""
  let scanning = false

  // Cache state
  let cacheCleared = false

  async function connect() {
    connectErr = ""
    connecting = true
    try {
      await ConnectToServer(connectURL, connectUser, connectPass)
      await refreshConnection()
      stopAutoReconnect()
      // Persist the URL + fresh token — never the password.
      saveSession(connectURL, $authToken)
      await loadTracks()
      connectUser = ""
      connectPass = ""
    } catch (e: any) {
      connectErr = e?.toString() ?? "Connection failed"
    }
    connecting = false
  }

  async function disconnect() {
    stopAutoReconnect()
    clearSession()
    await DisconnectFromServer()
    await refreshConnection()
  }

  async function scan() {
    scanMsg = ""
    scanning = true
    try {
      const res = await fetch(`${$serverURL}/api/library/scan`, {
        method: "POST",
        headers: { Authorization: `Bearer ${$authToken}` },
      })
      if (res.status === 403) {
        scanMsg = "Admin access required."
      } else if (!res.ok) {
        scanMsg = `Server error: ${res.status}`
      } else {
        scanMsg = "Scan started."
      }
    } catch (e) {
      scanMsg = `Error: ${e}`
    }
    scanning = false
  }
</script>

<section>
  <h2>Settings</h2>

  <!-- ── Server connection ── -->
  <div class="group">
    <h3>Server Connection</h3>
    {#if $connected}
      <p class="text-3 connected-status">
        ✓ Connected to <code>{$serverURL}</code>
      </p>
      <button class="btn-danger" on:click={disconnect}>Disconnect</button>
    {:else if $isReconnecting}
      <p class="text-3 reconnecting-status">
        ↺ Reconnecting to server…
      </p>
      <button class="btn-danger" on:click={() => { stopAutoReconnect() }}>Cancel</button>
    {:else}
      <div class="connect-form">
        <input
          type="url"
          placeholder="http://192.168.1.10:8989"
          bind:value={connectURL}
        />
        <input
          type="text"
          placeholder="Username"
          bind:value={connectUser}
          autocomplete="username"
        />
        <input
          type="password"
          placeholder="Password"
          bind:value={connectPass}
          autocomplete="current-password"
          on:keydown={(e) => e.key === "Enter" && connect()}
        />
        <button on:click={connect} disabled={connecting || !connectURL || !connectUser || !connectPass}>
          {connecting ? "Connecting…" : "Connect"}
        </button>
        {#if connectErr}
          <p class="msg error">{connectErr}</p>
        {/if}
      </div>
    {/if}
  </div>

  <!-- ── Watch folders / scan ── -->
  <div class="group">
    <h3>Watch Folders</h3>
    <p class="text-3">Edit <code>~/.pneuma/config.toml</code> to add or remove watch folders, then rescan.</p>
    {#if $connected}
      <button on:click={scan} disabled={scanning}>
        {scanning ? "Scanning…" : "↺ Rescan Now"}
      </button>
      {#if scanMsg}
        <p class="msg">{scanMsg}</p>
      {/if}
    {:else}
      <p class="text-3 muted">Connect to a server to trigger a rescan.</p>
    {/if}
  </div>

  <!-- ── Local files ── -->
  <div class="group">
    <h3>Local Files</h3>
    {#if $scanProgress}
      <p class="text-3 scan-progress">
        Scanning <code>{$scanProgress.folder.split('/').pop()}</code> — {$scanProgress.done} / {$scanProgress.total} songs
      </p>
      <div class="progress-bar">
        <div class="progress-fill" style="width: {($scanProgress.total > 0 ? ($scanProgress.done / $scanProgress.total) * 100 : 0)}%"></div>
      </div>
    {:else if $localLoading}
      <p class="text-3 muted">Loading local library…</p>
    {/if}
    <label class="toggle-row">
      <input type="checkbox" bind:checked={$autoDupeCheck} />
      <span>Auto-check for duplicates on startup</span>
    </label>
    <p class="text-3 muted">When disabled, use the "Check Now" button in Library → Local Files → Duplicates.</p>
  </div>

  <!-- ── Cache ── -->
  <div class="group">
    <h3>Cache</h3>
    <p class="text-3">Thumbnail images are cached on disk to speed up the album grid.</p>
    <button on:click={async () => {
      await ClearArtworkCache()
      cacheCleared = true
      setTimeout(() => cacheCleared = false, 3000)
    }}>Clear Artwork Cache</button>
    {#if cacheCleared}<p class="msg">Cache cleared.</p>{/if}
  </div>

  <div class="group">
    <h3>About</h3>
    <p class="text-3">pneuma — open-source, self-hosted music server</p>
    <p class="text-3">v0.1.0</p>
  </div>
</section>

<style>
  section { height: 100%; display: flex; flex-direction: column; gap: 32px; overflow-y: auto; }
  h2 { margin: 0 0 4px; font-size: 20px; font-weight: 700; }

  .group { display: flex; flex-direction: column; gap: 8px; }
  h3 { margin: 0; font-size: 14px; font-weight: 600; }

  .connect-form {
    display: flex;
    flex-direction: column;
    gap: 8px;
    max-width: 320px;
  }

  .connect-form input {
    padding: 7px 10px;
    border-radius: var(--r-sm);
    border: 1px solid var(--border);
    background: var(--surface);
    color: var(--fg);
    font-size: 13px;
  }

  .connect-form input:focus {
    outline: none;
    border-color: var(--accent);
  }

  .connected-status { display: flex; align-items: center; gap: 6px; }
  .reconnecting-status { display: flex; align-items: center; gap: 6px; color: var(--accent); }

  .btn-danger { color: #ef4444; }
  .btn-danger:hover { background: color-mix(in srgb, #ef4444 15%, transparent); }

  .msg { font-size: 13px; margin: 0; color: var(--accent); }
  .msg.error { color: #ef4444; }
  .muted { opacity: 0.5; }

  .toggle-row {
    display: flex;
    align-items: center;
    gap: 8px;
    font-size: 13px;
    cursor: pointer;
  }
  .toggle-row input[type="checkbox"] {
    width: 16px;
    height: 16px;
    cursor: pointer;
    accent-color: var(--accent);
  }

  code {
    background: var(--surface);
    padding: 2px 6px;
    border-radius: 4px;
    font-size: 12px;
  }

  .scan-progress {
    display: flex;
    align-items: center;
    gap: 6px;
    color: var(--accent);
  }

  .progress-bar {
    width: 100%;
    max-width: 320px;
    height: 4px;
    background: var(--border);
    border-radius: 2px;
    overflow: hidden;
  }

  .progress-fill {
    height: 100%;
    background: var(--accent);
    border-radius: 2px;
    transition: width 0.15s ease-out;
  }
</style>
