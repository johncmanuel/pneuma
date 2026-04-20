<script lang="ts">
  import { onMount, onDestroy } from "svelte";
  import { tryAutoAuth, loggedIn, logout } from "./lib/api";
  import { connectWS, disconnectWS } from "./lib/ws";
  import { loadAlbumGroupsPage } from "./lib/stores/library";
  import {
    loadPlaylists,
    ensureFavoritesPlaylist,
    favoritesPlaylistId
  } from "./lib/stores/playlists";
  import { loadRecent } from "./lib/stores/recent";
  import { loadPlaybackState } from "./lib/stores/playback";
  import {
    activePanel,
    closePanel,
    currentView,
    initialDataLoaded,
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
  import SearchView from "./views/Search.svelte";
  import Library from "./views/Library.svelte";
  import Playlists from "./views/Playlists.svelte";
  import Favorites from "./views/Favorites.svelte";
  import SettingsView from "./views/Settings.svelte";
  import Login from "./views/Login.svelte";
  import Register from "./views/Register.svelte";
  import {
    ChevronLeft,
    ChevronRight,
    Heart,
    House,
    ListMusic,
    LogOut,
    Menu,
    Search,
    X
  } from "@lucide/svelte";
  import { ThemeToggle, Toasts } from "@pneuma/ui";
  import { clamp as clampShared } from "@pneuma/shared";
  import {
    applyPWAUpdate,
    checkForPWAUpdate,
    pwaUpdateAvailable
  } from "./lib/pwa";

  let wasLoggedIn = $state(false);
  let searchBar: any = $state(undefined);

  const SIDEBAR_MIN_WIDTH = 180;
  const SIDEBAR_MAX_WIDTH = 340;
  const SIDEBAR_DEFAULT_WIDTH = 200;

  const PANEL_MIN_WIDTH = 260;
  const PANEL_MAX_WIDTH = 420;
  const PANEL_DEFAULT_WIDTH = 320;
  const MOBILE_BREAKPOINT = 980;

  const sidebarWidthKey = "pneuma:web:sidebar-width";
  const sidebarCollapsedKey = "pneuma:web:sidebar-collapsed";
  const panelWidthKey = "pneuma:web:panel-width";

  let sidebarWidth = $state(SIDEBAR_DEFAULT_WIDTH);
  let sidebarCollapsed = $state(false);
  let panelWidth = $state(PANEL_DEFAULT_WIDTH);
  let isResizingLayout = $state(false);
  let isMobileView = $state(false);
  let mobileSidebarOpen = $state(false);

  type MobileTab = "library" | "search" | "playlists" | "favorites";

  const MOBILE_TAB_MAP: Record<string, MobileTab> = {
    favorites: "favorites",
    playlists: "playlists",
    search: "search",
    library: "library"
  };

  let activeMobileTab = $derived(MOBILE_TAB_MAP[$currentView] ?? null);

  let resizeMoveHandler: ((event: MouseEvent) => void) | null = $state(null);
  let resizeUpHandler: (() => void) | null = $state(null);
  let mobileQuery: MediaQueryList | null = $state(null);
  let mobileQueryHandler: ((event: MediaQueryListEvent) => void) | null =
    $state(null);
  let updateOnlineHandler: (() => void) | null = $state(null);
  let updateVisibilityHandler: (() => void) | null = $state(null);
  let dismissedPWAUpdateNotice = $state(false);

  let showPWAUpdateNotice = $derived(
    $pwaUpdateAvailable && !dismissedPWAUpdateNotice
  );

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
    if (isMobileView) return;

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
    if (isMobileView) return;

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

  function closeMobileSidebar() {
    mobileSidebarOpen = false;
  }

  function toggleMobileSidebar() {
    mobileSidebarOpen = !mobileSidebarOpen;
  }

  let sidebarTouchStartX = 0;
  function handleSidebarTouchStart(e: TouchEvent) {
    if (isMobileView && mobileSidebarOpen) {
      sidebarTouchStartX = e.touches[0].clientX;
    }
  }

  function handleSidebarTouchEnd(e: TouchEvent) {
    if (isMobileView && mobileSidebarOpen) {
      const touchEndX = e.changedTouches[0].clientX;
      if (sidebarTouchStartX - touchEndX > 50) {
        closeMobileSidebar();
      }
    }
  }

  async function navigateMobileTab(tab: MobileTab) {
    closeMobileSidebar();
    closePanel();

    if (tab === "search") {
      pushNav({
        view: "search",
        albumKey: null,
        playlistId: null
      });
      return;
    }

    if (tab === "favorites") {
      const favoriteID =
        $favoritesPlaylistId ?? (await ensureFavoritesPlaylist());
      pushNav({
        view: "favorites",
        playlistId: favoriteID ?? null,
        albumKey: null
      });
      return;
    }

    pushNav({
      view: tab,
      albumKey: null,
      playlistId: null
    });
  }

  function handleKeydown(e: KeyboardEvent) {
    if (e.key === "Escape") {
      if (mobileSidebarOpen) {
        closeMobileSidebar();
        return;
      }

      if ($activePanel !== null) {
        closePanel();
      }

      return;
    }

    if ((e.ctrlKey || e.metaKey) && e.key === "k") {
      e.preventDefault();

      if (isMobileView) {
        closeMobileSidebar();
        closePanel();
        pushNav({
          view: "search",
          albumKey: null,
          playlistId: null
        });
        return;
      }

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

  function dismissPWAUpdateNotice() {
    dismissedPWAUpdateNotice = true;
  }

  function applyPendingPWAUpdate() {
    applyPWAUpdate();
  }

  async function refreshPWAUpdateState() {
    try {
      await checkForPWAUpdate();
    } catch {
      console.info("PWA: update check skipped");
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

    mobileQuery = window.matchMedia(`(max-width: ${MOBILE_BREAKPOINT}px)`);
    isMobileView = mobileQuery.matches;

    mobileQueryHandler = (event: MediaQueryListEvent) => {
      isMobileView = event.matches;

      if (!event.matches) {
        mobileSidebarOpen = false;
      }
    };

    mobileQuery.addEventListener("change", mobileQueryHandler);

    updateOnlineHandler = () => {
      refreshPWAUpdateState();
    };

    updateVisibilityHandler = () => {
      if (document.visibilityState === "visible") {
        refreshPWAUpdateState();
      }
    };

    window.addEventListener("online", updateOnlineHandler);
    document.addEventListener("visibilitychange", updateVisibilityHandler);

    await tryAutoAuth();
    refreshPWAUpdateState();
  });

  onDestroy(() => {
    clearResizeListeners();

    if (mobileQuery && mobileQueryHandler) {
      mobileQuery.removeEventListener("change", mobileQueryHandler);
    }

    if (updateOnlineHandler) {
      window.removeEventListener("online", updateOnlineHandler);
    }

    if (updateVisibilityHandler) {
      document.removeEventListener("visibilitychange", updateVisibilityHandler);
    }

    disconnectWS();
  });

  $effect(() => {
    localStorage.setItem(sidebarWidthKey, String(sidebarWidth));
    localStorage.setItem(panelWidthKey, String(panelWidth));
    localStorage.setItem(sidebarCollapsedKey, sidebarCollapsed ? "1" : "0");
  });

  $effect(() => {
    if (isMobileView && $activePanel !== null) {
      mobileSidebarOpen = false;
    }
  });

  $effect(() => {
    if (!$pwaUpdateAvailable) {
      dismissedPWAUpdateNotice = false;
    }
  });

  // Reactively connect/disconnect WS whenever auth state changes.
  $effect(() => {
    if ($loggedIn && !wasLoggedIn) {
      wasLoggedIn = true;
      connectWS();
      loadPlaybackState();
      if (!$initialDataLoaded) {
        initialDataLoaded.set(true);
        loadAlbumGroupsPage(0);
        loadPlaylists();
        loadRecent();
      }
    } else if (!$loggedIn && wasLoggedIn) {
      wasLoggedIn = false;
      disconnectWS();
      initialDataLoaded.set(false);
    }
  });

  function handleNavigate(view: string) {
    pushNav({
      view: view,
      albumKey: null,
      playlistId: null
    });

    closePanel();

    if (isMobileView) {
      closeMobileSidebar();
    }
  }

  function handleSidebarInteraction() {
    if (isMobileView) {
      closeMobileSidebar();
    }
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
    class:mobile={isMobileView}
    class:mobile-sidebar-open={mobileSidebarOpen}
    style="--app-sidebar-w: {sidebarWidth}px; --app-panel-w: {panelWidth}px;"
  >
    <button
      class="mobile-sidebar-backdrop"
      class:open={isMobileView && mobileSidebarOpen}
      onclick={closeMobileSidebar}
      aria-label="Close navigation"
    ></button>

    <button
      class="mobile-panel-backdrop"
      class:open={isMobileView && $activePanel !== null}
      onclick={closePanel}
      aria-label="Close side panel"
    ></button>

    <!-- svelte-ignore a11y_no_static_element_interactions -->
    <div
      class="sidebar-area"
      ontouchstart={handleSidebarTouchStart}
      ontouchend={handleSidebarTouchEnd}
    >
      <Sidebar
        activeView={$currentView}
        collapsed={isMobileView ? false : sidebarCollapsed}
        onToggleCollapse={isMobileView ? closeMobileSidebar : toggleSidebar}
        onNavigate={handleNavigate}
        onInteraction={handleSidebarInteraction}
      />
    </div>

    {#if !sidebarCollapsed && !isMobileView}
      <button
        class="resize-handle sidebar-resize-handle"
        onmousedown={startSidebarResize}
        title="Resize sidebar"
      ></button>
    {/if}

    <header class="topbar">
      <div class="topbar-left">
        {#if isMobileView}
          <button
            class="mobile-menu-btn"
            onclick={toggleMobileSidebar}
            title={mobileSidebarOpen ? "Close sidebar" : "Open sidebar"}
            aria-label={mobileSidebarOpen ? "Close sidebar" : "Open sidebar"}
          >
            {#if mobileSidebarOpen}
              <X size={18} />
            {:else}
              <Menu size={18} />
            {/if}
          </button>
        {/if}

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
      </div>

      <div class="search-wrapper">
        <SearchBar bind:this={searchBar} />
      </div>

      <div class="topbar-spacer"></div>

      <div class="topbar-actions">
        <ThemeToggle />
        <button class="sign-out-btn" onclick={logout}>Sign out</button>
        <button
          class="icon-sign-out-btn"
          onclick={logout}
          title="Sign out"
          aria-label="Sign out"
        >
          <LogOut size={16} />
        </button>
      </div>
    </header>

    <main class="content">
      {#if $currentView === "library"}
        <Library />
      {:else if $currentView === "favorites"}
        <Favorites />
      {:else if $currentView === "playlists"}
        <Playlists />
      {:else if $currentView === "search"}
        <SearchView mobileView={isMobileView} />
      {:else if $currentView === "settings"}
        <SettingsView />
      {/if}
    </main>

    <div class="panel-area" class:open={$activePanel !== null}>
      {#if $activePanel === "queue"}
        <QueuePanel />
      {/if}
    </div>
    {#if !isMobileView}
      <button
        class="resize-handle panel-resize-handle"
        class:hidden={$activePanel === null}
        onmousedown={startPanelResize}
        title="Resize side panel"
      ></button>
    {/if}

    <div class="player-wrapper">
      <PlayerBar mobileView={isMobileView} />
    </div>

    {#if isMobileView}
      <nav class="mobile-bottom-nav" aria-label="Primary">
        <button
          class="mobile-tab"
          class:active={activeMobileTab === "library"}
          onclick={() => navigateMobileTab("library")}
        >
          <House size={18} />
          <span>Library</span>
        </button>
        <button
          class="mobile-tab"
          class:active={activeMobileTab === "search"}
          onclick={() => navigateMobileTab("search")}
        >
          <Search size={18} />
          <span>Search</span>
        </button>
        <button
          class="mobile-tab"
          class:active={activeMobileTab === "playlists"}
          onclick={() => navigateMobileTab("playlists")}
        >
          <ListMusic size={18} />
          <span>Playlists</span>
        </button>
        <button
          class="mobile-tab"
          class:active={activeMobileTab === "favorites"}
          onclick={() => navigateMobileTab("favorites")}
        >
          <Heart size={18} />
          <span>Favorites</span>
        </button>
      </nav>
    {/if}

    <Toasts />
  </div>
{/if}

{#if showPWAUpdateNotice}
  <aside class="pwa-update-banner" role="status" aria-live="polite">
    <div class="pwa-update-text">
      <strong>Update available</strong>
      <span>Reload now to get the latest fixes.</span>
    </div>
    <div class="pwa-update-actions">
      <button class="pwa-update-btn primary" onclick={applyPendingPWAUpdate}
        >Update</button
      >
      <button class="pwa-update-btn" onclick={dismissPWAUpdateNotice}
        >Later</button
      >
    </div>
  </aside>
{/if}

<style>
  .auth-shell {
    height: 100dvh;
    display: flex;
    align-items: center;
    justify-content: center;
    background: var(--bg);
    position: relative;
    padding: 16px;
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
    --app-panel-current-w: 0px;
    display: grid;
    grid-template-columns: var(--app-sidebar-w) 1fr var(--app-panel-current-w);
    grid-template-rows: 48px 1fr auto;
    grid-template-areas:
      "sidebar topbar panel"
      "sidebar content panel"
      "player  player player";
    height: 100dvh;
    width: 100vw;
    position: relative;
    overflow: hidden;
    background: var(--bg);
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

  .topbar-left {
    display: flex;
    align-items: center;
    gap: 8px;
    flex-shrink: 0;
    min-width: 0;
  }

  .mobile-menu-btn,
  .icon-sign-out-btn,
  .mobile-sidebar-backdrop,
  .mobile-panel-backdrop,
  .mobile-bottom-nav {
    display: none;
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
    min-width: 0;
  }

  .topbar-spacer {
    flex: 1;
  }

  .topbar-actions {
    display: flex;
    align-items: center;
    gap: 8px;
  }

  .icon-sign-out-btn {
    width: 30px;
    height: 30px;
    border-radius: 50%;
    align-items: center;
    justify-content: center;
    color: var(--text-3);
    transition:
      background 0.12s,
      color 0.12s;
  }

  .icon-sign-out-btn:hover {
    background: var(--surface-hover);
    color: var(--danger);
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
    min-width: 0;
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

  .pwa-update-banner {
    position: fixed;
    right: 16px;
    bottom: calc(var(--player-h) + 16px);
    z-index: 180;
    display: flex;
    align-items: center;
    gap: 12px;
    width: min(420px, calc(100vw - 24px));
    border: 1px solid var(--border);
    border-radius: 12px;
    background: var(--surface);
    padding: 10px 12px;
    box-shadow: var(--shadow-pop);
  }

  .pwa-update-text {
    min-width: 0;
    display: flex;
    flex-direction: column;
    gap: 2px;
  }

  .pwa-update-text strong {
    font-size: 13px;
    font-weight: 700;
    line-height: 1.3;
  }

  .pwa-update-text span {
    font-size: 12px;
    color: var(--text-2);
    line-height: 1.35;
  }

  .pwa-update-actions {
    flex-shrink: 0;
    display: flex;
    align-items: center;
    gap: 6px;
  }

  .pwa-update-btn {
    min-height: 30px;
    border-radius: 999px;
    border: 1px solid var(--border);
    padding: 0 12px;
    font-size: 12px;
    font-weight: 600;
    color: var(--text-1);
    background: var(--surface-2);
  }

  .pwa-update-btn:hover {
    background: var(--surface-hover);
  }

  .pwa-update-btn.primary {
    background: var(--accent);
    border-color: transparent;
    color: var(--on-accent);
  }

  .pwa-update-btn.primary:hover {
    filter: brightness(1.03);
  }

  @media (max-width: 980px) {
    .auth-shell {
      align-items: flex-start;
      justify-content: center;
      padding-top: max(16px, calc(env(safe-area-inset-top) + 12px));
    }

    .shell,
    .shell.sidebar-collapsed,
    .shell.panel-open {
      grid-template-columns: 1fr;
      grid-template-rows: 56px 1fr auto auto;
      grid-template-areas:
        "topbar"
        "content"
        "player"
        "mobile-nav";
      --app-panel-current-w: 0px;
    }

    .sidebar-area {
      position: fixed;
      top: 0;
      left: 0;
      bottom: 0;
      width: min(82vw, 320px);
      z-index: 80;
      transform: translateX(-104%);
      transition: transform 0.2s ease;
      box-shadow: var(--shadow-pop);
    }

    .shell.mobile-sidebar-open .sidebar-area {
      transform: translateX(0);
    }

    .mobile-sidebar-backdrop {
      display: block;
      position: fixed;
      inset: 0;
      z-index: 70;
      background: var(--overlay-strong);
      opacity: 0;
      pointer-events: none;
      transition: opacity 0.15s ease;
    }

    .mobile-sidebar-backdrop.open {
      opacity: 1;
      pointer-events: auto;
    }

    .mobile-panel-backdrop {
      position: fixed;
      inset: 0;
      z-index: 72;
      background: var(--overlay-strong);
      opacity: 0;
      pointer-events: none;
      transition: opacity 0.15s ease;
    }

    .mobile-panel-backdrop.open {
      opacity: 1;
      pointer-events: auto;
    }

    .topbar {
      gap: 8px;
      padding: 0 10px;
      z-index: 60;
    }

    .mobile-menu-btn {
      display: inline-flex;
      width: 32px;
      height: 32px;
      border-radius: 50%;
      align-items: center;
      justify-content: center;
      background: var(--surface);
      color: var(--text-1);
    }

    .topbar-left .nav-history {
      display: none;
    }

    .search-wrapper {
      max-width: none;
    }

    .topbar-spacer {
      display: none;
    }

    .topbar-actions {
      gap: 4px;
    }

    .sign-out-btn {
      display: none;
    }

    .icon-sign-out-btn {
      display: inline-flex;
    }

    .content {
      padding: 12px;
      padding-bottom: 8px;
    }

    .panel-area {
      position: fixed;
      top: 64px;
      left: 8px;
      right: 8px;
      bottom: 8px;
      border: 1px solid var(--border);
      border-radius: var(--r-lg);
      background: var(--surface);
      box-shadow: var(--shadow-pop);
      z-index: 75;
      opacity: 0;
      pointer-events: none;
      transform: translateY(12px);
    }

    .panel-area.open {
      opacity: 1;
      pointer-events: auto;
      transform: translateY(0);
    }

    .resize-handle {
      display: none;
    }

    .player-wrapper {
      position: relative;
      z-index: 65;
      padding: 6px 10px 4px;
      background: var(--bg);
    }

    .pwa-update-banner {
      left: 10px;
      right: 10px;
      bottom: calc(env(safe-area-inset-bottom) + 84px);
      width: auto;
      max-width: none;
      flex-direction: column;
      align-items: stretch;
      gap: 10px;
      padding: 10px;
    }

    .pwa-update-actions {
      width: 100%;
      gap: 8px;
    }

    .pwa-update-btn {
      flex: 1;
      min-height: 36px;
      text-align: center;
    }

    .mobile-bottom-nav {
      grid-area: mobile-nav;
      display: grid;
      grid-template-columns: repeat(4, minmax(0, 1fr));
      gap: 4px;
      align-items: stretch;
      padding: 4px 10px max(8px, env(safe-area-inset-bottom));
      border-top: 1px solid var(--border);
      background: var(--surface);
      z-index: 66;
    }

    .mobile-tab {
      display: flex;
      flex-direction: column;
      align-items: center;
      justify-content: center;
      gap: 3px;
      min-height: 52px;
      border-radius: 10px;
      color: var(--text-3);
      transition:
        color 0.12s,
        background 0.12s;
    }

    .mobile-tab span {
      font-size: 11px;
      font-weight: 600;
      line-height: 1;
    }

    .mobile-tab.active {
      color: var(--text-1);
      background: var(--surface-hover);
    }

    .topbar .search-wrapper {
      visibility: hidden;
      pointer-events: none;
      width: 0;
      min-width: 0;
      max-width: 0;
      margin: 0;
      padding: 0;
      overflow: hidden;
    }

    .topbar .topbar-spacer {
      display: block;
      flex: 1;
    }
  }

  @media (max-width: 560px) {
    .mobile-tab span {
      font-size: 10px;
    }

    .mobile-bottom-nav {
      padding-left: 8px;
      padding-right: 8px;
      gap: 2px;
    }

    .player-wrapper {
      padding-left: 8px;
      padding-right: 8px;
    }

    .pwa-update-banner {
      left: 8px;
      right: 8px;
    }
  }
</style>
