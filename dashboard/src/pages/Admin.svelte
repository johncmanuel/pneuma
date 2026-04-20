<script lang="ts">
  import { currentUser } from "../lib/api";
  import TracksPanel from "../lib/admin/TracksPanel.svelte";
  import UsersPanel from "../lib/admin/UsersPanel.svelte";
  import AuditPanel from "../lib/admin/AuditPanel.svelte";
  import DiskUsagePanel from "../lib/admin/DiskUsagePanel.svelte";

  type Tab = "tracks" | "users" | "audit" | "disk";

  let isAdmin = $derived($currentUser?.is_admin ?? false);
  let hasAnyPerm = $derived(
    isAdmin ||
      ($currentUser?.can_upload ?? false) ||
      ($currentUser?.can_edit ?? false) ||
      ($currentUser?.can_delete ?? false)
  );

  let activeTab: Tab = $state("tracks");

  let availableTabs = $derived(buildTabs($currentUser));

  function buildTabs(user: typeof $currentUser): { id: Tab; label: string }[] {
    const tabs: { id: Tab; label: string }[] = [];
    if (
      user &&
      (user.is_admin || user.can_upload || user.can_edit || user.can_delete)
    ) {
      tabs.push({ id: "tracks", label: "Tracks" });
    }
    if (user?.is_admin) {
      tabs.push({ id: "users", label: "Users" });
      tabs.push({ id: "audit", label: "Audit Log" });
      tabs.push({ id: "disk", label: "Disk Usage" });
    }
    return tabs;
  }

  // Reset to first available tab when permissions change
  $effect(() => {
    if (
      availableTabs.length > 0 &&
      !availableTabs.find((t) => t.id === activeTab)
    ) {
      activeTab = availableTabs[0].id;
    }
  });
</script>

<section>
  <div class="toolbar">
    <h2>Admin</h2>
  </div>

  {#if !hasAnyPerm}
    <p class="text-3">You don't have permission to access admin features.</p>
  {:else}
    <div class="tabs">
      {#each availableTabs as tab (tab.id)}
        <button
          class="tab-btn"
          class:active={activeTab === tab.id}
          onclick={() => {
            activeTab = tab.id;
          }}
        >
          {tab.label}
        </button>
      {/each}
    </div>

    <div class="tab-content">
      {#if activeTab === "tracks"}
        <TracksPanel />
      {:else if activeTab === "users"}
        <UsersPanel />
      {:else if activeTab === "audit"}
        <AuditPanel />
      {:else if activeTab === "disk"}
        <DiskUsagePanel />
      {/if}
    </div>
  {/if}
</section>

<style>
  section {
    height: 100%;
    display: flex;
    flex-direction: column;
    overflow: hidden;
  }

  .toolbar {
    display: flex;
    align-items: center;
    margin-bottom: 16px;
    flex-shrink: 0;
  }

  h2 {
    margin: 0;
    font-size: 20px;
    font-weight: 700;
  }

  .tabs {
    display: flex;
    gap: 0;
    border-bottom: 1px solid var(--border);
    margin-bottom: 16px;
    flex-shrink: 0;
  }

  .tab-btn {
    padding: 8px 20px;
    font-size: 14px;
    color: var(--text-2);
    border-bottom: 2px solid transparent;
    transition:
      color 0.12s,
      border-color 0.12s;
  }
  .tab-btn:hover {
    color: var(--text-1);
  }
  .tab-btn.active {
    color: var(--accent);
    border-bottom-color: var(--accent);
  }

  .tab-content {
    flex: 1;
    overflow-y: auto;
  }
</style>
