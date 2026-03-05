<script lang="ts">
  import { onMount, onDestroy } from "svelte"
  import { initApi, connected } from "./lib/api"
  import { connectWS, disconnectWS } from "./stores/ws"
  import { loadTracks } from "./stores/library"
  import { activePanel } from "./stores/ui"

  import Sidebar from "./lib/Sidebar.svelte"
  import Player from "./lib/Player.svelte"
  import Library from "./lib/Library.svelte"
  import SearchBar from "./lib/SearchBar.svelte"
  import Queue from "./lib/Queue.svelte"
  import DevicesPanel from "./lib/DevicesPanel.svelte"
  import Toasts from "./lib/Toasts.svelte"
  import Downloads from "./lib/Downloads.svelte"
  import Settings from "./lib/Settings.svelte"
  import DisconnectBanner from "./lib/DisconnectBanner.svelte"

  let view = "library"
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
    view = e.detail
  }
</script>

<div class="shell" class:panel-open={$activePanel !== null}>
  <div class="sidebar-area">
    <Sidebar activeView={view} on:navigate={handleNavigate} />
  </div>

  <header class="topbar">
    <div class="search-wrapper">
      <SearchBar />
    </div>
  </header>

  <main class="content">
    {#if view === "library"}
      <Library />
    {:else if view === "downloads"}
      <Downloads />
    {:else if view === "settings"}
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
    padding: 0 24px;
    background: var(--bg);
    border-bottom: 1px solid var(--border);
  }

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
