<script lang="ts">
  import { Sidebar } from "@pneuma/ui";
  import { pushNav, selectedPlaylistView } from "../stores/ui";
  import {
    ensureFavoritesPlaylist,
    favoritesPlaylistId
  } from "../stores/playlists";
  import {
    recentAlbums,
    recentPlaylists,
    getRecentAlbumArtUrl,
    getRecentPlaylistArtUrl
  } from "../stores/recentAlbums";
  import { serverURL, authToken } from "../utils/api";

  interface Props {
    activeView?: string;
    onNavigate?: (id: string) => void;
  }

  let { activeView = "library", onNavigate }: Props = $props();

  const navItems = [
    { id: "library", label: "Library" },
    { id: "favorites", label: "Favorites" },
    { id: "playlists", label: "Playlists" },
    { id: "settings", label: "Settings" }
  ];

  let _authDeps = $derived([$serverURL, $authToken]);

  let sidebarActiveView = $derived(
    activeView === "playlists" &&
      $selectedPlaylistView &&
      $favoritesPlaylistId &&
      $selectedPlaylistView === $favoritesPlaylistId
      ? "favorites"
      : activeView
  );

  $effect(() => {
    if (!_authDeps) return;
    ensureFavoritesPlaylist().catch(() => {
      console.warn("Failed to ensure favorites playlist exists");
    });
  });

  let filteredRecentPlaylists = $derived(
    $recentPlaylists.filter(
      (p) =>
        p.id !== $favoritesPlaylistId &&
        p.name.trim().toLowerCase() !== "favorites"
    )
  );

  let recentItems = $derived(
    [
      ...filteredRecentPlaylists.map((p) => ({
        key: "pl-" + p.id,
        name: p.name,
        sub: "Playlist",
        artworkUrl: p.artworkPath
          ? getRecentPlaylistArtUrl(p.artworkPath)
          : undefined,
        playedAt: p.playedAt
      })),
      ...$recentAlbums.map((a) => ({
        key: "al-" + a.key,
        name: a.name,
        sub: a.artist,
        artworkUrl:
          _authDeps && getRecentAlbumArtUrl(a)
            ? getRecentAlbumArtUrl(a)
            : undefined,
        playedAt: a.playedAt ?? 0
      }))
    ].sort((a, b) => (b.playedAt ?? 0) - (a.playedAt ?? 0))
  );

  async function handleNavClick(id: string) {
    if (id === "favorites") {
      const favoritesID =
        $favoritesPlaylistId ?? (await ensureFavoritesPlaylist());
      if (favoritesID) {
        pushNav({ view: "playlists", playlistId: favoritesID, albumKey: null });
      }
      return;
    }

    onNavigate?.(id);
  }

  function handleRecentClick(item: { key: string }) {
    if (item.key.startsWith("pl-")) {
      const pl = filteredRecentPlaylists.find((p) => "pl-" + p.id === item.key);
      if (pl) pushNav({ view: "playlists", playlistId: pl.id, albumKey: null });
    } else {
      const album = $recentAlbums.find((a) => "al-" + a.key === item.key);
      if (album) {
        pushNav({
          view: "library",
          tab: album.isLocal ? "local" : "library",
          subTab: "albums",
          albumKey: album.key
        });
      }
    }
  }
</script>

<Sidebar
  activeView={sidebarActiveView}
  {navItems}
  {recentItems}
  onNavigate={handleNavClick}
  onRecentClick={handleRecentClick}
/>
