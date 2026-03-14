<script lang="ts" generics="T">
  export let currentField: T;
  export let sortDir: "asc" | "desc";
  export let field: T;

  let className = "";
  export { className as class };

  function toggle() {
    if (currentField === field) {
      sortDir = sortDir === "asc" ? "desc" : "asc";
    } else {
      currentField = field;
      sortDir = "asc";
    }
  }

  // i'll keep the character arrows for now
  $: indicator =
    currentField === field ? (sortDir === "asc" ? " ↑" : " ↓") : "";
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
