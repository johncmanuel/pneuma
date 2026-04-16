<script lang="ts">
  import { SearchBar } from "@pneuma/ui";
  import { searchTracks, searchAlbumGroups } from "../lib/stores/library";
  import { artworkUrl, apiFetch } from "../lib/api";
  import { pushNav } from "../lib/stores/ui";
  import { playerState } from "../lib/stores/playback";
  import { setPlayingPlaylistContext } from "../lib/stores/playlists";
  import { wsSend } from "../lib/ws";
  import type { Track, AlbumGroup } from "@pneuma/shared";

  interface Props {
    onItemSelected?: () => void;
  }

  let { onItemSelected }: Props = $props();

  let searchBar = $state<SearchBar | undefined>();

  async function searchFn(query: string) {
    const [tracks, albums] = await Promise.all([
      searchTracks(query),
      searchAlbumGroups(query)
    ]);
    return { tracks: tracks ?? [], albums: albums ?? [] };
  }

  async function fetchAlbumTracksForQueue(track: Track): Promise<Track[]> {
    try {
      const params = new URLSearchParams();
      params.set("album_name", track.album_name ?? "");
      if (track.album_artist) params.set("album_artist", track.album_artist);

      const r = await apiFetch(`/api/library/tracks?${params}`);
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

  async function playTrack(track: Track) {
    let albumTracks = await fetchAlbumTracksForQueue(track);
    if (albumTracks.length === 0) {
      albumTracks = [track];
    }

    const idx = albumTracks.findIndex((t) => t.id === track.id);
    const queueIds = albumTracks.slice(Math.max(0, idx)).map((t) => t.id);

    setPlayingPlaylistContext(null);

    playerState.update((s) => ({
      ...s,
      trackId: track.id,
      track,
      queue: queueIds,
      queueIndex: 0,
      positionMs: 0,
      paused: false
    }));

    wsSend("playback.queue", {
      track_ids: queueIds,
      start_index: 0
    });
    wsSend("playback.play", {
      track_id: track.id,
      position_ms: 0
    });

    onItemSelected?.();
  }

  function openAlbum(album: AlbumGroup) {
    pushNav({ view: "library", albumKey: album.key });
    onItemSelected?.();
  }

  export function focus() {
    searchBar?.focus();
  }
</script>

<SearchBar
  bind:this={searchBar}
  {searchFn}
  onPlayTrack={playTrack}
  onOpenAlbum={openAlbum}
  {artworkUrl}
/>
