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
    localFolders,
    addLocalFolder,
    removeLocalFolder,
    scanLocalFolders
  } from "../stores/localLibrary";
  import {
    streamQuality,
    type StreamPresetOption,
    streamPresetOptions,
    addToast,
    crossfadeConfig
  } from "@pneuma/shared";
  import { db } from "../utils/db";
  import { RotateCcw, Check, CircleAlert, X } from "@lucide/svelte";
  import { BrowserOpenURL } from "../../wailsjs/runtime";

  function handlePresetClick(option: StreamPresetOption) {
    streamQuality.set(option.value);
  }

  function toggleCrossfade() {
    crossfadeConfig.update((c) => ({ ...c, enabled: !c.enabled }));
  }

  function setCrossfadeDuration(e: Event) {
    const sec = Number((e.target as HTMLInputElement).value);
    crossfadeConfig.update((c) => ({ ...c, durationSec: sec }));
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

  async function handleAddFolder() {
    await addLocalFolder();
  }
</script>

<section>
  <h2>Settings</h2>

  <div class="group">
    <h3>Local Music Folders</h3>
    <p class="text-3">
      Add folders from your computer to scan for music files.
    </p>
    <div style="display: flex; gap: 8px;">
      <button onclick={handleAddFolder}>+ Add Folder</button>
      {#if $localFolders.length > 0}
        <button onclick={() => scanLocalFolders()} title="Rescan local folders"
          ><RotateCcw size={14} /> Rescan</button
        >
      {/if}
    </div>
    {#if $localFolders.length > 0}
      <div class="folder-chips">
        {#each $localFolders as dir}
          <span class="folder-chip">
            {dir.split("/").pop() || dir}
            <button
              class="chip-remove"
              onclick={() => removeLocalFolder(dir)}
              title="Remove folder"><X size={14} /></button
            >
          </span>
        {/each}
      </div>
    {/if}
  </div>

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
    <h3>Crossfade</h3>
    <p class="text-3">Smoothly blend between tracks at the end of each song.</p>

    <label class="toggle-row">
      <span>Enable crossfade</span>
      <input
        type="checkbox"
        id="crossfade-toggle"
        checked={$crossfadeConfig.enabled}
        onchange={toggleCrossfade}
      />
    </label>

    {#if $crossfadeConfig.enabled}
      <div class="slider-row">
        <label for="crossfade-duration" class="text-3">
          Duration: <strong>{$crossfadeConfig.durationSec} s</strong>
        </label>
        <input
          type="range"
          id="crossfade-duration"
          min="1"
          max="12"
          step="1"
          value={$crossfadeConfig.durationSec}
          oninput={setCrossfadeDuration}
          class="duration-slider"
          aria-label="Crossfade duration in seconds"
        />
        <div class="slider-ticks" aria-hidden="true">
          <span>1s</span><span>6s</span><span>12s</span>
        </div>
      </div>
    {/if}
  </div>

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

  .toggle-row {
    display: flex;
    align-items: center;
    justify-content: space-between;
    gap: 12px;
    cursor: pointer;
    max-width: 320px;
    font-size: 13px;
  }

  .slider-row {
    display: flex;
    flex-direction: column;
    gap: 6px;
    max-width: 320px;
  }

  .duration-slider {
    width: 100%;
    accent-color: var(--accent);
    height: 4px;
    padding: 0;
    margin: 0;
  }

  .slider-ticks {
    display: flex;
    justify-content: space-between;
    font-size: 11px;
    color: var(--text-3);
  }

  .folder-chips {
    display: flex;
    flex-wrap: wrap;
    gap: 6px;
  }

  .folder-chip {
    display: inline-flex;
    align-items: center;
    gap: 4px;
    padding: 3px 10px;
    background: var(--surface-2);
    border: 1px solid var(--border);
    border-radius: 999px;
    font-size: 12px;
    color: var(--text-2);
  }

  .chip-remove {
    font-size: 14px;
    color: var(--text-3);
    padding: 0 2px;
    line-height: 1;
  }
  .chip-remove:hover {
    color: var(--danger);
  }
</style>
