<script lang="ts">
  import { TrackRow } from "@pneuma/ui";
  import {
    playlists,
    addTrackToPlaylist,
    visiblePlaylistsForAddMenu
  } from "../stores/playlists";
  import { connected } from "../utils/api";
  import type { Track } from "@pneuma/shared";

  interface Props {
    track?: Track | null;
    active?: boolean;
    hideAlbum?: boolean;
    isLocal?: boolean;
    disableLocal?: boolean;
    isFavorite?: boolean;
    hideFavoriteIcon?: boolean;
    dateAdded?: string;
    showRemove?: boolean;
    onPlay?: (track: Track | null) => void;
    onSelect?: () => void;
    onAddToQueue?: (track: Track | null) => void;
    onRemove?: (track: Track | null) => void;
    onToggleFavorite?: (track: Track | null) => void;
  }

  let {
    track,
    active,
    hideAlbum,
    isLocal,
    disableLocal,
    isFavorite,
    hideFavoriteIcon,
    dateAdded,
    showRemove,
    onPlay,
    onSelect,
    onAddToQueue,
    onRemove,
    onToggleFavorite
  }: Props = $props();

  function handleAddToPlaylist(track: Track | null, playlistId: string) {
    if (track) {
      addTrackToPlaylist(playlistId, track, isLocal ?? false);
    }
  }
</script>

<TrackRow
  {track}
  {active}
  {hideAlbum}
  {isLocal}
  {isFavorite}
  {hideFavoriteIcon}
  {dateAdded}
  {showRemove}
  playlists={visiblePlaylistsForAddMenu($playlists)}
  offline={!isLocal && !$connected}
  disableLocal={disableLocal ?? false}
  {onPlay}
  {onSelect}
  {onAddToQueue}
  {onRemove}
  {onToggleFavorite}
  onAddToPlaylist={handleAddToPlaylist}
/>
