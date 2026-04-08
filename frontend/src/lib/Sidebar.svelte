<script lang="ts">
  import { Sidebar } from "@pneuma/ui";
  import { pushNav } from "../stores/ui";
  import {
    recentAlbums,
    recentPlaylists,
    getRecentAlbumArtUrl,
    getRecentPlaylistArtUrl
  } from "../stores/recentAlbums";
  import { serverURL, authToken } from "../utils/api";

  interface Props {
    activeView?: string;
    onnavigate?: (id: string) => void;
  }

  let { activeView = "library", onnavigate }: Props = $props();

  const navItems = [
    { id: "library", label: "Library" },
    { id: "playlists", label: "Playlists" },
    { id: "settings", label: "Settings" }
  ];

  let _authDeps = $derived([$serverURL, $authToken]);

  let recentItems = $derived(
    [
      ...$recentPlaylists.map((p) => ({
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

  function handleNavClick(id: string) {
    onnavigate?.(id);
  }

  function handleRecentClick(item: { key: string }) {
    if (item.key.startsWith("pl-")) {
      const pl = $recentPlaylists.find((p) => "pl-" + p.id === item.key);
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
  {activeView}
  {navItems}
  {recentItems}
  onnavigate={handleNavClick}
  onRecentClick={handleRecentClick}
/>
