<script lang="ts">
  import { onMount, onDestroy } from "svelte";
  import { loggedIn, tryAutoAuth, logout } from "./lib/api";
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

    <header class="topbar">
      <div class="topbar-spacer"></div>
      <button class="sign-out-btn" on:click={logout}>Sign out</button>
    </header>

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
    grid-template-rows: 48px 1fr;
    grid-template-areas:
      "sidebar topbar"
      "sidebar content";
    height: 100vh;
    width: 100vw;
  }

  .sidebar-area {
    grid-area: sidebar;
    display: flex;
    flex-direction: column;
    overflow: hidden;
  }

  .topbar {
    grid-area: topbar;
    display: flex;
    align-items: center;
    gap: 12px;
    padding: 0 24px;
    background: var(--bg);
    border-bottom: 1px solid var(--border);
  }

  .topbar-spacer {
    flex: 1;
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
</style>
