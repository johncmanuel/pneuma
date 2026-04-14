<script lang="ts">
  import { toasts, dismissToast, type Toast } from "@pneuma/shared";
  import { Info, TriangleAlert, CircleX, Check, X } from "@lucide/svelte";

  const icons = {
    info: Info,
    warning: TriangleAlert,
    error: CircleX,
    success: Check
  } satisfies Record<Toast["type"], typeof Info>;
</script>

{#if $toasts.length > 0}
  <div class="toast-container" role="status" aria-live="polite">
    {#each $toasts as toast (toast.id)}
      {@const ToastIcon = icons[toast.type]}
      <div class="toast toast-{toast.type}">
        <span class="toast-icon">
          <ToastIcon size={16} />
        </span>
        <span class="toast-msg">{toast.message}</span>
        <button
          class="toast-close"
          onclick={() => dismissToast(toast.id)}
          title="Dismiss"><X size={14} /></button
        >
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
    box-shadow: var(--shadow-toast);
    animation: slide-in 0.18s ease-out;
  }

  @keyframes slide-in {
    from {
      transform: translateX(24px);
      opacity: 0;
    }
    to {
      transform: translateX(0);
      opacity: 1;
    }
  }

  .toast-info {
    background: var(--toast-info-bg);
    border: 1px solid var(--toast-info-border);
    color: var(--toast-info-text);
  }
  .toast-success {
    background: var(--toast-success-bg);
    border: 1px solid var(--toast-success-border);
    color: var(--toast-success-text);
  }
  .toast-warning {
    background: var(--toast-warning-bg);
    border: 1px solid var(--toast-warning-border);
    color: var(--toast-warning-text);
  }
  .toast-error {
    background: var(--toast-error-bg);
    border: 1px solid var(--toast-error-border);
    color: var(--toast-error-text);
  }

  .toast-icon {
    font-size: 14px;
    flex-shrink: 0;
  }
  .toast-msg {
    flex: 1;
    line-height: 1.4;
  }

  .toast-close {
    font-size: 16px;
    line-height: 1;
    color: currentColor;
    opacity: 0.6;
    padding: 0 2px;
    flex-shrink: 0;
  }
  .toast-close:hover {
    opacity: 1;
  }
</style>
