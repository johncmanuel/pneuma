<script lang="ts">
  import { onMount, onDestroy } from "svelte";
  import { tryAutoAuth, loggedIn, logout } from "./lib/api";
  import { connectWS, disconnectWS } from "./lib/ws";
  import { loadAlbumGroupsPage } from "./lib/stores/library";
  import { loadPlaylists } from "./lib/stores/playlists";
  import { loadRecent } from "./lib/stores/recent";
  import { loadPlaybackState } from "./lib/stores/playback";
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
  import SearchBar from "./components/SearchBar.svelte";
  import Library from "./views/Library.svelte";
  import Playlists from "./views/Playlists.svelte";
  import Login from "./views/Login.svelte";
  import Register from "./views/Register.svelte";
  import { ChevronLeft, ChevronRight } from "@lucide/svelte";
  import { Toasts } from "@pneuma/ui";

  let wasLoggedIn = $state(false);
  let searchBar: any = $state(undefined);

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

  onMount(async () => {
    await tryAutoAuth();
  });

  onDestroy(() => {
    disconnectWS();
  });

  // Reactively connect/disconnect WS whenever auth state changes.
  $effect(() => {
    if ($loggedIn && !wasLoggedIn) {
      wasLoggedIn = true;
      connectWS();
      loadPlaybackState();
      loadAlbumGroupsPage(0);
      loadPlaylists();
      loadRecent();
    } else if (!$loggedIn && wasLoggedIn) {
      wasLoggedIn = false;
      disconnectWS();
    }
  });

  function handleNavigate(view: string) {
    if (view === "favorites") {
      view = "playlists";
    }

    pushNav({
      view: view,
      albumKey: null,
      playlistId: null
    });
  }
</script>

<svelte:window onkeydown={handleKeydown} onmouseup={handleMouseUp} />

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
      <div class="search-wrapper">
        <SearchBar bind:this={searchBar} />
      </div>
      <div class="topbar-spacer"></div>
      <button class="sign-out-btn" onclick={logout}>Sign out</button>
    </header>

    <main class="content">
      {#if $currentView === "library"}
        <Library />
      {:else if $currentView === "playlists"}
        <Playlists />
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

  .search-wrapper {
    position: relative;
    width: 100%;
    max-width: 420px;
  }

  .topbar-spacer {
    flex: 1;
  }

  .sign-out-btn {
    font-size: 13px;
    color: var(--text-3);
    padding: 6px 12px;
    border-radius: var(--r-sm);
    transition:
      background 0.1s,
      color 0.1s;
  }
  .sign-out-btn:hover {
    background: var(--surface-hover);
    color: var(--danger);
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
