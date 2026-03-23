<script lang="ts">
  import { onMount, onDestroy } from "svelte";
  import { tryAutoAuth, loggedIn } from "./lib/api";
  import { connectWS, disconnectWS } from "./lib/ws";
  import { loadAlbumGroupsPage } from "./lib/stores/library";
  import { loadPlaylists } from "./lib/stores/playlists";
  import {
    activePanel,
    currentView,
    pushNav,
    goBack,
    goForward,
    canGoBack,
    canGoForward
  } from "./lib/stores/ui";

  import Sidebar from "./components/Sidebar.svelte";
  import PlayerBar from "./components/PlayerBar.svelte";
  import QueuePanel from "./components/QueuePanel.svelte";
  import Toasts from "./components/Toasts.svelte";
  import Home from "./views/Home.svelte";
  import Library from "./views/Library.svelte";
  import Playlists from "./views/Playlists.svelte";
  import Search from "./views/Search.svelte";
  import Login from "./views/Login.svelte";
  import Register from "./views/Register.svelte";
  import { ChevronLeft, ChevronRight } from "@lucide/svelte";

  let wasLoggedIn = false;

  onMount(async () => {
    await tryAutoAuth();
  });

  onDestroy(() => {
    disconnectWS();
  });

  // Reactively connect/disconnect WS whenever auth state changes.
  $: if ($loggedIn && !wasLoggedIn) {
    wasLoggedIn = true;
    connectWS();
    loadAlbumGroupsPage(0);
    loadPlaylists();
  } else if (!$loggedIn && wasLoggedIn) {
    wasLoggedIn = false;
    disconnectWS();
  }

  function handleNavigate(view: string) {
    pushNav({
      view: view,
      albumKey: null,
      playlistId: null
    });
  }
</script>

{#if !$loggedIn}
  <div class="auth-shell">
    {#if $currentView === "register"}
      <Register onswitch={() => pushNav({ view: "login" })} />
    {:else}
      <Login onswitch={() => pushNav({ view: "register" })} />
    {/if}
  </div>
{:else}
  <div class="shell" class:panel-open={$activePanel !== null}>
    <div class="sidebar-area">
      <Sidebar activeView={$currentView} onnavigate={handleNavigate} />
    </div>

    <header class="topbar">
      <div class="nav-history">
        <button
          class="nav-btn"
          disabled={!$canGoBack}
          onclick={goBack}
          title="Go back"
        >
          <ChevronLeft size={18} />
        </button>
        <button
          class="nav-btn"
          disabled={!$canGoForward}
          onclick={goForward}
          title="Go forward"
        >
          <ChevronRight size={18} />
        </button>
      </div>
    </header>

    <main class="content">
      {#if $currentView === "home"}
        <Home onnavigate={handleNavigate} />
      {:else if $currentView === "library"}
        <Library />
      {:else if $currentView === "playlists"}
        <Playlists />
      {:else if $currentView === "search"}
        <Search />
      {/if}
    </main>

    {#if $activePanel === "queue"}
      <div class="panel-area">
        <QueuePanel />
      </div>
    {/if}

    <div class="player-wrapper">
      <PlayerBar />
    </div>
    <Toasts />
  </div>
{/if}

<style>
  .auth-shell {
    height: 100vh;
    display: flex;
    align-items: center;
    justify-content: center;
    background: var(--bg);
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
</style>
