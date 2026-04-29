<script lang="ts">
  import { SearchBar } from "@pneuma/ui";
  import {
    searchTracks,
    searchAlbumGroups,
    fetchAlbumTracks
  } from "../lib/stores/library";
  import { artworkUrl, apiFetch } from "../lib/api";
  import {
    markMissingTrackArtID,
    missingTrackArtIDs
  } from "../lib/stores/missing-art";
  import { pushNav } from "../lib/stores/ui";
  import { playerState, appendTrackToQueue } from "../lib/stores/playback";
  import {
    visiblePlaylistsForAddMenu,
    handleAddTracksToPlaylist,
    toggleFavoriteTrack,
    playlists,
    setPlayingPlaylistContext,
    favoriteTrackIDs
  } from "../lib/stores/playlists";
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

  function artworkUrlWithMissingSuppression(trackID: string) {
    if (!trackID || $missingTrackArtIDs[trackID]) return "";
    return artworkUrl(trackID);
  }

  function handleAlbumArtError(album: AlbumGroup) {
    if (album.first_track_id) {
      markMissingTrackArtID(album.first_track_id);
    }
  }

  function handleAddToQueue(track: Track) {
    appendTrackToQueue(track);
  }

  async function handleToggleFavorite(track: Track) {
    await toggleFavoriteTrack(track);
  }

  function handleIsFavorite(track: Track): boolean {
    return $favoriteTrackIDs.has(track.id);
  }

  async function handleAddToPlaylist(track: Track, playlistId: string) {
    await handleAddTracksToPlaylist([track], playlistId);
  }

  async function handleAddAlbumToPlaylist(
    album: AlbumGroup,
    playlistId: string
  ) {
    const tracks = await fetchAlbumTracks(album.name, album.artist);
    await handleAddTracksToPlaylist(tracks, playlistId);
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
  artworkUrl={artworkUrlWithMissingSuppression}
  onAlbumArtError={handleAlbumArtError}
  onAddToQueue={handleAddToQueue}
  onToggleFavorite={handleToggleFavorite}
  isFavorite={handleIsFavorite}
  playlists={visiblePlaylistsForAddMenu($playlists)}
  onAddToPlaylist={handleAddToPlaylist}
  onAddAlbumToPlaylist={handleAddAlbumToPlaylist}
/>
