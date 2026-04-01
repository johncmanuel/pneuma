<script lang="ts">
  import { SearchBar } from "@pneuma/ui";
  import { searchTracks, searchAlbumGroups } from "../stores/library";
  import {
    searchLocalTracksQuery,
    searchLocalAlbumGroups,
    fetchLocalAlbumTracks,
    type LocalAlbumGroup,
    localTrackToTrack
  } from "../stores/localLibrary";
  import { playerState, type Track } from "../stores/player";
  import { connected, serverFetch, artworkUrl, localBase } from "../utils/api";
  import { wsSend } from "../stores/ws";
  import { pushNav } from "../stores/ui";

  interface TaggedTrack extends Track {
    _source: "remote" | "local";
  }

  let searchBar: SearchBar;

  async function searchFn(query: string) {
    const [remoteResults, localResults, remoteAlbums, localAlbums] =
      await Promise.all([
        searchTracks(query),
        searchLocalTracksQuery(query),
        searchAlbumGroups(query),
        searchLocalAlbumGroups(query)
      ]);

    const remote: TaggedTrack[] = (remoteResults ?? []).map((t) => ({
      ...t,
      _source: "remote" as const
    }));

    const local: TaggedTrack[] = (localResults ?? []).slice(0, 20).map((t) => ({
      ...localTrackToTrack(t),
      _source: "local" as const
    }));

    const localAlbumArt = (album: LocalAlbumGroup) => {
      const base = localBase();
      if (!base || !album.first_track_path) return "";
      return `${base}/local/art?path=${encodeURIComponent(album.first_track_path)}`;
    };

    return {
      tracks: [...remote, ...local],
      albums: [
        ...(remoteAlbums ?? []),
        ...(localAlbums ?? []).map((a) => ({
          key: a.key + "-local",
          name: a.name,
          artist: a.artist,
          track_count: a.track_count,
          first_track_id: a.first_track_path,
          _localArtUrl: localAlbumArt(a)
        }))
      ]
    };
  }

  async function fetchAlbumTracksForQueue(
    track: TaggedTrack
  ): Promise<Track[]> {
    try {
      if (track._source === "local") {
        const locals = await fetchLocalAlbumTracks(
          track.album_name ?? "",
          track.album_artist ?? ""
        );
        return [...locals]
          .sort(
            (a, b) =>
              (a.disc_number ?? 0) - (b.disc_number ?? 0) ||
              (a.track_number ?? 0) - (b.track_number ?? 0)
          )
          .map(localTrackToTrack);
      }

      const params = new URLSearchParams();
      params.set("album_name", track.album_name ?? "");
      if (track.album_artist) params.set("album_artist", track.album_artist);

      const r = await serverFetch(`/api/library/tracks?${params}`);
      if (r.ok) {
        const data = await r.json();
        const fetched: Track[] = Array.isArray(data)
          ? data
          : (data.tracks ?? []);
        return [...fetched].sort(
          (a, b) =>
            (a.disc_number ?? 0) - (b.disc_number ?? 0) ||
            (a.track_number ?? 0) - (b.track_number ?? 0)
        );
      }
    } catch {}
    return [];
  }

  async function playTrack(track: TaggedTrack) {
    if (track._source === "remote" && !$connected) return;

    let albumTracks = await fetchAlbumTracksForQueue(track);
    if (albumTracks.length === 0) {
      albumTracks = [track];
    }

    const idx = albumTracks.findIndex((t) => t.id === track.id);
    const queue = albumTracks.slice(Math.max(0, idx)).map((t) => t.id);
    const baseQueue = albumTracks.map((t) => t.id);

    playerState.update((s) => ({
      ...s,
      trackId: track.id,
      track,
      queue,
      baseQueue,
      queueIndex: 0,
      positionMs: 0,
      paused: false
    }));

    if (track._source === "remote") {
      wsSend("playback.queue", { track_ids: queue, start_index: 0 });
      wsSend("playback.play", { track_id: track.id, position_ms: 0 });
    }
  }

  function openAlbum(album: { key: string; first_track_path?: string }) {
    const isLocal = album.key.endsWith("-local") || !!album.first_track_path;
    pushNav({
      view: "library",
      tab: isLocal ? "local" : "library",
      subTab: "albums",
      albumKey: album.key.replace("-local", "")
    });
  }

  export function focus() {
    searchBar?.focus();
  }

  export const hasResults = () => searchBar?.hasResults?.() ?? false;
</script>

<SearchBar
  bind:this={searchBar}
  placeholder="Search tracks, playlists, albums..."
  {searchFn}
  onPlayTrack={(track) => playTrack(track as TaggedTrack)}
  onOpenAlbum={openAlbum}
  artworkUrl={(id) => {
    if (!id) return "";
    if (id.includes("/") || id.includes(":")) {
      const base = localBase();
      return base ? `${base}/local/art?path=${encodeURIComponent(id)}` : "";
    }
    return artworkUrl(id);
  }}
/>
