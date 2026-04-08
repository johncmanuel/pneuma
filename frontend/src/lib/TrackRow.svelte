<script lang="ts">
  import { TrackRow } from "@pneuma/ui";
  import { playlists, addTrackToPlaylist } from "../stores/playlists";
  import { connected } from "../utils/api";
  import type { Track } from "@pneuma/shared";

  interface Props {
    track?: Track | null;
    active?: boolean;
    hideAlbum?: boolean;
    isLocal?: boolean;
    disableLocal?: boolean;
    dateAdded?: string;
    showRemove?: boolean;
    onplay?: (track: Track | null) => void;
    onselect?: () => void;
    onaddtoqueue?: (track: Track | null) => void;
    onremove?: (track: Track | null) => void;
  }

  let {
    track,
    active,
    hideAlbum,
    isLocal,
    disableLocal,
    dateAdded,
    showRemove,
    onplay,
    onselect,
    onaddtoqueue,
    onremove
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
  {dateAdded}
  {showRemove}
  playlists={$playlists}
  offline={!isLocal && !$connected}
  disableLocal={disableLocal ?? false}
  {onplay}
  {onselect}
  {onaddtoqueue}
  {onremove}
  onaddtoplaylist={handleAddToPlaylist}
/>
