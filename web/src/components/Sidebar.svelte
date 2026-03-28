<script lang="ts">
  import { currentUser, artworkUrl, playlistArtUrl } from "../lib/api";
  import { recentAlbums, recentPlaylists } from "../lib/stores/recent";
  import { pushNav } from "../lib/stores/ui";
  import { Music } from "@lucide/svelte";

  let {
    activeView = "library",
    onnavigate
  }: { activeView?: string; onnavigate?: (id: string) => void } = $props();

  const baseNavItems = [
    { id: "library", label: "Library" },
    { id: "playlists", label: "Playlists" }
  ];

  let navItems = $derived(
    $currentUser?.is_admin
      ? [...baseNavItems, { id: "__dashboard", label: "Dashboard" }]
      : baseNavItems
  );

  let recentItems = $derived(
    [
      ...$recentAlbums.map((a) => ({
        kind: "album" as const,
        key: "al-" + a.album_name + "|||" + a.album_artist,
        name: a.album_name,
        sub: a.album_artist,
        playedAt: a.played_at,
        album: a
      })),
      ...$recentPlaylists.map((p) => ({
        kind: "playlist" as const,
        key: "pl-" + p.playlist_id,
        name: p.name,
        sub: "Playlist",
        playedAt: p.played_at,
        pl: p
      }))
    ].sort((a, b) => b.playedAt.localeCompare(a.playedAt))
  );

  function handleClick(item: (typeof recentItems)[number]) {
    if (item.kind === "album") {
      pushNav({ view: "library", albumKey: item.key.replace("al-", "") });
    } else {
      pushNav({ view: "playlists", playlistId: item.pl.playlist_id });
    }
  }

  function hideImg(e: Event) {
    (e.currentTarget as HTMLImageElement).style.display = "none";
  }
</script>

<nav>
  <div class="logo">pneuma</div>
  <ul>
    {#each navItems as item}
      <li>
        <button
          class:active={item.id !== "__dashboard" && activeView === item.id}
          onclick={() => {
            if (item.id === "__dashboard") {
              window.location.href = "/dashboard";
            } else {
              onnavigate?.(item.id);
            }
          }}
        >
          {item.label}
        </button>
      </li>
    {/each}
  </ul>

  {#if recentItems.length > 0}
    <div class="section-label">Recently Played</div>
    <div class="recent-list">
      {#each recentItems as item (item.key)}
        <button class="recent-item" onclick={() => handleClick(item)}>
          <div class="recent-art">
            {#if item.kind === "album" && item.album.first_track_id}
              <img
                src={artworkUrl(item.album.first_track_id)}
                alt=""
                onerror={hideImg}
                loading="lazy"
              />
            {:else if item.kind === "playlist" && item.pl.playlist_id}
              <img
                src={playlistArtUrl(item.pl.playlist_id)}
                alt=""
                onerror={hideImg}
                loading="lazy"
              />
            {/if}
            <span class="recent-art-ph"><Music size={14} /></span>
          </div>
          <div class="recent-info">
            <span class="recent-name truncate">{item.name}</span>
            <span class="recent-sub truncate text-3">{item.sub}</span>
          </div>
        </button>
      {/each}
    </div>
  {/if}

  <div class="spacer"></div>
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

  .section-label {
    font-size: 10px;
    font-weight: 700;
    text-transform: uppercase;
    letter-spacing: 0.08em;
    color: var(--text-3);
    padding: 16px 16px 6px;
  }

  .recent-list {
    display: flex;
    flex-direction: column;
    gap: 1px;
  }

  .recent-item {
    display: flex;
    align-items: center;
    gap: 10px;
    width: 100%;
    text-align: left;
    padding: 6px 16px;
    transition: background 0.1s;
  }
  .recent-item:hover {
    background: var(--surface-hover);
  }

  .recent-art {
    width: 36px;
    height: 36px;
    border-radius: 4px;
    background: var(--surface-2);
    flex-shrink: 0;
    overflow: hidden;
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
  .recent-art-ph {
    font-size: 14px;
    color: var(--text-3);
  }

  .recent-info {
    display: flex;
    flex-direction: column;
    min-width: 0;
    flex: 1;
    gap: 1px;
  }
  .recent-name {
    font-size: 12px;
    font-weight: 500;
  }
  .recent-sub {
    font-size: 11px;
  }

  .spacer {
    flex: 1;
  }
</style>
