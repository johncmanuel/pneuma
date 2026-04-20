<script lang="ts">
  import { onMount } from "svelte";
  import { apiFetch } from "../api";
  import { formatDate } from "../utils";

  interface AuditEntry {
    id: string;
    user_id: string;
    action: string;
    target_type: string;
    target_id: string;
    detail: string;
    created_at: string;
  }

  let entries: AuditEntry[] = $state([]);
  let loading = $state(false);

  onMount(loadAudit);

  async function loadAudit() {
    loading = true;
    try {
      const r = await apiFetch("/api/admin/audit");
      if (r.ok) entries = await r.json();
    } finally {
      loading = false;
    }
  }

  function actionColor(action: string): string {
    if (action.includes("delete") || action.includes("clear"))
      return "var(--danger)";
    if (action.includes("upload") || action.includes("create"))
      return "var(--accent)";
    return "var(--text-2)";
  }

  async function clearAudit() {
    if (!confirm("Are you sure you want to clear all audit logs?")) return;
    const r = await apiFetch("/api/admin/audit", { method: "DELETE" });
    if (r.ok) {
      entries = [];
      loadAudit();
    } else {
      alert("Failed to clear audit logs: " + (await r.text()));
    }
  }
</script>

<div class="panel">
  <div class="header">
    <h2>Audit Logs</h2>
    <button
      class="danger-btn"
      onclick={clearAudit}
      disabled={entries.length === 0}>Clear Logs</button
    >
  </div>
  {#if loading}
    <p class="text-3">Loading...</p>
  {:else if entries.length === 0}
    <p class="text-3">No audit entries.</p>
  {:else}
    <div class="table-wrap">
      <table>
        <thead>
          <tr>
            <th>Time</th>
            <th>Action</th>
            <th>Target</th>
            <th>Target ID</th>
            <th>User ID</th>
            <th>Detail</th>
          </tr>
        </thead>
        <tbody>
          {#each entries as entry (entry.id)}
            <tr>
              <td class="text-3 nowrap">{formatDate(entry.created_at)}</td>
              <td>
                <span
                  class="action-tag"
                  style="color: {actionColor(entry.action)}"
                  >{entry.action}</span
                >
              </td>
              <td class="text-2">{entry.target_type}</td>
              <td class="mono text-3">{entry.target_id?.slice(0, 8) ?? "–"}</td>
              <td class="mono text-3">{entry.user_id?.slice(0, 8) ?? "–"}</td>
              <td class="text-2 truncate">{entry.detail || "–"}</td>
            </tr>
          {/each}
        </tbody>
      </table>
    </div>
  {/if}
</div>

<style>
  .panel {
    display: flex;
    flex-direction: column;
    gap: 12px;
  }

  .header {
    display: flex;
    justify-content: space-between;
    align-items: center;
  }

  .header h2 {
    margin: 0;
    font-size: 16px;
  }

  .danger-btn {
    padding: 6px 14px;
    border-radius: var(--r-md);
    background: var(--surface-2);
    border: 1px solid var(--danger);
    color: var(--danger);
    font-size: 13px;
    cursor: pointer;
  }
  .danger-btn:hover:not(:disabled) {
    background: var(--danger-soft);
  }
  .danger-btn:disabled {
    opacity: 0.4;
    cursor: not-allowed;
  }

  .table-wrap {
    overflow-x: auto;
  }

  table {
    width: 100%;
    border-collapse: collapse;
    font-size: 13px;
  }

  th {
    text-align: left;
    padding: 8px 12px;
    font-size: 11px;
    text-transform: uppercase;
    letter-spacing: 0.05em;
    color: var(--text-3);
    border-bottom: 1px solid var(--border);
  }

  td {
    padding: 6px 12px;
    border-bottom: 1px solid var(--border);
  }

  tr:hover {
    background: var(--surface-hover);
  }

  .nowrap {
    white-space: nowrap;
  }
  .mono {
    font-family: monospace;
    font-size: 12px;
  }

  .action-tag {
    font-weight: 600;
    font-size: 12px;
  }
</style>
