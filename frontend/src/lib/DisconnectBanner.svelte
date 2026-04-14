<script lang="ts">
  import { serverDisconnected } from "../stores/ws";
  import { TriangleAlert } from "@lucide/svelte";
</script>

{#if $serverDisconnected}
  <div class="disconnect-banner" role="alert">
    <span class="disconnect-icon"><TriangleAlert size={16} /></span>
    <span>Lost connection to server. Reconnecting...</span>
  </div>
{/if}

<style>
  .disconnect-banner {
    width: 100%;
    display: flex;
    align-items: center;
    justify-content: center;
    gap: 8px;
    padding: 6px 16px;
    background: var(--toast-error-bg);
    border-top: 1px solid var(--toast-error-border);
    color: var(--toast-error-text);
    font-size: 13px;
    font-weight: 500;
    z-index: 100;
    animation: slide-down 0.2s ease-out;
  }

  @keyframes slide-down {
    from {
      transform: translateY(-100%);
      opacity: 0;
    }
    to {
      transform: translateY(0);
      opacity: 1;
    }
  }

  .disconnect-icon {
    font-size: 14px;
    flex-shrink: 0;
  }
</style>
