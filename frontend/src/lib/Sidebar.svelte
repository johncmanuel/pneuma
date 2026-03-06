<script lang="ts">
  import { createEventDispatcher } from "svelte"

  export let activeView: string = "library"

  const dispatch = createEventDispatcher()

  const navItems = [
    { id: "library",    label: "Library"    },
    { id: "downloads",  label: "Downloads"  },
    { id: "settings",   label: "Settings"   },
  ]
</script>

<nav>
  <div class="logo">pneuma</div>
  <ul>
    {#each navItems as item}
      <li>
        <button
          class:active={activeView === item.id}
          on:click={() => dispatch("navigate", item.id)}
        >
          {item.label}
        </button>
      </li>
    {/each}
  </ul>
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

  ul { list-style: none; padding: 0; }

  li button {
    display: block;
    width: 100%;
    text-align: left;
    padding: 8px 16px;
    border-radius: 0;
    color: var(--text-2);
    font-size: 13px;
    transition: background 0.1s, color 0.1s;
  }

  li button:hover { background: var(--surface-hover); color: var(--text-1); }
  li button.active { color: var(--text-1); font-weight: 600; }
</style>
