<script lang="ts">
  import { onMount } from "svelte"
  import { apiFetch } from "../api"

  interface User {
    id: string
    username: string
    is_admin: boolean
    can_upload: boolean
    can_edit: boolean
    can_delete: boolean
    created_at: string
  }

  let users: User[] = []
  let loading = false

  onMount(loadUsers)

  async function loadUsers() {
    loading = true
    try {
      const r = await apiFetch("/api/admin/users")
      if (r.ok) users = await r.json()
    } finally {
      loading = false
    }
  }

  async function updatePerms(user: User) {
    await apiFetch(`/api/admin/users/${user.id}/permissions`, {
      method: "PUT",
      body: JSON.stringify({
        can_upload: user.can_upload,
        can_edit: user.can_edit,
        can_delete: user.can_delete,
      }),
    })
  }

  async function deleteUser(id: string, username: string) {
    if (!confirm(`Delete user "${username}"? This cannot be undone.`)) return
    const r = await apiFetch(`/api/admin/users/${id}`, { method: "DELETE" })
    if (r.ok) await loadUsers()
  }

  function formatDate(iso: string): string {
    try {
      return new Date(iso).toLocaleDateString()
    } catch {
      return iso
    }
  }
</script>

<div class="panel">
  {#if loading}
    <p class="text-3">Loading…</p>
  {:else if users.length === 0}
    <p class="text-3">No users found.</p>
  {:else}
    <div class="table-wrap">
      <table>
        <thead>
          <tr>
            <th>Username</th>
            <th>Role</th>
            <th>Upload</th>
            <th>Edit</th>
            <th>Delete</th>
            <th>Joined</th>
            <th>Actions</th>
          </tr>
        </thead>
        <tbody>
          {#each users as user (user.id)}
            <tr>
              <td>{user.username}</td>
              <td>
                {#if user.is_admin}
                  <span class="badge admin">Admin</span>
                {:else}
                  <span class="badge user">User</span>
                {/if}
              </td>
              <td>
                <input
                  type="checkbox"
                  bind:checked={user.can_upload}
                  on:change={() => updatePerms(user)}
                  disabled={user.is_admin}
                />
              </td>
              <td>
                <input
                  type="checkbox"
                  bind:checked={user.can_edit}
                  on:change={() => updatePerms(user)}
                  disabled={user.is_admin}
                />
              </td>
              <td>
                <input
                  type="checkbox"
                  bind:checked={user.can_delete}
                  on:change={() => updatePerms(user)}
                  disabled={user.is_admin}
                />
              </td>
              <td class="text-3">{formatDate(user.created_at)}</td>
              <td>
                {#if !user.is_admin}
                  <button class="sm-btn danger" on:click={() => deleteUser(user.id, user.username)}>Delete</button>
                {/if}
              </td>
            </tr>
          {/each}
        </tbody>
      </table>
    </div>
  {/if}
</div>

<style>
  .panel { display: flex; flex-direction: column; gap: 12px; }

  .table-wrap { overflow-x: auto; }

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

  tr:hover { background: var(--surface-hover); }

  input[type="checkbox"] {
    width: 16px;
    height: 16px;
    accent-color: var(--accent);
    cursor: pointer;
  }

  .badge {
    font-size: 10px;
    text-transform: uppercase;
    letter-spacing: 0.05em;
    padding: 2px 8px;
    border-radius: 999px;
    font-weight: 600;
  }
  .badge.admin { background: var(--accent-dim); color: #000; }
  .badge.user { background: var(--surface-2); color: var(--text-2); }

  .sm-btn {
    padding: 3px 10px;
    border-radius: var(--r-sm);
    font-size: 12px;
    background: var(--surface-2);
    border: 1px solid var(--border);
  }
  .sm-btn.danger { color: var(--danger); }
  .sm-btn.danger:hover { background: rgba(248, 113, 113, 0.1); }
</style>
