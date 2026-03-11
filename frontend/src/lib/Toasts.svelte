<script lang="ts">
  import { toasts, dismissToast, type Toast } from "../stores/toasts"

  const icons: Record<Toast["type"], string> = {
    info: "i",
    warning: "⚠",
    error: "✕",
    success: "✓",
  }
</script>

{#if $toasts.length > 0}
  <div class="toast-container" role="status" aria-live="polite">
    {#each $toasts as toast (toast.id)}
      <div class="toast toast-{toast.type}">
        <span class="toast-icon">{icons[toast.type]}</span>
        <span class="toast-msg">{toast.message}</span>
        <button class="toast-close" on:click={() => dismissToast(toast.id)} title="Dismiss">×</button>
      </div>
    {/each}
  </div>
{/if}

<style>
  .toast-container {
    position: fixed;
    bottom: calc(var(--player-h) + 16px);
    right: 20px;
    z-index: 9999;
    display: flex;
    flex-direction: column;
    gap: 8px;
    pointer-events: none;
  }

  .toast {
    display: flex;
    align-items: center;
    gap: 10px;
    padding: 10px 14px;
    border-radius: 8px;
    font-size: 13px;
    min-width: 260px;
    max-width: 420px;
    pointer-events: all;
    box-shadow: 0 4px 16px rgba(0,0,0,0.4);
    animation: slide-in 0.18s ease-out;
  }

  @keyframes slide-in {
    from { transform: translateX(24px); opacity: 0; }
    to   { transform: translateX(0);    opacity: 1; }
  }

  .toast-info    { background: var(--surface); border: 1px solid var(--border); color: var(--text-1); }
  .toast-success { background: #1a3a1a; border: 1px solid #2d6a2d; color: #6ddb6d; }
  .toast-warning { background: #3a2e0a; border: 1px solid #7a6010; color: #e0b030; }
  .toast-error   { background: #3a100a; border: 1px solid #7a2010; color: #e06030; }

  .toast-icon { font-size: 14px; flex-shrink: 0; }
  .toast-msg  { flex: 1; line-height: 1.4; }

  .toast-close {
    font-size: 16px;
    line-height: 1;
    color: currentColor;
    opacity: 0.6;
    padding: 0 2px;
    flex-shrink: 0;
  }
  .toast-close:hover { opacity: 1; }
</style>
