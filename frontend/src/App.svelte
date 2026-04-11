<script lang="ts">
  import { onMount, onDestroy } from "svelte";
  import { initApi, connected } from "./utils/api";
  import { connectWS, disconnectWS } from "./stores/ws";
  import { loadRemoteAlbumGroupsPage } from "./stores/library";
  import { initPlaylists } from "./stores/playlists";
  import {
    activePanel,
    currentView,
    pushNav,
    goBack,
    goForward,
    canGoBack,
    canGoForward
  } from "./stores/ui";

  import Sidebar from "./lib/Sidebar.svelte";
  import Player from "./lib/Player.svelte";
  import Library from "./lib/Library.svelte";
  import Playlists from "./lib/Playlists.svelte";
  import SearchBar from "./lib/SearchBar.svelte";
  import Queue from "./lib/Queue.svelte";
  import Settings from "./lib/Settings.svelte";
  import DisconnectBanner from "./lib/DisconnectBanner.svelte";
  import { ChevronLeft, ChevronRight } from "@lucide/svelte";
  import { Toasts } from "@pneuma/ui";

  let wasConnected = $state(false);
  let searchBar: SearchBar | undefined = $state();

  onMount(async () => {
    await initApi();
    await initPlaylists();
  });

  onDestroy(() => {
    disconnectWS();
  });

  // Reactively connect/disconnect WS whenever connected state changes.
  // This covers: initial connect, autoReconnect success, and manual disconnect.
  $effect(() => {
    if ($connected && !wasConnected) {
      wasConnected = true;
      connectWS();
      loadRemoteAlbumGroupsPage(0);
    } else if (!$connected && wasConnected) {
      wasConnected = false;
      disconnectWS();
    }
  });

  function handleNavigate(view: string) {
    if (view === "favorites") {
      view = "playlists";
    }

    pushNav({
      view: view,
      tab: "library",
      subTab: "albums",
      albumKey: null,
      playlistId: null
    });
  }

  function handleKeydown(e: KeyboardEvent) {
    if ((e.ctrlKey || e.metaKey) && e.key === "k") {
      e.preventDefault();
      searchBar?.focus();
    }
  }

  function handleMouseUp(e: MouseEvent) {
    if (e.button === 3) {
      e.preventDefault();
      goBack();
    } else if (e.button === 4) {
      e.preventDefault();
      goForward();
    }
  }
</script>

<svelte:window onkeydown={handleKeydown} onmouseup={handleMouseUp} />

<div class="shell" class:panel-open={$activePanel !== null}>
  <div class="sidebar-area">
    <Sidebar activeView={$currentView} onNavigate={handleNavigate} />
  </div>

  <header class="topbar">
    <div class="nav-history">
      <button
        class="nav-btn"
        onclick={goBack}
        disabled={!$canGoBack}
        title="Go back"
      >
        <ChevronLeft size={18} />
      </button>
      <button
        class="nav-btn"
        onclick={goForward}
        disabled={!$canGoForward}
        title="Go forward"
      >
        <ChevronRight size={18} />
      </button>
    </div>
    <div class="search-wrapper">
      <SearchBar bind:this={searchBar} />
    </div>
  </header>

  <main class="content">
    {#if $currentView === "library"}
      <Library />
    {:else if $currentView === "playlists"}
      <Playlists />
    {:else if $currentView === "settings"}
      <Settings />
    {/if}
  </main>

  {#if $activePanel === "queue"}
    <div class="panel-area">
      <Queue />
    </div>
  {/if}

  <div class="player-wrapper">
    <DisconnectBanner />
    <Player />
  </div>
  <Toasts />
</div>

<style>
  :global(*, *::before, *::after) {
    box-sizing: border-box;
  }
  :global(body) {
    margin: 0;
    background: var(--bg);
    color: var(--fg);
    font-family:
      system-ui,
      -apple-system,
      "Segoe UI",
      sans-serif;
    user-select: none;
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
    transition:
      background 0.12s,
      opacity 0.12s;
    padding: 0;
  }
  .nav-btn:hover:not(:disabled) {
    background: var(--surface-hover);
  }
  .nav-btn:disabled {
    opacity: 0.3;
    cursor: default;
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

  :global(.player-wrapper > .player) {
    height: var(--player-h);
  }
</style>
