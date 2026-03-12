<script lang="ts">
  import { albums, loading } from "../stores/library";
  import { apiBase } from "../utils/api";
  import { Music } from "@lucide/svelte";

  function artUrl(artworkId: string | null): string {
    if (!artworkId) return "";
    const base = apiBase();
    if (!base) return "";
    return `${base}/api/library/artwork/${artworkId}`;
  }
</script>

<section>
  <h2>Albums</h2>

  {#if $loading}
    <p class="text-3">Loading...</p>
  {:else if $albums.length === 0}
    <p class="text-3">No albums found yet.</p>
  {:else}
    <div class="grid">
      {#each $albums as album (album.id)}
        <div class="card">
          <div class="art">
            {#if album.artwork_id}
              <img
                src={artUrl(album.artwork_id)}
                alt={album.title}
                loading="lazy"
                decoding="async"
              />
            {:else}
              <div class="placeholder"><Music size={24} /></div>
            {/if}
          </div>
          <p class="album-title truncate">{album.title}</p>
          {#if album.artist_name}
            <p class="album-artist truncate text-3">{album.artist_name}</p>
          {/if}
        </div>
      {/each}
    </div>
  {/if}
</section>

<style>
  section {
    height: 100%;
    overflow-y: auto;
  }
  h2 {
    margin: 0 0 20px;
    font-size: 20px;
    font-weight: 700;
  }

  .grid {
    display: grid;
    grid-template-columns: repeat(auto-fill, minmax(140px, 1fr));
    gap: 16px;
  }

  .card {
    cursor: pointer;
  }
  .card:hover .art {
    border-color: var(--accent);
  }

  .art {
    aspect-ratio: 1;
    border-radius: 6px;
    overflow: hidden;
    border: 2px solid transparent;
    background: var(--surface);
    display: flex;
    align-items: center;
    justify-content: center;
    margin-bottom: 8px;
    transition: border-color 0.15s;
  }

  .art img {
    width: 100%;
    height: 100%;
    object-fit: cover;
  }
  .placeholder {
    font-size: 36px;
    color: var(--fg-3);
  }

  .album-title {
    margin: 0;
    font-size: 12px;
    font-weight: 600;
  }
  .album-artist {
    margin: 2px 0 0;
    font-size: 11px;
  }
</style>
