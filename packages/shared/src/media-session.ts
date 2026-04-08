import type { Track } from "./types";

interface MediaSessionActions {
  onPlay: () => void;
  onPause: () => void;
  onPrev: () => void;
  onNext: () => void;
}

// holds the url of the currently displayed artwork so we can revoke it when a new track is loaded or the media session is cleared
let currArtObjUrl: string | null = null;

// prevent race conditions when rapidly skipping through tracks and fetching artwork for each one
// if a newer track's artwork finishes loading before an older one, we don't want to overwrite
// the media session metadata with the older track's artwork when it eventually finishes loading
let metadataVersion = 0;

function mediaMetadata(track: Track, artworkSrc?: string): MediaMetadata {
  return new MediaMetadata({
    title: track.title || "Unknown Title", // i would be surprised if the title is ever missing, but still gonna handle this just in case
    artist: track.artist_name || track.album_artist || "Unknown Artist",
    album: track.album_name || "Unknown Album",
    artwork: artworkSrc ? [{ src: artworkSrc, sizes: "512x512" }] : undefined
  });
}

// https://developer.mozilla.org/en-US/docs/Web/API/MediaSession/setActionHandler#examples
function safeSetActionHandler(
  action: MediaSessionAction,
  handler: MediaSessionActionHandler
) {
  try {
    navigator.mediaSession.setActionHandler(action, handler);
  } catch {
    // log the error and move on
    console.warn(`The media session action "${action}" is not supported yet.`);
  }
}

export function setupMediaSessionActions(actions: MediaSessionActions) {
  if (!("mediaSession" in navigator)) return;

  safeSetActionHandler("play", actions.onPlay);
  safeSetActionHandler("pause", actions.onPause);
  safeSetActionHandler("previoustrack", actions.onPrev);
  safeSetActionHandler("nexttrack", actions.onNext);
}

export function setMediaSessionPlaybackState(paused: boolean | null) {
  if (!("mediaSession" in navigator)) return;

  navigator.mediaSession.playbackState =
    paused === null ? "none" : paused ? "paused" : "playing";
}

export function setMediaSessionTrack(track: Track | null) {
  if (!("mediaSession" in navigator)) return;

  if (!track) {
    resetObjUrl();

    navigator.mediaSession.metadata = null;
    return;
  }

  navigator.mediaSession.metadata = mediaMetadata(
    track,
    currArtObjUrl || undefined
  );
}

export function updateMediaSessionMetadata(
  track: Track | null,
  getArtworkUrl: (trackId: string) => string
) {
  if (!("mediaSession" in navigator) || !track) return;

  const version = ++metadataVersion;
  const artworkUrl = getArtworkUrl(track.id);

  resetObjUrl();

  navigator.mediaSession.metadata = mediaMetadata(track);

  if (!artworkUrl) return;

  // Fetch the artwork as a blob and create an object URL for it.
  // This is necessary to work around CORS issues with some
  // media session implementations that don't support CORS on artwork URLs
  async function fetchAndSetArtwork() {
    try {
      const res = await fetch(artworkUrl);
      if (!res.ok || version !== metadataVersion) return;

      const blob = await res.blob();
      if (version !== metadataVersion) return;

      const nextObjURL = URL.createObjectURL(blob);
      currArtObjUrl = nextObjURL;

      navigator.mediaSession.metadata = mediaMetadata(track!, nextObjURL);
    } catch {
      if (version !== metadataVersion) return;

      // Fall back to direct URL if blob doesn't work
      navigator.mediaSession.metadata = mediaMetadata(track!, artworkUrl);
    }
  }

  void fetchAndSetArtwork();
}

function resetObjUrl() {
  if (currArtObjUrl) {
    URL.revokeObjectURL(currArtObjUrl);
    currArtObjUrl = null;
  }
}
