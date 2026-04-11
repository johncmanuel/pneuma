<script lang="ts" generics="T">
  let {
    currentField = $bindable(),
    sortDir = $bindable(),
    field,
    class: className = ""
  }: {
    currentField: T;
    sortDir: "asc" | "desc";
    field: T;
    class?: string;
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
  <slot />{indicator}
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
