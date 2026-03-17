<script lang="ts">
  import {
    ConnectToServer,
    DisconnectFromServer,
    ClearArtworkCache
  } from "../../wailsjs/go/desktop/App";
  import {
    connected,
    serverURL,
    authToken,
    refreshConnection,
    saveSession,
    clearSession,
    isReconnecting,
    stopAutoReconnect
  } from "../utils/api";
  import { RotateCcw } from "@lucide/svelte";
  import { BrowserOpenURL } from "../../wailsjs/runtime";

  let connectURL = "http://127.0.0.1:8989";
  let connectUser = "";
  let connectPass = "";
  let connectErr = "";
  let connecting = false;

  let cacheCleared = false;

  async function connect() {
    connectErr = "";
    connecting = true;
    try {
      await ConnectToServer(connectURL, connectUser, connectPass);
      await refreshConnection();
      stopAutoReconnect();

      // Persist only the URL + fresh token
      saveSession(connectURL, $authToken);

      connectUser = "";
      connectPass = "";
    } catch (e: any) {
      connectErr = e?.toString() ?? "Connection failed";
    }
    connecting = false;
  }

  async function disconnect() {
    stopAutoReconnect();
    clearSession();
    await DisconnectFromServer();
    await refreshConnection();
  }

  function handleOpenUrl(url: string) {
    BrowserOpenURL(url);
  }

  function handleArtworkCacheClear() {
    ClearArtworkCache();
    cacheCleared = true;
    setTimeout(() => (cacheCleared = false), 3000);
  }
</script>

<section>
  <h2>Settings</h2>
  <div class="group">
    <h3>Server Connection</h3>
    {#if $connected}
      <p class="text-3 connected-status">
        Connected to <code>{$serverURL}</code>
      </p>
      <button class="btn-danger" on:click={disconnect}>Disconnect</button>
    {:else if $isReconnecting}
      <p class="text-3 reconnecting-status">
        <RotateCcw size={14} /> Reconnecting to server...
      </p>
      <button
        class="btn-danger"
        on:click={() => {
          stopAutoReconnect();
        }}>Cancel</button
      >
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
        <button
          on:click={connect}
          disabled={connecting || !connectURL || !connectUser || !connectPass}
        >
          {connecting ? "Connecting..." : "Connect"}
        </button>
        {#if connectErr}
          <p class="msg error">{connectErr}</p>
        {/if}
      </div>
    {/if}
  </div>

  <div class="group">
    <h3>Cache</h3>
    <p class="text-3">
      Thumbnail images are cached on disk. Clear the cache whenever issues with
      album artwork arise.
    </p>
    <button on:click={handleArtworkCacheClear}>Clear Artwork Cache</button>
    {#if cacheCleared}<p class="msg">Cache cleared.</p>{/if}
  </div>

  <div class="group">
    <h3>About</h3>
    <p class="text-3">
      pneuma, an open-source, self-hostable, and local-first music player and
      server.
    </p>
    <p class="text-3">v0.1.0</p>
    <p class="text-3">
      Source code available on <button
        on:click|preventDefault={() =>
          handleOpenUrl("https://github.com/johncmanuel/pneuma")}
        aria-label="GitHub repository"
        style="text-decoration: underline;"
      >
        GitHub
      </button>.
    </p>
  </div>
</section>

<style>
  section {
    height: 100%;
    display: flex;
    flex-direction: column;
    gap: 32px;
    overflow-y: auto;
  }
  h2 {
    margin: 0 0 4px;
    font-size: 20px;
    font-weight: 700;
  }

  .group {
    display: flex;
    flex-direction: column;
    gap: 8px;
  }
  h3 {
    margin: 0;
    font-size: 14px;
    font-weight: 600;
  }

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

  .connected-status {
    display: flex;
    align-items: center;
    gap: 6px;
  }
  .reconnecting-status {
    display: flex;
    align-items: center;
    gap: 6px;
    color: var(--accent);
  }

  .btn-danger {
    color: #ef4444;
  }
  .btn-danger:hover {
    background: color-mix(in srgb, #ef4444 15%, transparent);
  }

  .msg {
    font-size: 13px;
    margin: 0;
    color: var(--accent);
  }
  .msg.error {
    color: #ef4444;
  }

  code {
    background: var(--surface);
    padding: 2px 6px;
    border-radius: 4px;
    font-size: 12px;
  }
</style>
