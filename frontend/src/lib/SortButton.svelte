<script lang="ts" generics="T">
  import type { Snippet } from "svelte";

  let {
    currentField = $bindable(),
    sortDir = $bindable(),
    field,
    class: className = "",
    children
  }: {
    currentField: T;
    sortDir: "asc" | "desc";
    field: T;
    class?: string;
    children?: Snippet;
  } = $props();

  function toggle() {
    if (currentField === field) {
      sortDir = sortDir === "asc" ? "desc" : "asc";
    } else {
      currentField = field;
      sortDir = "asc";
    }
  }

  // i'll keep the character arrows for now
  let indicator = $derived(
    currentField === field ? (sortDir === "asc" ? " ↑" : " ↓") : ""
  );
</script>

<button class={className} onclick={toggle}>
  {#if children}{@render children()}{/if}{indicator}
</button>

<style>
  button {
    cursor: pointer;
    background: none;
    border: none;
    padding: 0;
    color: inherit;
    font: inherit;
    text-align: inherit;
    text-transform: inherit;
    letter-spacing: inherit;
  }
</style>
