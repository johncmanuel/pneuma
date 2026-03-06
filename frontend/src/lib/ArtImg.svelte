<script lang="ts">
  import { onMount } from "svelte"
  import { cachedArtUrl } from "../stores/artCache"

  /** Stable cache key (trackId for remote, file path for local). */
  export let cacheKey: string = ""
  /** Raw URL to fetch artwork from. */
  export let rawUrl: string = ""
  export let alt: string = ""
  export let width: string | undefined = undefined
  export let height: string | undefined = undefined
  export let style: string = ""

  let blobUrl: string | null = null
  let loading = true
  let failed = false

  // Re-run whenever cacheKey or rawUrl changes
  $: if (rawUrl) load(cacheKey, rawUrl)

  async function load(key: string, url: string) {
    if (!url) { loading = false; failed = true; return }
    loading = true
    failed = false
    blobUrl = null
    try {
      const resolved = await cachedArtUrl(key, url)
      // Guard against stale async results from a previous key
      if (key === cacheKey && url === rawUrl) {
        blobUrl = resolved
        loading = false
      }
    } catch {
      if (key === cacheKey && url === rawUrl) {
        failed = true
        loading = false
      }
    }
  }
</script>

{#if blobUrl && !failed}
  <img
    src={blobUrl}
    {alt}
    {width}
    {height}
    {style}
    on:error={() => { failed = true; blobUrl = null }}
  />
{/if}
<!-- Always render the placeholder slot; CSS visibility is controlled by the parent -->
<slot name="placeholder" {loading} {failed} />
