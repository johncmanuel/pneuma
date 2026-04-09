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
    onplay?: (track: Track | null) => void;
    onselect?: () => void;
    onaddtoqueue?: (track: Track | null) => void;
    onremove?: (track: Track | null) => void;
    onaddtoplaylist?: (track: Track | null, playlistId: string) => void;
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
    onplay,
    onselect,
    onaddtoqueue,
    onremove,
    onaddtoplaylist,
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
  {onplay}
  {onselect}
  {onaddtoqueue}
  {onremove}
  {onaddtoplaylist}
  {onToggleFavorite}
/>
