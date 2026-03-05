<script lang="ts">
  import { loggedIn } from "./lib/api"
  import Login from "./lib/Login.svelte"
  import Sidebar from "./lib/Sidebar.svelte"
  import Library from "./pages/Library.svelte"
  import Admin from "./pages/Admin.svelte"
  import Player from "./lib/Player.svelte"

  let view = "library"

  function handleNavigate(e: CustomEvent<string>) {
    view = e.detail
  }
</script>

{#if $loggedIn}
  <div class="shell">
    <div class="sidebar-area">
      <Sidebar activeView={view} on:navigate={handleNavigate} />
    </div>

    <main class="content">
      {#if view === "library"}
        <Library />
      {:else if view === "admin"}
        <Admin />
      {/if}
    </main>

    <Player />
  </div>
{:else}
  <Login />
{/if}

<style>
  .shell {
    display: grid;
    grid-template-columns: var(--sidebar-w) 1fr;
    grid-template-rows: 1fr var(--player-h);
    grid-template-areas:
      "sidebar content"
      "player  player";
    height: 100vh;
    width: 100vw;
  }

  .sidebar-area {
    grid-area: sidebar;
    display: flex;
    flex-direction: column;
    overflow: hidden;
  }

  .content {
    grid-area: content;
    overflow-y: auto;
    padding: 24px;
  }

  :global(.shell > .player) { grid-area: player; }
</style>
