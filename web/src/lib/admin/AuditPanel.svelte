<script lang="ts">
  import { onMount } from "svelte";
  import { apiFetch } from "../api";

  interface AuditEntry {
    id: string;
    user_id: string;
    action: string;
    target_type: string;
    target_id: string;
    detail: string;
    created_at: string;
  }

  let entries: AuditEntry[] = [];
  let loading = false;

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

  function formatDate(iso: string): string {
    try {
      const d = new Date(iso);
      return d.toLocaleString();
    } catch {
      return iso;
    }
  }

  function actionColor(action: string): string {
    if (action.includes("delete")) return "var(--danger)";
    if (action.includes("upload") || action.includes("create"))
      return "var(--accent)";
    return "var(--text-2)";
  }
</script>

<div class="panel">
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
