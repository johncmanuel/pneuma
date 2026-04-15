<script lang="ts">
  import { onMount } from "svelte";
  import SearchBar from "../components/SearchBar.svelte";

  interface Props {
    mobileView?: boolean;
  }

  let { mobileView = false }: Props = $props();
  let searchBar = $state<SearchBar | undefined>();

  onMount(() => {
    if (mobileView) {
      window.setTimeout(() => {
        searchBar?.focus();
      }, 0);
    }
  });
</script>

<section class="search-view" class:mobile={mobileView}>
  <header class="search-header">
    <h1>Search</h1>
    <p class="text-3">Find songs and albums quickly.</p>
  </header>

  <div class="search-input-wrap">
    <SearchBar bind:this={searchBar} />
  </div>
</section>

<style>
  .search-view {
    display: flex;
    flex-direction: column;
    gap: 16px;
    padding: 0;
  }

  .search-header h1 {
    margin: 0;
    font-size: 24px;
    font-weight: 700;
  }

  .search-header p {
    margin: 6px 0 0;
    font-size: 13px;
  }

  .search-input-wrap {
    max-width: 620px;
  }

  .search-input-wrap :global(.search-bar) {
    max-width: none;
  }

  .search-input-wrap :global(.search-results) {
    max-height: min(68vh, 560px);
  }

  .search-view.mobile {
    gap: 10px;
  }

  .search-view.mobile .search-header h1 {
    font-size: 21px;
  }

  .search-view.mobile .search-header p {
    margin-top: 4px;
    font-size: 12px;
  }

  .search-view.mobile .search-input-wrap {
    max-width: none;
  }

  .search-view.mobile .search-input-wrap :global(.search-results) {
    max-height: min(62vh, 500px);
  }
</style>
