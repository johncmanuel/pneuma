<script lang="ts">
  import { TrackRow } from "@pneuma/ui";
  import type { Track, PlaylistSummary } from "@pneuma/shared";

  interface Props {
    track?: Track | null;
    active?: boolean;
    hideAlbum?: boolean;
    dateAdded?: string;
    showRemove?: boolean;
    isLocal?: boolean;
    playlists?: PlaylistSummary[];
    onplay?: (track: Track | null) => void;
    onselect?: () => void;
    onaddtoqueue?: (track: Track | null) => void;
    onremove?: (track: Track | null) => void;
    onaddtoplaylist?: (track: Track | null, playlistId: string) => void;
  }

  let {
    track,
    active,
    hideAlbum,
    dateAdded,
    showRemove,
    isLocal,
    playlists,
    onplay,
    onselect,
    onaddtoqueue,
    onremove,
    onaddtoplaylist
  }: Props = $props();
</script>

<TrackRow
  {track}
  {active}
  {hideAlbum}
  {dateAdded}
  {showRemove}
  {isLocal}
  {playlists}
  {onplay}
  {onselect}
  {onaddtoqueue}
  {onremove}
  {onaddtoplaylist}
/>
