<script lang="ts">
  import { onMount, onDestroy } from "svelte";
  import { loggedIn, tryAutoAuth } from "./lib/api";
  import { connectWS, disconnectWS } from "./lib/ws";
  import Login from "./lib/components/Login.svelte";
  import Sidebar from "./lib/components/Sidebar.svelte";
  import Admin from "./pages/Admin.svelte";

  let ready = false;

  onMount(async () => {
    await tryAutoAuth();
    ready = true;
    if ($loggedIn) connectWS();
  });

  $: if (ready && $loggedIn) connectWS();
  $: if (ready && !$loggedIn) disconnectWS();

  onDestroy(() => disconnectWS());
</script>

{#if !ready}
  <div class="loading-screen"><p>Connecting...</p></div>
{:else if $loggedIn}
  <div class="shell">
    <div class="sidebar-area">
      <Sidebar />
    </div>
    <main class="content">
      <Admin />
    </main>
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
    height: 100vh;
    width: 100vw;
  }

  .sidebar-area {
    display: flex;
    flex-direction: column;
    overflow: hidden;
  }

  .content {
    overflow-y: auto;
    padding: 24px;
  }
</style>
