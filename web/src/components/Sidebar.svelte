<script lang="ts">
  import { logout } from "../lib/api";

  let {
    activeView = "home",
    onnavigate
  }: { activeView?: string; onnavigate?: (id: string) => void } = $props();

  const navItems = [
    { id: "home", label: "Home" },
    { id: "library", label: "Library" },
    { id: "playlists", label: "Playlists" },
    { id: "search", label: "Search" }
  ];
</script>

<nav>
  <div class="logo">pneuma</div>
  <ul>
    {#each navItems as item}
      <li>
        <button
          class:active={activeView === item.id}
          onclick={() => onnavigate?.(item.id)}
        >
          {item.label}
        </button>
      </li>
    {/each}
  </ul>

  <div class="spacer"></div>

  <div class="bottom">
    <button class="logout-btn" onclick={logout}> Sign out </button>
  </div>
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

  .logo {
    font-size: 18px;
    font-weight: 700;
    color: var(--accent);
    letter-spacing: 2px;
    padding: 0 16px 20px;
  }

  ul {
    list-style: none;
    padding: 0;
    margin: 0;
  }

  li button {
    display: block;
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

  li button:hover {
    background: var(--surface-hover);
    color: var(--text-1);
  }
  li button.active {
    color: var(--text-1);
    font-weight: 600;
  }

  .spacer {
    flex: 1;
  }

  .bottom {
    padding: 16px;
    border-top: 1px solid var(--border);
  }

  .logout-btn {
    display: block;
    width: 100%;
    text-align: left;
    padding: 8px 16px;
    color: var(--text-3);
    font-size: 13px;
    border-radius: var(--r-sm);
    transition:
      background 0.1s,
      color 0.1s;
  }

  .logout-btn:hover {
    background: var(--surface-hover);
    color: var(--text-1);
  }
</style>
