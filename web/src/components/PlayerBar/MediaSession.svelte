<script lang="ts">
  import { onMount, onDestroy } from "svelte";
  import { playerState } from "../../lib/stores/playback";
  import {
    setupMediaSessionActions,
    setMediaSessionPlaybackState,
    setMediaSessionTrack,
    updateMediaSessionMetadata
  } from "@pneuma/shared";
  import { artworkUrl } from "../../lib/api";
  import { missingTrackArtIDs } from "../../lib/stores/missing-art";

  interface Props {
    onTogglePause: () => void;
    onSkipPrev: () => void;
    onSkipNext: () => void;
  }

  let { onTogglePause, onSkipPrev, onSkipNext }: Props = $props();

  let track = $derived($playerState.track);
  let hasTrack = $derived(!!$playerState.trackId);
  let trackArtSrc = $derived(
    track && !$missingTrackArtIDs[track.id] ? artworkUrl(track.id) : ""
  );

  let mediaMetadataKey = $derived(
    track
      ? `${track.id}|${track.title}|${track.artist_name}|${track.album_artist}|${track.album_name}`
      : ""
  );

  let lastMediaMetadataKey = $state("");

  onMount(() => {
    setupMediaSessionActions({
      onPlay: () => {
        if ($playerState.paused) onTogglePause();
      },
      onPause: () => {
        if (!$playerState.paused) onTogglePause();
      },
      onPrev: onSkipPrev,
      onNext: onSkipNext
    });
  });

  onDestroy(() => {
    setMediaSessionTrack(null);
    setMediaSessionPlaybackState(null);
  });

  $effect(() => {
    setMediaSessionPlaybackState(hasTrack ? $playerState.paused : null);
  });

  $effect(() => {
    if (
      mediaMetadataKey &&
      mediaMetadataKey !== lastMediaMetadataKey &&
      track
    ) {
      lastMediaMetadataKey = mediaMetadataKey;
      setMediaSessionTrack(track);
      updateMediaSessionMetadata(track, () => trackArtSrc);
    }
  });

  $effect(() => {
    if (!$playerState.trackId) {
      lastMediaMetadataKey = "";
      setMediaSessionTrack(null);
      setMediaSessionPlaybackState(null);
    }
  });
</script>
