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
  import { recentAlbums, recentPlaylists } from "../stores/recentAlbums";
  import {
    favoritesSyncEnabled,
    setFavoritesSyncEnabled
  } from "../stores/playlists";
  import {
    streamQuality,
    type StreamPresetOption,
    streamPresetOptions,
    addToast
  } from "@pneuma/shared";
  import { db } from "../utils/db";
  import { RotateCcw, Check, CircleAlert } from "@lucide/svelte";
  import { BrowserOpenURL } from "../../wailsjs/runtime";

  function handlePresetClick(option: StreamPresetOption) {
    streamQuality.set(option.value);
  }

  let connectURL = $state("http://127.0.0.1:8989");
  let connectUser = $state("");
  let connectPass = $state("");
  let connectErr = $state("");
  let connecting = $state(false);

  let cacheCleared = $state(false);
  let changingFavoritesSync = $state(false);

  const githubUrl = "https://github.com/johncmanuel/pneuma";

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

  async function handleResetRecent() {
    await db.clearAllRecent();
    recentAlbums.set([]);
    recentPlaylists.set([]);
    addToast("Recently played has been reset.", "info");
  }

  async function handleFavoritesSyncToggle(e: Event) {
    const next = (e.currentTarget as HTMLInputElement).checked;
    changingFavoritesSync = true;
    try {
      await setFavoritesSyncEnabled(next);
      addToast(
        next
          ? "Favorites sync enabled"
          : "Favorites sync disabled. Favorites are now local-only.",
        "info"
      );
    } finally {
      changingFavoritesSync = false;
    }
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
      <button class="btn-danger" onclick={disconnect}>Disconnect</button>
    {:else if $isReconnecting}
      <p class="text-3 reconnecting-status">
        <RotateCcw size={14} /> Reconnecting to server...
      </p>
      <button
        class="btn-danger"
        onclick={() => {
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
          onkeydown={(e) => e.key === "Enter" && connect()}
        />
        <button
          onclick={connect}
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
    <button onclick={handleArtworkCacheClear}>Clear Artwork Cache</button>
    {#if cacheCleared}<p class="msg">Cache cleared.</p>{/if}
  </div>

  <div class="group">
    <h3>Data</h3>
    <p class="text-3">
      Clear all recently played albums and playlists from the sidebar.
    </p>
    <button onclick={handleResetRecent}>Reset Recently Played</button>
  </div>

  <div class="group">
    <h3>Favorites</h3>
    <p class="text-3">Sync Favorites with server</p>

    <label class="text-3">
      <input
        type="checkbox"
        checked={$favoritesSyncEnabled}
        onchange={handleFavoritesSyncToggle}
        disabled={changingFavoritesSync}
      />
    </label>
  </div>

  <article class="quality-panel" aria-labelledby="stream-quality-heading">
    <header class="quality-header">
      <h3 id="stream-quality-heading">Streaming Quality</h3>
      <p class="panel-note text-3">
        <CircleAlert size={14} aria-hidden="true" />
        <span>
          Quality changes on the next track. If a higher-quality cached copy is
          already available, playback may continue at that quality.
        </span>
      </p>
    </header>

    <section class="quality-group" aria-labelledby="wifi-streaming-heading">
      <h4 id="wifi-streaming-heading">Wi-Fi streaming quality</h4>
      <p class="group-description text-3">
        Choose the quality used while streaming from your library.
      </p>

      <ul class="preset-list" aria-label="Available streaming quality presets">
        {#each streamPresetOptions as option}
          <li>
            <button
              type="button"
              class="preset-item"
              class:active={option.value === $streamQuality}
              aria-pressed={option.value === $streamQuality}
              onclick={() => handlePresetClick(option)}
            >
              <div class="preset-main">
                <span class="preset-label">{option.label}</span>
                <span class="preset-meta text-3">{option.meta}</span>
                <span class="preset-description text-3"
                  >{option.description}</span
                >
              </div>
              <span class="check-slot" aria-hidden="true">
                {#if option.value === $streamQuality}
                  <Check size={18} class="check-icon" />
                {/if}
              </span>
            </button>
          </li>
        {/each}
      </ul>
    </section>
  </article>

  <div class="group">
    <h3>About</h3>
    <p class="text-3">
      pneuma, an open-source, self-hostable, and local-first music player and
      server.
    </p>
    <p class="text-3">
      Source code available on <button
        onclick={(e) => {
          e.preventDefault();
          handleOpenUrl(githubUrl);
        }}
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

  h4 {
    margin: 0;
    font-size: 13px;
    font-weight: 600;
  }

  .quality-panel {
    display: flex;
    flex-direction: column;
    gap: 10px;
    padding: 0;
    border: none;
    background: transparent;
  }

  .quality-header {
    display: flex;
    flex-direction: column;
    gap: 4px;
  }

  .panel-note {
    margin: 0;
    display: flex;
    align-items: flex-start;
    gap: 6px;
    font-size: 12px;
    line-height: 1.4;
  }

  .panel-note :global(svg) {
    margin-top: 2px;
    flex-shrink: 0;
  }

  .quality-group {
    display: flex;
    flex-direction: column;
    gap: 4px;
  }

  .group-description {
    margin: 0;
    font-size: 12px;
  }

  .preset-list {
    list-style: none;
    margin: 4px 0 0;
    padding: 0;
    display: grid;
    gap: 0;
  }

  .preset-list li {
    border-bottom: 1px solid color-mix(in srgb, var(--border) 88%, transparent);
  }

  .preset-list li:first-child {
    border-top: 1px solid color-mix(in srgb, var(--border) 88%, transparent);
  }

  .preset-item {
    border: 1px solid transparent;
    border-radius: 6px;
    padding: 8px 6px;
    display: flex;
    align-items: flex-start;
    justify-content: space-between;
    gap: 12px;
    background: transparent;
    cursor: pointer;
    font: inherit;
    text-align: left;
    width: 100%;
    transition:
      background 0.12s,
      border-color 0.12s;
  }

  .preset-item:hover {
    background: color-mix(in srgb, var(--surface-hover) 70%, transparent);
  }

  .preset-item:focus-visible {
    outline: none;
    border-color: var(--accent);
  }

  .preset-item.active {
    border-color: color-mix(in srgb, var(--accent) 44%, var(--border));
    background: color-mix(in srgb, var(--accent) 11%, transparent);
  }

  .preset-main {
    display: flex;
    flex-direction: column;
    gap: 2px;
    min-width: 0;
  }

  .preset-label {
    font-size: 13px;
    font-weight: 600;
    line-height: 1.2;
  }

  .preset-meta {
    font-size: 12px;
    line-height: 1.3;
  }

  .preset-description {
    margin-top: 2px;
    font-size: 12px;
    line-height: 1.35;
  }

  .check-slot {
    min-height: 20px;
    min-width: 20px;
    display: flex;
    align-items: center;
    justify-content: center;
    margin-top: 1px;
    flex-shrink: 0;
  }

  .check-slot :global(.check-icon) {
    color: var(--accent);
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
    color: var(--danger);
  }
  .btn-danger:hover {
    background: var(--danger-soft);
  }

  .msg {
    font-size: 13px;
    margin: 0;
    color: var(--accent);
  }
  .msg.error {
    color: var(--danger);
  }

  code {
    background: var(--surface);
    padding: 2px 6px;
    border-radius: 4px;
    font-size: 12px;
  }
</style>
