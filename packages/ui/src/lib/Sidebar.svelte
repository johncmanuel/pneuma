<script lang="ts">
  import {
    Heart,
    LayoutDashboard,
    ListMusic,
    Music,
    PanelLeftClose,
    PanelLeftOpen,
    Settings as SettingsIcon,
    SquareLibrary
  } from "@lucide/svelte";

  interface NavItem {
    id: string;
    label: string;
  }

  interface RecentItem {
    key: string;
    name: string;
    sub: string;
    artworkUrl?: string;
    artworkTrackId?: string;
    artworkPlaylistId?: string;
  }

  interface Props {
    activeView?: string;
    navItems?: NavItem[];
    recentItems?: RecentItem[];
    collapsed?: boolean;
    onToggleCollapse?: () => void;
    onNavigate?: (id: string) => void;
    onRecentClick?: (item: RecentItem) => void;
    onRecentArtError?: (item: RecentItem) => void;
  }

  let {
    activeView = "library",
    navItems = [],
    recentItems = [],
    collapsed = false,
    onToggleCollapse,
    onNavigate,
    onRecentClick,
    onRecentArtError
  }: Props = $props();

  function handleRecentArtError(e: Event, item: RecentItem) {
    (e.currentTarget as HTMLImageElement).style.display = "none";
    onRecentArtError?.(item);
  }

  const navIconById: Record<string, any> = {
    library: SquareLibrary,
    favorites: Heart,
    playlists: ListMusic,
    dashboard: LayoutDashboard,
    __dashboard: LayoutDashboard,
    settings: SettingsIcon
  };
</script>

<nav class:collapsed>
  <div class="head-row">
    {#if !collapsed}
      <div class="logo">pneuma</div>
    {/if}
    <button
      class="collapse-btn"
      onclick={() => onToggleCollapse?.()}
      title={collapsed ? "Show sidebar" : "Hide sidebar"}
      aria-label={collapsed ? "Show sidebar" : "Hide sidebar"}
    >
      {#if collapsed}
        <PanelLeftOpen size={18} />
      {:else}
        <PanelLeftClose size={16} />
      {/if}
    </button>
  </div>

  <ul>
    {#each navItems as item}
      <li>
        <button
          class:active={activeView === item.id}
          onclick={() => onNavigate?.(item.id)}
          title={collapsed ? item.label : undefined}
        >
          {#if navIconById[item.id]}
            {@const NavIcon = navIconById[item.id]}
            <span class="nav-icon"><NavIcon size={18} /></span>
          {/if}
          {#if !collapsed}
            <span class="nav-label">{item.label}</span>
          {/if}
        </button>
      </li>
    {/each}
  </ul>

  {#if recentItems.length > 0}
    {#if !collapsed}
      <div class="section-label">Recently Played</div>
    {/if}
    <div class="recent-list" class:collapsed-list={collapsed}>
      {#each recentItems as item (item.key)}
        <button
          class="recent-item"
          class:collapsed-item={collapsed}
          onclick={() => onRecentClick?.(item)}
          title={collapsed ? item.name : undefined}
        >
          <div class="recent-art">
            {#if item.artworkUrl}
              <img
                src={item.artworkUrl}
                alt=""
                onerror={(e) => handleRecentArtError(e, item)}
                loading="lazy"
              />
            {/if}
            <span class="recent-art-ph"><Music size={14} /></span>
          </div>
          {#if !collapsed}
            <div class="recent-info">
              <span class="recent-name truncate">{item.name}</span>
              <span class="recent-sub truncate text-3">{item.sub}</span>
            </div>
          {/if}
        </button>
      {/each}
    </div>
  {/if}

  <div class="spacer"></div>
</nav>

<style>
  nav {
    background: var(--surface);
    border-right: 1px solid var(--border);
    display: flex;
    flex-direction: column;
    flex: 1;
    overflow-y: auto;
    padding: 16px 0;
  }

  .head-row {
    display: flex;
    align-items: center;
    justify-content: space-between;
    gap: 14px;
    padding: 0 12px 16px;
  }

  nav.collapsed .head-row {
    justify-content: center;
    padding: 0 8px 16px;
  }

  .logo {
    font-size: 18px;
    font-weight: 700;
    color: var(--accent);
    letter-spacing: 2px;
  }

  .collapse-btn {
    display: inline-flex;
    align-items: center;
    justify-content: center;
    width: 28px;
    height: 28px;
    border-radius: 50%;
    background: var(--surface-2);
    color: var(--text-2);
    cursor: pointer;
    border: none;
    flex-shrink: 0;
  }

  .collapse-btn:hover {
    background: var(--surface-hover);
    color: var(--text-1);
  }

  ul {
    list-style: none;
    padding: 0;
    margin: 0;
  }

  li button {
    display: flex;
    align-items: center;
    gap: 8px;
    width: 100%;
    text-align: left;
    padding: 8px 16px;
    border-radius: 0;
    color: var(--text-2);
    font-size: 13px;
    transition:
      background 0.1s,
      color 0.1s;
  }

  nav.collapsed li button {
    justify-content: center;
    padding: 6px 0;
  }

  .nav-icon {
    display: inline-flex;
    align-items: center;
    justify-content: center;
    color: currentColor;
    flex-shrink: 0;
  }

  nav.collapsed .nav-icon {
    width: 28px;
    height: 28px;
  }

  .nav-label {
    min-width: 0;
  }

  li button:hover {
    background: var(--surface-hover);
    color: var(--text-1);
  }
  li button.active {
    color: var(--text-1);
    font-weight: 600;
  }

  .section-label {
    font-size: 10px;
    font-weight: 700;
    text-transform: uppercase;
    letter-spacing: 0.08em;
    color: var(--text-3);
    padding: 16px 16px 6px;
  }

  .recent-list {
    display: flex;
    flex-direction: column;
    gap: 1px;
  }

  .recent-list.collapsed-list {
    gap: 6px;
    align-items: center;
    padding: 0 6px;
  }

  .recent-item {
    display: flex;
    align-items: center;
    gap: 10px;
    width: 100%;
    text-align: left;
    padding: 6px 16px;
    transition: background 0.1s;
  }
  .recent-item:hover {
    background: var(--surface-hover);
  }

  .recent-item.collapsed-item {
    justify-content: center;
    padding: 4px;
  }

  .recent-art {
    width: 36px;
    height: 36px;
    border-radius: 4px;
    background: var(--surface-2);
    flex-shrink: 0;
    overflow: hidden;
    position: relative;
    display: flex;
    align-items: center;
    justify-content: center;
  }
  .recent-art img {
    position: absolute;
    width: 100%;
    height: 100%;
    object-fit: cover;
    z-index: 1;
  }
  .recent-art-ph {
    font-size: 14px;
    color: var(--text-3);
  }

  .recent-info {
    display: flex;
    flex-direction: column;
    min-width: 0;
    flex: 1;
    gap: 1px;
  }
  .recent-name {
    font-size: 12px;
    font-weight: 500;
  }
  .recent-sub {
    font-size: 11px;
  }

  .spacer {
    flex: 1;
  }
</style>
