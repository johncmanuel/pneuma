<script lang="ts">
  import { Sidebar } from "@pneuma/ui";
  import { currentUser, artworkUrl, playlistArtUrl } from "../lib/api";
  import { recentAlbums, recentPlaylists } from "../lib/stores/recent";
  import { pushNav } from "../lib/stores/ui";

  interface Props {
    activeView?: string;
    onnavigate?: (id: string) => void;
  }

  let { activeView = "library", onnavigate }: Props = $props();

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
        key: "al-" + a.album_name + "|||" + a.album_artist,
        name: a.album_name,
        sub: a.album_artist,
        artworkUrl: a.first_track_id ? artworkUrl(a.first_track_id) : undefined,
        playedAt: new Date(a.played_at).getTime()
      })),
      ...$recentPlaylists.map((p) => ({
        key: "pl-" + p.playlist_id,
        name: p.name,
        sub: "Playlist",
        artworkUrl: p.playlist_id ? playlistArtUrl(p.playlist_id) : undefined,
        playedAt: new Date(p.played_at).getTime()
      }))
    ].sort((a, b) => (b.playedAt ?? 0) - (a.playedAt ?? 0))
  );

  function handleNavClick(id: string) {
    if (id === "__dashboard") {
      window.location.href = "/dashboard";
    } else {
      onnavigate?.(id);
    }
  }

  function handleRecentClick(item: { key: string }) {
    if (item.key.startsWith("al-")) {
      pushNav({ view: "library", albumKey: item.key.replace("al-", "") });
    } else {
      const pl = $recentPlaylists.find(
        (p) => "pl-" + p.playlist_id === item.key
      );
      if (pl) pushNav({ view: "playlists", playlistId: pl.playlist_id });
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
