<script lang="ts">
  import { onMount, onDestroy } from "svelte";
  import { playerState } from "../../lib/stores/playback";
  import { streamUrl } from "../../lib/api";
  import { wsSend } from "../../lib/ws";
  import type { StreamQuality } from "@pneuma/shared";

  interface Props {
    audio: HTMLAudioElement;
    volume: number;
    mobileView: boolean;
    quality: StreamQuality;
    displayPosition: number;
    audioDurationMs: number;
    seeking: boolean;
    onEnded: () => void;
  }

  let {
    audio = $bindable(),
    volume,
    mobileView,
    quality,
    displayPosition = $bindable(),
    audioDurationMs = $bindable(),
    seeking,
    onEnded
  }: Props = $props();

  let supportsOpusStream = $state(true);
  const OPUS_PROFILES = new Set<StreamQuality>(["low", "medium", "high"]);

  let seekSyncTimer: ReturnType<typeof setTimeout> | null = $state(null);
  let currentAudioSrc = $state("");
  let lastTrackId = $state("");
  let lastPaused = $state(true);
  let rafId = $state(0);

  function startPositionLoop() {
    cancelAnimationFrame(rafId);
    function tick() {
      if (audio && !seeking) {
        displayPosition = audio.currentTime * 1000;
      }
      rafId = requestAnimationFrame(tick);
    }
    rafId = requestAnimationFrame(tick);
  }

  function stopPositionLoop() {
    cancelAnimationFrame(rafId);
  }

  function resolveEffectiveStreamQuality(): StreamQuality {
    if (quality === "original") return "original";
    if (quality === "auto") {
      const autoChoice: StreamQuality = mobileView ? "medium" : "original";
      if (OPUS_PROFILES.has(autoChoice) && !supportsOpusStream)
        return "original";
      return autoChoice;
    }
    if (OPUS_PROFILES.has(quality) && !supportsOpusStream) return "original";
    return quality;
  }

  onMount(() => {
    if (audio) {
      audio.volume = volume;
      const supportsOgg = audio.canPlayType("audio/ogg");
      const supportsOpus = audio.canPlayType('audio/ogg; codecs="opus"');
      supportsOpusStream = Boolean(supportsOpus || supportsOgg);
    }
  });

  onDestroy(() => {
    stopPositionLoop();
    if (seekSyncTimer) clearTimeout(seekSyncTimer);
  });

  $effect(() => {
    if (audio) {
      audio.volume = volume;
    }
  });

  $effect(() => {
    if (audio && $playerState.trackId) {
      const trackChanged = $playerState.trackId !== lastTrackId;
      const pausedChanged = $playerState.paused !== lastPaused;

      if (trackChanged) {
        lastTrackId = $playerState.trackId;
        lastPaused = $playerState.paused;

        if (seekSyncTimer) {
          clearTimeout(seekSyncTimer);
          seekSyncTimer = null;
        }

        const url = streamUrl(
          $playerState.trackId,
          resolveEffectiveStreamQuality()
        );

        if (url) {
          currentAudioSrc = url;
          audio.src = url;
          audio.currentTime = $playerState.positionMs / 1000;
          displayPosition = $playerState.positionMs;
        }

        if (!$playerState.paused) {
          audio.play().catch((e) => {
            if (e.name !== "AbortError") console.warn("Audio play failed", e);
          });
          startPositionLoop();
        }
      } else if (pausedChanged) {
        lastPaused = $playerState.paused;

        if ($playerState.paused && !audio.paused) {
          audio.pause();
          stopPositionLoop();
          displayPosition = audio.currentTime * 1000;
        } else if (!$playerState.paused && audio.paused) {
          audio.play().catch((e) => {
            if (e.name !== "AbortError") console.warn("Audio play failed", e);
          });
          startPositionLoop();
        }
      }
    }
  });

  $effect(() => {
    if (audio && !$playerState.trackId && currentAudioSrc) {
      audio.pause();
      audio.src = "";
      currentAudioSrc = "";
      lastTrackId = "";
      lastPaused = true;
      displayPosition = 0;
      stopPositionLoop();
    }
  });

  function onTimeUpdate() {
    const debounceMs = 5000;
    if (!seekSyncTimer) {
      seekSyncTimer = setTimeout(() => {
        seekSyncTimer = null;
        wsSend("playback.seek", {
          position_ms: audio.currentTime * 1000
        });
      }, debounceMs);
    }
  }

  function changeAudioDuration() {
    if (audio && isFinite(audio.duration)) {
      audioDurationMs = audio.duration * 1000;
    }
  }
</script>

<audio
  bind:this={audio}
  ontimeupdate={onTimeUpdate}
  onended={onEnded}
  onloadedmetadata={changeAudioDuration}
  ondurationchange={changeAudioDuration}
  preload="metadata"
></audio>
