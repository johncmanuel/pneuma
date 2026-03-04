<script lang="ts">
  import { TriggerScan } from "../../wailsjs/go/main/App"

  let watchFolder = ""
  let message = ""
  let scanning = false

  async function scan() {
    scanning = true
    message = ""
    try {
      TriggerScan()
      message = "Scan started."
    } catch (e) {
      message = `Error: ${e}`
    }
    scanning = false
  }
</script>

<section>
  <h2>Settings</h2>

  <div class="group">
    <h3>Watch Folders</h3>
    <p class="text-3">Edit <code>~/.pneuma/config.toml</code> to add or remove watch folders, then rescan.</p>
    <button on:click={scan} disabled={scanning}>
      {scanning ? "Scanning…" : "↺ Rescan Now"}
    </button>
    {#if message}
      <p class="msg">{message}</p>
    {/if}
  </div>

  <div class="group">
    <h3>About</h3>
    <p class="text-3">pneuma — open-source, self-hosted music server</p>
    <p class="text-3">v0.1.0</p>
  </div>
</section>

<style>
  section { height: 100%; display: flex; flex-direction: column; gap: 32px; }
  h2 { margin: 0 0 4px; font-size: 20px; font-weight: 700; }

  .group { display: flex; flex-direction: column; gap: 8px; }
  h3 { margin: 0; font-size: 14px; font-weight: 600; }

  code {
    background: var(--surface);
    padding: 2px 6px;
    border-radius: 4px;
    font-size: 12px;
  }

  .msg { color: var(--accent); font-size: 13px; margin: 0; }
</style>
