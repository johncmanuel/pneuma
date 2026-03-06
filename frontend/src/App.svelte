<script lang="ts">
  import { onMount, onDestroy } from "svelte"
  import { initApi, connected } from "./lib/api"
  import { connectWS, disconnectWS } from "./stores/ws"
  import { loadTracks } from "./stores/library"
  import { activePanel, currentView, pushNav, goBack, goForward, canGoBack, canGoForward } from "./stores/ui"

  import Sidebar from "./lib/Sidebar.svelte"
  import Player from "./lib/Player.svelte"
  import Library from "./lib/Library.svelte"
  import SearchBar from "./lib/SearchBar.svelte"
  import Queue from "./lib/Queue.svelte"
  import DevicesPanel from "./lib/DevicesPanel.svelte"
  import Toasts from "./lib/Toasts.svelte"
  import Settings from "./lib/Settings.svelte"
  import DisconnectBanner from "./lib/DisconnectBanner.svelte"

  let wasConnected = false

  onMount(async () => {
    await initApi()
  })

  onDestroy(() => {
    disconnectWS()
  })

  // Reactively connect/disconnect WS whenever connected state changes.
  // This covers: initial connect, autoReconnect success, and manual disconnect.
  $: if ($connected && !wasConnected) {
    wasConnected = true
    connectWS()
    loadTracks()
  } else if (!$connected && wasConnected) {
    wasConnected = false
    disconnectWS()
  }

  function handleNavigate(e: CustomEvent<string>) {
    pushNav({ view: e.detail, tab: "library", subTab: "albums", albumKey: null })
  }
</script>

<div class="shell" class:panel-open={$activePanel !== null}>
  <div class="sidebar-area">
    <Sidebar activeView={$currentView} on:navigate={handleNavigate} />
  </div>

  <header class="topbar">
    <div class="nav-history">
      <button class="nav-btn" disabled={!$canGoBack} on:click={goBack} title="Go back">
        <svg viewBox="0 0 24 24" width="18" height="18" fill="none" stroke="currentColor" stroke-width="2.5" stroke-linecap="round" stroke-linejoin="round"><polyline points="15 18 9 12 15 6"/></svg>
      </button>
      <button class="nav-btn" disabled={!$canGoForward} on:click={goForward} title="Go forward">
        <svg viewBox="0 0 24 24" width="18" height="18" fill="none" stroke="currentColor" stroke-width="2.5" stroke-linecap="round" stroke-linejoin="round"><polyline points="9 6 15 12 9 18"/></svg>
      </button>
    </div>
    <div class="search-wrapper">
      <SearchBar />
    </div>
  </header>

  <main class="content">
    {#if $currentView === "library"}
      <Library />
    {:else if $currentView === "settings"}
      <Settings />
    {/if}
  </main>

  {#if $activePanel === 'queue'}
    <div class="panel-area">
      <Queue />
    </div>
  {:else if $activePanel === 'devices'}
    <div class="panel-area">
      <DevicesPanel />
    </div>
  {/if}

  <div class="player-wrapper">
    <DisconnectBanner />
    <Player />
  </div>
  <Toasts />
</div>

<style>
  :global(*, *::before, *::after) { box-sizing: border-box; }
  :global(body) {
    margin: 0;
    background: var(--bg);
    color: var(--fg);
    font-family: system-ui, -apple-system, "Segoe UI", sans-serif;
    overflow: hidden;
  }

  .shell {
    display: grid;
    grid-template-columns: var(--sidebar-w) 1fr;
    grid-template-rows: 48px 1fr auto;
    grid-template-areas:
      "sidebar topbar"
      "sidebar content"
      "player  player";
    height: 100vh;
    width: 100vw;
  }

  .shell.panel-open {
    grid-template-columns: var(--sidebar-w) 1fr 320px;
    grid-template-areas:
      "sidebar topbar  panel"
      "sidebar content panel"
      "player  player  player";
  }

  .sidebar-area {
    grid-area: sidebar;
    display: flex;
    flex-direction: column;
    overflow: hidden;
  }

  .topbar {
    grid-area: topbar;
    display: flex;
    align-items: center;
    gap: 12px;
    padding: 0 24px;
    background: var(--bg);
    border-bottom: 1px solid var(--border);
    /* Ensure the search-results dropdown renders above the content area */
    position: relative;
    z-index: 10;
  }

  .nav-history {
    display: flex;
    gap: 4px;
    flex-shrink: 0;
  }

  .nav-btn {
    display: flex;
    align-items: center;
    justify-content: center;
    width: 28px;
    height: 28px;
    border-radius: 50%;
    background: var(--surface);
    color: var(--text-1);
    border: none;
    cursor: pointer;
    transition: background 0.12s, opacity 0.12s;
    padding: 0;
  }
  .nav-btn:hover:not(:disabled) { background: var(--surface-hover); }
  .nav-btn:disabled { opacity: 0.3; cursor: default; }

  .search-wrapper {
    position: relative;
    width: 100%;
    max-width: 420px;
  }

  .content {
    grid-area: content;
    overflow-y: auto;
    padding: 24px;
  }

  .panel-area {
    grid-area: panel;
    overflow: hidden;
  }

  .player-wrapper {
    grid-area: player;
    display: flex;
    flex-direction: column;
  }

  :global(.player-wrapper > .player) { height: var(--player-h); }
</style>
