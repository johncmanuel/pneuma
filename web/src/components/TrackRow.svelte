<script lang="ts">
  import { TrackRow } from "@pneuma/ui";
  import { visiblePlaylistsForAddMenu } from "../lib/stores/playlists";
  import type { Track, PlaylistSummary } from "@pneuma/shared";

  interface Props {
    track?: Track | null;
    active?: boolean;
    hideAlbum?: boolean;
    dateAdded?: string;
    showRemove?: boolean;
    isLocal?: boolean;
    isFavorite?: boolean;
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
    isFavorite,
    playlists,
    onplay,
    onselect,
    onaddtoqueue,
    onremove,
    onaddtoplaylist,
    onToggleFavorite
  }: Props = $props();
</script>

<TrackRow
  {track}
  {active}
  {hideAlbum}
  {dateAdded}
  {showRemove}
  {isLocal}
  {isFavorite}
  playlists={visiblePlaylistsForAddMenu(playlists ?? [])}
  {onplay}
  {onselect}
  {onaddtoqueue}
  {onremove}
  {onaddtoplaylist}
  {onToggleFavorite}
/>
