<script lang="ts">
  import { TrackRow } from "@pneuma/ui";
  import {
    visiblePlaylistsForAddMenu,
    favoriteTrackIDs
  } from "../lib/stores/playlists";
  import type { Track, PlaylistSummary } from "@pneuma/shared";

  interface Props {
    track?: Track | null;
    active?: boolean;
    hideAlbum?: boolean;
    dateAdded?: string;
    showRemove?: boolean;
    isLocal?: boolean;
    hideFavoriteIcon?: boolean;
    playlists?: PlaylistSummary[];
    onPlay?: (track: Track | null) => void;
    onSelect?: () => void;
    onAddToQueue?: (track: Track | null) => void;
    onRemove?: (track: Track | null) => void;
    onAddToPlaylist?: (track: Track | null, playlistId: string) => void;
    onToggleFavorite?: (track: Track | null) => void;
  }

  let {
    track,
    active,
    hideAlbum,
    dateAdded,
    showRemove,
    isLocal,
    hideFavoriteIcon,
    playlists,
    onPlay,
    onSelect,
    onAddToQueue,
    onRemove,
    onAddToPlaylist,
    onToggleFavorite
  }: Props = $props();

  const isFavorite = $derived($favoriteTrackIDs.has(track?.id ?? ""));
</script>

<TrackRow
  {track}
  {active}
  {hideAlbum}
  {dateAdded}
  {showRemove}
  {isLocal}
  {isFavorite}
  {hideFavoriteIcon}
  playlists={visiblePlaylistsForAddMenu(playlists ?? [])}
  {onPlay}
  {onSelect}
  {onAddToQueue}
  {onRemove}
  {onAddToPlaylist}
  {onToggleFavorite}
/>
