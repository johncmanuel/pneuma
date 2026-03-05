<script lang="ts">
  import { createEventDispatcher } from "svelte"
  import { currentUser, logout } from "./api"

  export let activeView: string = "library"

  const dispatch = createEventDispatcher<{ navigate: string }>()

  interface NavItem {
    id: string
    label: string
    icon: string
    /** If set, only show when this returns true */
    show?: () => boolean
  }

  $: items = buildNavItems($currentUser)

  function buildNavItems(user: typeof $currentUser): NavItem[] {
    const out: NavItem[] = [
      { id: "library", label: "Library", icon: "🎵" },
    ]
    // Admin dashboard visible to admins or users with any permission
    if (user && (user.is_admin || user.can_upload || user.can_edit || user.can_delete)) {
      out.push({ id: "admin", label: "Admin", icon: "⚙" })
    }
    return out
  }
</script>

<nav class="sidebar">
  <div class="brand">
    <span class="brand-icon">♫</span>
    <span class="brand-text">Pneuma</span>
  </div>

  <div class="nav-items">
    {#each items as item (item.id)}
      <button
        class="nav-btn"
        class:active={activeView === item.id}
        on:click={() => dispatch("navigate", item.id)}
      >
        <span class="nav-icon">{item.icon}</span>
        <span class="nav-label">{item.label}</span>
      </button>
    {/each}
  </div>

  <div class="sidebar-footer">
    {#if $currentUser}
      <div class="user-info truncate">
        <span class="text-2">{$currentUser.username}</span>
        {#if $currentUser.is_admin}
          <span class="badge">admin</span>
        {/if}
      </div>
    {/if}
    <button class="nav-btn logout-btn" on:click={logout}>
      <span class="nav-icon">🚪</span>
      <span class="nav-label">Sign Out</span>
    </button>
  </div>
</nav>

<style>
  .sidebar {
    width: var(--sidebar-w);
    height: 100%;
    background: var(--surface);
    border-right: 1px solid var(--border);
    display: flex;
    flex-direction: column;
    padding: 16px 0;
    overflow-y: auto;
  }

  .brand {
    display: flex;
    align-items: center;
    gap: 8px;
    padding: 0 20px 20px;
    font-weight: 700;
    font-size: 18px;
  }
  .brand-icon { color: var(--accent); font-size: 22px; }

  .nav-items {
    flex: 1;
    display: flex;
    flex-direction: column;
    gap: 2px;
    padding: 0 8px;
  }

  .nav-btn {
    display: flex;
    align-items: center;
    gap: 10px;
    padding: 8px 12px;
    border-radius: var(--r-md);
    font-size: 14px;
    color: var(--text-2);
    text-align: left;
    width: 100%;
    transition: background 0.12s, color 0.12s;
  }
  .nav-btn:hover { background: var(--surface-hover); color: var(--text-1); }
  .nav-btn.active { background: var(--surface-hover); color: var(--accent); }

  .nav-icon { font-size: 16px; flex-shrink: 0; }
  .nav-label { flex: 1; }

  .sidebar-footer {
    padding: 12px 8px 0;
    border-top: 1px solid var(--border);
    margin-top: auto;
  }

  .user-info {
    padding: 0 12px 8px;
    font-size: 13px;
    display: flex;
    align-items: center;
    gap: 6px;
  }

  .badge {
    font-size: 10px;
    text-transform: uppercase;
    letter-spacing: 0.05em;
    background: var(--accent-dim);
    color: #000;
    padding: 1px 6px;
    border-radius: 999px;
    font-weight: 600;
  }

  .logout-btn { color: var(--text-3); }
  .logout-btn:hover { color: var(--danger); }
</style>
