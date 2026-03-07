<script lang="ts">
  import { createEventDispatcher } from "svelte"
  import { recentAlbums, getRecentAlbumArtUrl } from "../stores/recentAlbums"
  import { pushNav } from "../stores/ui"
  import { serverURL, authToken } from "../utils/api"
  export let activeView: string = "library"

  const dispatch = createEventDispatcher()

  const navItems = [
    { id: "library",   label: "Library"  },
    { id: "settings",  label: "Settings" },
  ]

  function openRecentAlbum(album: import("../stores/recentAlbums").RecentAlbum) {
    pushNav({
      view: "library",
      tab: album.isLocal ? "local" : "library",
      subTab: "albums",
      albumKey: album.key,
    })
  }

  function hideImg(e: Event) {
    const img = e.currentTarget as HTMLImageElement
    if (img) img.style.display = "none"
  }

  // Re-compute artwork URLs whenever auth state changes (needed for remote albums)
  $: _authDeps = [$serverURL, $authToken]
</script>

<nav>
  <div class="logo">pneuma</div>
  <ul>
    {#each navItems as item}
      <li>
        <button
          class:active={activeView === item.id}
          on:click={() => dispatch("navigate", item.id)}
        >
          {item.label}
        </button>
      </li>
    {/each}
  </ul>

  {#if $recentAlbums.length > 0}
    <div class="recents-section">
      <p class="recents-heading">Recently Played</p>
      <ul class="recents-list">
        {#each $recentAlbums as album (album.key)}
          <li>
            <button class="recent-row" on:click={() => openRecentAlbum(album)}>
              <div class="recent-art">
                {#if _authDeps && getRecentAlbumArtUrl(album)}
                  <img src={_authDeps && getRecentAlbumArtUrl(album)} alt={album.name} on:error={hideImg} loading="lazy"/>
                {/if}
                <span class="recent-art-placeholder">♫</span>
              </div>
              <div class="recent-info">
                <span class="recent-name truncate">{album.name}</span>
                <span class="recent-artist truncate">{album.artist}</span>
              </div>
            </button>
          </li>
        {/each}
      </ul>
    </div>
  {/if}
</nav>

<style>
  nav {
    background: var(--surface);
    border-right: 1px solid var(--border);
    display: flex;
    flex-direction: column;
    flex: 1;
    overflow-y: auto;
    padding: 16px 0;
  }

  .logo {
    font-size: 18px;
    font-weight: 700;
    color: var(--accent);
    letter-spacing: 2px;
    padding: 0 16px 20px;
  }

  ul { list-style: none; padding: 0; margin: 0; }

  li button {
    display: block;
    width: 100%;
    text-align: left;
    padding: 8px 16px;
    border-radius: 0;
    color: var(--text-2);
    font-size: 13px;
    transition: background 0.1s, color 0.1s;
  }

  li button:hover { background: var(--surface-hover); color: var(--text-1); }
  li button.active { color: var(--text-1); font-weight: 600; }

  /* Recently Played */
  .recents-section {
    margin-top: 24px;
    flex: 1;
    min-height: 0;
    /* display: flex;
    flex-direction: column;
    justify-content: flex-end; */
  }

  .recents-heading {
    margin: 0 0 8px;
    padding: 0 16px;
    font-size: 10px;
    font-weight: 700;
    text-transform: uppercase;
    letter-spacing: 0.08em;
    color: var(--text-3);
  }

  .recents-list { list-style: none; padding: 0; margin: 0; }

  .recent-row {
    display: flex;
    align-items: center;
    gap: 10px;
    width: 100%;
    padding: 5px 16px;
    text-align: left;
    color: var(--text-2);
    transition: background 0.1s, color 0.1s;
  }
  .recent-row:hover { background: var(--surface-hover); color: var(--text-1); }

  .recent-art {
    width: 36px;
    height: 36px;
    flex-shrink: 0;
    border-radius: 4px;
    overflow: hidden;
    background: var(--surface-2, var(--surface));
    position: relative;
    display: flex;
    align-items: center;
    justify-content: center;
  }

  .recent-art img {
    position: absolute;
    width: 100%;
    height: 100%;
    object-fit: cover;
    z-index: 1;
  }

  .recent-art-placeholder {
    font-size: 14px;
    color: var(--text-3);
  }

  .recent-info {
    display: flex;
    flex-direction: column;
    min-width: 0;
    gap: 2px;
  }

  .recent-name {
    font-size: 12px;
    font-weight: 500;
    color: var(--text-1);
    display: block;
  }

  .recent-artist {
    font-size: 11px;
    color: var(--text-3);
    display: block;
  }

  .truncate {
    white-space: nowrap;
    overflow: hidden;
    text-overflow: ellipsis;
  }
</style>
