<script lang="ts">
  import {
    recentAlbums,
    recentPlaylists,
    getRecentAlbumArtUrl,
    getRecentPlaylistArtUrl
  } from "../stores/recentAlbums";
  import { pushNav } from "../stores/ui";
  import { serverURL, authToken } from "../utils/api";
  import { Music } from "@lucide/svelte";

  let {
    activeView = "library",
    onnavigate
  }: { activeView?: string; onnavigate?: (id: string) => void } = $props();

  const navItems = [
    { id: "library", label: "Library" },
    { id: "playlists", label: "Playlists" },
    { id: "settings", label: "Settings" }
  ];

  function openRecentAlbum(
    album: import("../stores/recentAlbums").RecentAlbum
  ) {
    pushNav({
      view: "library",
      tab: album.isLocal ? "local" : "library",
      subTab: "albums",
      albumKey: album.key
    });
  }

  function openRecentPlaylist(
    pl: import("../stores/recentAlbums").RecentPlaylist
  ) {
    pushNav({ view: "playlists", playlistId: pl.id, albumKey: null });
  }

  function hideImg(e: Event) {
    const img = e.currentTarget as HTMLImageElement;
    if (img) img.style.display = "none";
  }

  // Re-compute artwork URLs whenever auth state changes (needed for remote albums)
  let _authDeps = $derived([$serverURL, $authToken]);

  // Merge and sort recent albums + playlists by playedAt in descending order
  let recentItems = $derived(
    [
      ...$recentPlaylists.map((p) => ({
        kind: "playlist" as const,
        key: "pl-" + p.id,
        name: p.name,
        sub: "Playlist",
        playedAt: p.playedAt,
        pl: p,
        album: null
      })),
      ...$recentAlbums.map((a) => ({
        kind: "album" as const,
        key: "al-" + a.key,
        name: a.name,
        sub: a.artist,
        playedAt: a.playedAt ?? 0,
        pl: null,
        album: a
      }))
    ].sort((a, b) => b.playedAt - a.playedAt)
    // no need to a limit for now; but if performance does get worst, i'm adding this back in
    // .slice(0, 20)
  );
</script>

<nav>
  <div class="logo">pneuma</div>
  <ul>
    {#each navItems as item}
      <li>
        <button
          class:active={activeView === item.id}
          onclick={() => onnavigate?.(item.id)}
        >
          {item.label}
        </button>
      </li>
    {/each}
  </ul>

  {#if recentItems.length > 0}
    <div class="recents-section">
      <p class="recents-heading">Recently Played</p>
      <ul class="recents-list">
        {#each recentItems as item (item.key)}
          <li>
            {#if item.kind === "playlist" && item.pl}
              <button
                class="recent-row"
                onclick={() => openRecentPlaylist(item.pl)}
              >
                <div class="recent-art">
                  {#if item.pl.artworkPath && getRecentPlaylistArtUrl(item.pl.artworkPath)}
                    <img
                      src={getRecentPlaylistArtUrl(item.pl.artworkPath)}
                      alt={item.pl.name}
                      onerror={hideImg}
                      loading="lazy"
                    />
                  {/if}
                  <span class="recent-art-placeholder"><Music size={12} /></span
                  >
                </div>
                <div class="recent-info">
                  <span class="recent-name truncate">{item.pl.name}</span>
                  <span class="recent-artist truncate">Playlist</span>
                </div>
              </button>
            {:else if item.kind === "album" && item.album}
              <button
                class="recent-row"
                onclick={() => openRecentAlbum(item.album)}
              >
                <div class="recent-art">
                  {#if _authDeps && getRecentAlbumArtUrl(item.album)}
                    <img
                      src={_authDeps && getRecentAlbumArtUrl(item.album)}
                      alt={item.album.name}
                      onerror={hideImg}
                      loading="lazy"
                    />
                  {/if}
                  <span class="recent-art-placeholder"><Music size={12} /></span
                  >
                </div>
                <div class="recent-info">
                  <span class="recent-name truncate">{item.album.name}</span>
                  <span class="recent-artist truncate">{item.album.artist}</span
                  >
                </div>
              </button>
            {/if}
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

  ul {
    list-style: none;
    padding: 0;
    margin: 0;
  }

  li button {
    display: block;
    width: 100%;
    text-align: left;
    padding: 8px 16px;
    border-radius: 0;
    color: var(--text-2);
    font-size: 13px;
    transition:
      background 0.1s,
      color 0.1s;
  }

  li button:hover {
    background: var(--surface-hover);
    color: var(--text-1);
  }
  li button.active {
    color: var(--text-1);
    font-weight: 600;
  }

  .recents-section {
    margin-top: 24px;
    flex: 1;
    min-height: 0;
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

  .recents-list {
    list-style: none;
    padding: 0;
    margin: 0;
  }

  .recent-row {
    display: flex;
    align-items: center;
    gap: 10px;
    width: 100%;
    padding: 5px 16px;
    text-align: left;
    color: var(--text-2);
    transition:
      background 0.1s,
      color 0.1s;
  }
  .recent-row:hover {
    background: var(--surface-hover);
    color: var(--text-1);
  }

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
