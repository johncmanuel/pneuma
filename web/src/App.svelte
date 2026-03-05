<script lang="ts">
  import { onMount, onDestroy } from "svelte"
  import { loggedIn, tryAutoAuth } from "./lib/api"
  import { connectWS, disconnectWS } from "./lib/ws"
  import Login from "./lib/Login.svelte"
  import Sidebar from "./lib/Sidebar.svelte"
  import Library from "./pages/Library.svelte"
  import Admin from "./pages/Admin.svelte"
  import Player from "./lib/Player.svelte"

  let view = "library"
  let ready = false

  onMount(async () => {
    await tryAutoAuth()
    ready = true
    // Connect WebSocket once authenticated
    if ($loggedIn) connectWS()
  })

  // Reconnect WS when auth state changes
  $: if (ready && $loggedIn) connectWS()
  $: if (ready && !$loggedIn) disconnectWS()

  onDestroy(() => disconnectWS())

  function handleNavigate(e: CustomEvent<string>) {
    view = e.detail
  }
</script>

{#if !ready}
  <div class="loading-screen"><p>Connecting…</p></div>
{:else if $loggedIn}
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
  .loading-screen {
    height: 100vh;
    display: flex;
    align-items: center;
    justify-content: center;
    color: var(--text-3);
    font-size: 14px;
  }
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
