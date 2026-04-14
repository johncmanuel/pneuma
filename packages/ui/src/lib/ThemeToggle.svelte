<script lang="ts">
  import { Monitor, Moon, Sun } from "@lucide/svelte";
  import { themeMode, setThemeMode, type ThemeMode } from "./theme";

  let { class: className = "" }: { class?: string } = $props();

  const options: {
    id: ThemeMode;
    label: string;
    Icon: typeof Sun;
  }[] = [
    { id: "light", label: "Light", Icon: Sun },
    { id: "dark", label: "Dark", Icon: Moon },
    { id: "system", label: "System", Icon: Monitor }
  ];
</script>

<div class={`theme-toggle ${className}`.trim()} role="group" aria-label="Theme">
  {#each options as option (option.id)}
    <button
      class="theme-btn"
      class:active={$themeMode === option.id}
      onclick={() => setThemeMode(option.id)}
      aria-label={`${option.label} theme`}
      title={`${option.label} theme`}
      type="button"
    >
      <option.Icon size={16} />
    </button>
  {/each}
</div>

<style>
  .theme-toggle {
    display: inline-flex;
    align-items: center;
    gap: 2px;
    background: var(--surface);
    border: 1px solid var(--border);
    border-radius: 999px;
    padding: 2px;
  }

  .theme-btn {
    width: 28px;
    height: 28px;
    display: inline-flex;
    align-items: center;
    justify-content: center;
    border-radius: 999px;
    color: var(--text-2);
    transition:
      background 0.12s,
      color 0.12s;
  }

  .theme-btn:hover {
    background: var(--surface-hover);
    color: var(--text-1);
  }

  .theme-btn.active {
    background: var(--surface-2);
    color: var(--accent);
  }

  .theme-btn:focus-visible {
    outline: 2px solid var(--accent);
    outline-offset: 1px;
  }
</style>
