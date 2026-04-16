<script lang="ts">
  import { Sidebar } from "@pneuma/ui";
  import { currentUser, artworkUrl, playlistArtUrl } from "../lib/api";
  import {
    ensureFavoritesPlaylist,
    favoritesPlaylistId
  } from "../lib/stores/playlists";
  import { recentAlbums, recentPlaylists } from "../lib/stores/recent";
  import { pushNav } from "../lib/stores/ui";

  interface Props {
    activeView?: string;
    collapsed?: boolean;
    onToggleCollapse?: () => void;
    onNavigate?: (id: string) => void;
    onInteraction?: () => void;
  }

  let {
    activeView = "library",
    collapsed = false,
    onToggleCollapse,
    onNavigate,
    onInteraction
  }: Props = $props();

  const baseNavItems = [
    { id: "library", label: "Library" },
    { id: "favorites", label: "Favorites" },
    { id: "playlists", label: "Playlists" }
  ];

  let navItems = $derived(
    $currentUser?.is_admin
      ? [...baseNavItems, { id: "__dashboard", label: "Dashboard" }]
      : baseNavItems
  );

  let sidebarActiveView = $derived(activeView);

  let filteredRecentPlaylists = $derived(
    $recentPlaylists.filter(
      (p) =>
        p.playlist_id !== $favoritesPlaylistId &&
        p.name.trim().toLowerCase() !== "favorites"
    )
  );

  let recentItems = $derived(
    [
      ...$recentAlbums.map((a) => ({
        key: "al-" + a.album_name + "|||" + a.album_artist,
        name: a.album_name,
        sub: a.album_artist,
        artworkUrl: a.first_track_id ? artworkUrl(a.first_track_id) : undefined,
        playedAt: new Date(a.played_at).getTime()
      })),
      ...filteredRecentPlaylists.map((p) => ({
        key: "pl-" + p.playlist_id,
        name: p.name,
        sub: "Playlist",
        artworkUrl: p.playlist_id ? playlistArtUrl(p.playlist_id) : undefined,
        playedAt: new Date(p.played_at).getTime()
      }))
    ].sort((a, b) => (b.playedAt ?? 0) - (a.playedAt ?? 0))
  );

  $effect(() => {
    ensureFavoritesPlaylist().catch((err) => {
      console.error("Failed to ensure favorites playlist", err);
    });
  });

  async function handleNavClick(id: string) {
    if (id === "__dashboard") {
      window.location.href = "/dashboard";
      onInteraction?.();
    } else if (id === "favorites") {
      const favoritesID =
        $favoritesPlaylistId ?? (await ensureFavoritesPlaylist());
      if (favoritesID) {
        pushNav({ view: "favorites", playlistId: favoritesID });
        onInteraction?.();
      }
    } else {
      onNavigate?.(id);
      onInteraction?.();
    }
  }

  function handleRecentClick(item: { key: string }) {
    if (item.key.startsWith("al-")) {
      pushNav({ view: "library", albumKey: item.key.replace("al-", "") });
      onInteraction?.();
    } else {
      const pl = filteredRecentPlaylists.find(
        (p) => "pl-" + p.playlist_id === item.key
      );
      if (pl) {
        pushNav({ view: "playlists", playlistId: pl.playlist_id });
        onInteraction?.();
      }
    }
  }
</script>

<Sidebar
  activeView={sidebarActiveView}
  {collapsed}
  {onToggleCollapse}
  {navItems}
  {recentItems}
  onNavigate={handleNavClick}
  onRecentClick={handleRecentClick}
/>
