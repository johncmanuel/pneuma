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
    canGoForward,
    type DesktopView
  } from "./stores/ui";

  import Sidebar from "./lib/Sidebar.svelte";
  import Player from "./lib/Player.svelte";
  import Library from "./lib/Library.svelte";
  import Playlists from "./lib/Playlists.svelte";
  import Favorites from "./lib/Favorites.svelte";
  import SearchBar from "./lib/SearchBar.svelte";
  import Queue from "./lib/Queue.svelte";
  import Settings from "./lib/Settings.svelte";
  import DisconnectBanner from "./lib/DisconnectBanner.svelte";
  import { ChevronLeft, ChevronRight } from "@lucide/svelte";
  import { ThemeToggle, Toasts } from "@pneuma/ui";
  import { clamp as clampShared } from "@pneuma/shared";

  let wasConnected = $state(false);
  let searchBar: SearchBar | undefined = $state();

  const SIDEBAR_MIN_WIDTH = 180;
  const SIDEBAR_MAX_WIDTH = 340;
  const SIDEBAR_DEFAULT_WIDTH = 200;
  const PANEL_MIN_WIDTH = 260;
  const PANEL_MAX_WIDTH = 420;
  const PANEL_DEFAULT_WIDTH = 320;

  const sidebarWidthKey = "pneuma:desktop:sidebar-width";
  const sidebarCollapsedKey = "pneuma:desktop:sidebar-collapsed";
  const panelWidthKey = "pneuma:desktop:panel-width";

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

    await initApi();
    await initPlaylists();
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
    pushNav({
      view: view as DesktopView,
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
    <div class="topbar-spacer"></div>
    <ThemeToggle />
  </header>

  <main class="content">
    {#if $currentView === "library"}
      <Library />
    {:else if $currentView === "favorites"}
      <Favorites />
    {:else if $currentView === "playlists"}
      <Playlists />
    {:else if $currentView === "settings"}
      <Settings />
    {/if}
  </main>

  <div class="panel-area" class:open={$activePanel !== null}>
    {#if $activePanel === "queue"}
      <Queue />
    {/if}
  </div>
  <button
    class="resize-handle panel-resize-handle"
    class:hidden={$activePanel === null}
    onmousedown={startPanelResize}
    title="Resize side panel"
  ></button>

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
    font-family: var(--font);
    user-select: none;
    overflow: hidden;
  }

  .shell {
    --app-sidebar-w: var(--sidebar-w);
    --app-sidebar-collapsed-w: 64px;
    --app-panel-w: 320px;
    --app-panel-current-w: 0px;
    display: grid;
    grid-template-columns: var(--app-sidebar-w) 1fr var(--app-panel-current-w);
    grid-template-rows: 48px 1fr auto;
    grid-template-areas:
      "sidebar topbar panel"
      "sidebar content panel"
      "player  player player";
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
    grid-template-columns: var(--app-sidebar-collapsed-w) 1fr var(
        --app-panel-current-w
      );
  }

  .shell.panel-open {
    --app-panel-current-w: var(--app-panel-w);
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

  .content {
    grid-area: content;
    overflow-y: auto;
    padding: 24px;
  }

  .panel-area {
    grid-area: panel;
    overflow: hidden;
    min-width: 0;
    opacity: 0;
    transform: translateX(8px);
    transition:
      opacity 0.12s ease,
      transform 0.12s ease;
  }

  .panel-area.open {
    opacity: 1;
    transform: translateX(0);
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
    left: calc(100% - var(--app-panel-current-w));
    transition: opacity 0.12s ease;
  }

  .panel-resize-handle.hidden {
    opacity: 0;
    pointer-events: none;
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
