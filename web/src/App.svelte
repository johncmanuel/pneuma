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
  import Favorites from "./views/Favorites.svelte";
  import Login from "./views/Login.svelte";
  import Register from "./views/Register.svelte";
  import { ChevronLeft, ChevronRight } from "@lucide/svelte";
  import { ThemeToggle, Toasts } from "@pneuma/ui";
  import { clamp as clampShared } from "@pneuma/shared";

  let wasLoggedIn = $state(false);
  let searchBar: any = $state(undefined);

  const SIDEBAR_MIN_WIDTH = 180;
  const SIDEBAR_MAX_WIDTH = 340;
  const SIDEBAR_DEFAULT_WIDTH = 200;

  const PANEL_MIN_WIDTH = 260;
  const PANEL_MAX_WIDTH = 420;
  const PANEL_DEFAULT_WIDTH = 320;

  const sidebarWidthKey = "pneuma:web:sidebar-width";
  const sidebarCollapsedKey = "pneuma:web:sidebar-collapsed";
  const panelWidthKey = "pneuma:web:panel-width";

  let sidebarWidth = $state(SIDEBAR_DEFAULT_WIDTH);
  let sidebarCollapsed = $state(false);
  let panelWidth = $state(PANEL_DEFAULT_WIDTH);
  let isResizingLayout = $state(false);

  let resizeMoveHandler: ((event: MouseEvent) => void) | null = $state(null);
  let resizeUpHandler: (() => void) | null = $state(null);

  function clearResizeListeners() {
    if (resizeMoveHandler) {
      window.removeEventListener("mousemove", resizeMoveHandler);
      resizeMoveHandler = null;
    }

    if (resizeUpHandler) {
      window.removeEventListener("mouseup", resizeUpHandler);
      resizeUpHandler = null;
    }
  }

  function beginHorizontalResize(
    event: MouseEvent,
    onDelta: (deltaX: number) => void
  ) {
    event.preventDefault();

    const startX = event.clientX;
    isResizingLayout = true;
    clearResizeListeners();

    resizeMoveHandler = (moveEvent: MouseEvent) => {
      onDelta(moveEvent.clientX - startX);
    };

    resizeUpHandler = () => {
      isResizingLayout = false;
      clearResizeListeners();
    };

    window.addEventListener("mousemove", resizeMoveHandler);
    window.addEventListener("mouseup", resizeUpHandler);
  }

  function startSidebarResize(event: MouseEvent) {
    const startWidth = sidebarWidth;

    beginHorizontalResize(event, (deltaX) => {
      sidebarCollapsed = false;
      sidebarWidth = clampShared(
        startWidth + deltaX,
        SIDEBAR_MIN_WIDTH,
        SIDEBAR_MAX_WIDTH
      );
    });
  }

  function startPanelResize(event: MouseEvent) {
    const startWidth = panelWidth;

    beginHorizontalResize(event, (deltaX) => {
      panelWidth = clampShared(
        startWidth - deltaX,
        PANEL_MIN_WIDTH,
        PANEL_MAX_WIDTH
      );
    });
  }

  function toggleSidebar() {
    sidebarCollapsed = !sidebarCollapsed;
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

  onMount(async () => {
    const rawSidebarWidth = Number(localStorage.getItem(sidebarWidthKey));
    const rawPanelWidth = Number(localStorage.getItem(panelWidthKey));

    sidebarWidth = Number.isFinite(rawSidebarWidth)
      ? clampShared(rawSidebarWidth, SIDEBAR_MIN_WIDTH, SIDEBAR_MAX_WIDTH)
      : SIDEBAR_DEFAULT_WIDTH;

    panelWidth = Number.isFinite(rawPanelWidth)
      ? clampShared(rawPanelWidth, PANEL_MIN_WIDTH, PANEL_MAX_WIDTH)
      : PANEL_DEFAULT_WIDTH;

    sidebarCollapsed = localStorage.getItem(sidebarCollapsedKey) === "1";

    await tryAutoAuth();
  });

  onDestroy(() => {
    clearResizeListeners();
    disconnectWS();
  });

  $effect(() => {
    localStorage.setItem(sidebarWidthKey, String(sidebarWidth));
    localStorage.setItem(panelWidthKey, String(panelWidth));
    localStorage.setItem(sidebarCollapsedKey, sidebarCollapsed ? "1" : "0");
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
    <div class="auth-theme">
      <ThemeToggle />
    </div>
    {#if $currentView === "register"}
      <Register onSwitch={() => pushNav({ view: "login" })} />
    {:else}
      <Login onSwitch={() => pushNav({ view: "register" })} />
    {/if}
  </div>
{:else}
  <div
    class="shell"
    class:panel-open={$activePanel !== null}
    class:sidebar-collapsed={sidebarCollapsed}
    class:is-resizing={isResizingLayout}
    style="--app-sidebar-w: {sidebarWidth}px; --app-panel-w: {panelWidth}px;"
  >
    <div class="sidebar-area">
      <Sidebar
        activeView={$currentView}
        collapsed={sidebarCollapsed}
        onToggleCollapse={toggleSidebar}
        onNavigate={handleNavigate}
      />
    </div>

    {#if !sidebarCollapsed}
      <button
        class="resize-handle sidebar-resize-handle"
        onmousedown={startSidebarResize}
        title="Resize sidebar"
      ></button>
    {/if}

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
      <div class="topbar-actions">
        <ThemeToggle />
        <button class="sign-out-btn" onclick={logout}>Sign out</button>
      </div>
    </header>

    <main class="content">
      {#if $currentView === "library"}
        <Library />
      {:else if $currentView === "favorites"}
        <Favorites />
      {:else if $currentView === "playlists"}
        <Playlists />
      {/if}
    </main>

    {#if $activePanel === "queue"}
      <div class="panel-area">
        <QueuePanel />
      </div>
      <button
        class="resize-handle panel-resize-handle"
        onmousedown={startPanelResize}
        title="Resize side panel"
      ></button>
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
    position: relative;
  }

  .auth-theme {
    position: absolute;
    top: 16px;
    right: 16px;
    z-index: 2;
  }

  .shell {
    --app-sidebar-w: var(--sidebar-w);
    --app-sidebar-collapsed-w: 64px;
    --app-panel-w: 320px;
    display: grid;
    grid-template-columns: var(--app-sidebar-w) 1fr;
    grid-template-rows: 48px 1fr auto;
    grid-template-areas:
      "sidebar topbar"
      "sidebar content"
      "player  player";
    height: 100vh;
    width: 100vw;
    position: relative;
  }

  .shell:not(.is-resizing) {
    transition: grid-template-columns 0.12s ease;
  }

  .shell.is-resizing {
    user-select: none;
  }

  .shell.sidebar-collapsed {
    grid-template-columns: var(--app-sidebar-collapsed-w) 1fr;
  }

  .shell.panel-open {
    grid-template-columns: var(--app-sidebar-w) 1fr var(--app-panel-w);
    grid-template-areas:
      "sidebar topbar  panel"
      "sidebar content panel"
      "player  player  player";
  }

  .shell.panel-open.sidebar-collapsed {
    grid-template-columns: var(--app-sidebar-collapsed-w) 1fr var(--app-panel-w);
  }

  .sidebar-area {
    grid-area: sidebar;
    display: flex;
    flex-direction: column;
    overflow: hidden;
    min-width: 0;
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

  .topbar-actions {
    display: flex;
    align-items: center;
    gap: 8px;
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

  .resize-handle {
    position: absolute;
    top: 0;
    bottom: var(--player-h);
    width: 10px;
    transform: translateX(-50%);
    background: transparent;
    border: none;
    padding: 0;
    cursor: col-resize;
    z-index: 30;
  }

  .resize-handle::after {
    content: "";
    position: absolute;
    top: 0;
    bottom: 0;
    left: 50%;
    width: 1px;
    background: var(--border);
    opacity: 0.75;
  }

  .resize-handle:hover::after,
  .shell.is-resizing .resize-handle::after {
    background: var(--accent);
    opacity: 1;
  }

  .sidebar-resize-handle {
    left: var(--app-sidebar-w);
  }

  .panel-resize-handle {
    left: calc(100% - var(--app-panel-w));
  }

  .player-wrapper {
    grid-area: player;
    display: flex;
    flex-direction: column;
  }
</style>
