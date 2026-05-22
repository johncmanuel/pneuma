<script lang="ts">
  import { onMount, onDestroy } from "svelte";
  import { playerState } from "../../lib/stores/playback";
  import { streamUrl } from "../../lib/api";
  import { wsSend } from "../../lib/ws";
  import type { CrossfadeConfig, StreamQuality } from "@pneuma/shared";

  interface Props {
    audio: HTMLAudioElement;
    volume: number;
    mobileView: boolean;
    quality: StreamQuality;
    crossfade: CrossfadeConfig;
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
    crossfade,
    displayPosition = $bindable(),
    audioDurationMs = $bindable(),
    seeking,
    onEnded
  }: Props = $props();

  let supportsOpusStream = $state(true);
  const OPUS_PROFILES = new Set<StreamQuality>(["low", "medium", "high"]);

  let seekSyncTimer: ReturnType<typeof setTimeout> | null = $state(null);
  let currentTrackIdInAudio = $state("");
  let lastTrackId = $state("");
  let lastPaused = $state(true);
  let rafId = $state(0);

  let audioCtx: AudioContext | null = $state(null);
  let gainA: GainNode | null = $state(null);
  let gainB: GainNode | null = $state(null);
  let sourceA: MediaElementAudioSourceNode | null = null;
  let sourceB: MediaElementAudioSourceNode | null = null;

  let audioA = $state<HTMLAudioElement | null>(null);
  let audioB = $state<HTMLAudioElement | null>(null);
  let primaryIsA = $state(true);

  let crossfadeTriggered = $state(false);
  let crossfadeActive = $state(false);

  // Keep parent's audio prop bound to the currently active element
  $effect(() => {
    const active = primaryIsA ? audioA : audioB;
    if (active) {
      audio = active;
    }
  });

  function startPositionLoop() {
    cancelAnimationFrame(rafId);
    function tick() {
      const active = primaryIsA ? audioA : audioB;
      if (active && !seeking) {
        displayPosition = active.currentTime * 1000;
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

  function ensureAudioContext(): AudioContext {
    if (!audioCtx) {
      audioCtx = new AudioContext();
    }
    if (audioCtx.state === "suspended") {
      audioCtx
        .resume()
        .catch((e) => console.warn("AudioContext resume failed", e));
    }
    return audioCtx;
  }

  function ensureAudioRouting() {
    const ctx = ensureAudioContext();
    if (audioA && !sourceA) {
      gainA = ctx.createGain();
      gainA.connect(ctx.destination);
      sourceA = ctx.createMediaElementSource(audioA);
      sourceA.connect(gainA);
      gainA.gain.value = primaryIsA ? volume : 0;
    }
    if (audioB && !sourceB) {
      gainB = ctx.createGain();
      gainB.connect(ctx.destination);
      sourceB = ctx.createMediaElementSource(audioB);
      sourceB.connect(gainB);
      gainB.gain.value = primaryIsA ? 0 : volume;
    }
  }

  function cancelCrossfade() {
    if (!audioCtx) return;
    const now = audioCtx.currentTime;

    gainA?.gain.cancelScheduledValues(now);
    gainB?.gain.cancelScheduledValues(now);

    const activeGain = primaryIsA ? gainA : gainB;
    if (activeGain) {
      activeGain.gain.setValueAtTime(volume, now);
    }

    crossfadeTriggered = false;
    crossfadeActive = false;

    const incoming = primaryIsA ? audioB : audioA;
    if (incoming) {
      incoming.pause();
      incoming.src = "";
    }
  }

  function startCrossfade(
    nextUrl: string,
    nextTrackId: string,
    durationSec: number
  ) {
    ensureAudioRouting();
    const ctx = audioCtx!;
    crossfadeActive = true;

    const incoming = primaryIsA ? audioB : audioA;
    if (!incoming) return;

    incoming.src = nextUrl;
    incoming.onerror = () => {
      console.warn("crossfade: incoming track failed to load, aborting");
      cancelCrossfade();
    };

    incoming.play().catch((e) => {
      if (e.name !== "AbortError") {
        console.warn("crossfade: incoming play failed", e);
        cancelCrossfade();
      }
    });

    const activeGain = primaryIsA ? gainA : gainB;
    const incomingGain = primaryIsA ? gainB : gainA;

    const now = ctx.currentTime;
    const end = now + durationSec;

    activeGain?.gain.setValueAtTime(activeGain?.gain.value ?? volume, now);
    activeGain?.gain.linearRampToValueAtTime(0, end);

    incomingGain?.gain.setValueAtTime(0, now);
    incomingGain?.gain.linearRampToValueAtTime(volume, end);

    setTimeout(() => {
      if (!crossfadeActive) return;

      const active = primaryIsA ? audioA : audioB;
      if (active) {
        active.pause();
        active.src = "";
      }

      primaryIsA = !primaryIsA;
      crossfadeActive = false;
      crossfadeTriggered = false;
      currentTrackIdInAudio = nextTrackId;

      // Update duration immediately since the metadata is already loaded
      if (incoming && isFinite(incoming.duration)) {
        audioDurationMs = incoming.duration * 1000;
      }

      onEnded();
    }, durationSec * 1000);
  }

  function handleEnded(el: HTMLAudioElement) {
    const active = primaryIsA ? audioA : audioB;
    if (el !== active) return;

    if (crossfadeActive) return;
    onEnded();
  }

  function onTimeUpdate(el: HTMLAudioElement) {
    const active = primaryIsA ? audioA : audioB;
    if (el !== active) return;

    const debounceMs = 5000;
    if (!seekSyncTimer) {
      seekSyncTimer = setTimeout(() => {
        seekSyncTimer = null;
        wsSend("playback.seek", {
          position_ms: active.currentTime * 1000
        });
      }, debounceMs);
    }

    if (active && isFinite(active.duration) && active.duration > 0) {
      const remaining = active.duration - active.currentTime;

      // Handle seeking backward out of crossfade zone
      if (
        (crossfadeActive || crossfadeTriggered) &&
        remaining > crossfade.durationSec
      ) {
        cancelCrossfade();
      }

      // Crossfade trigger (only if at least 1.0 seconds remain)
      if (
        crossfade.enabled &&
        !crossfadeTriggered &&
        !crossfadeActive &&
        remaining >= 1.0 &&
        remaining <= crossfade.durationSec
      ) {
        crossfadeTriggered = true;

        const q = $playerState.queue;
        const idx = q.indexOf($playerState.trackId);
        const nextIdx = idx + 1 < q.length ? idx + 1 : -1;
        if (nextIdx >= 0) {
          const nextId = q[nextIdx];
          const nextUrl = streamUrl(nextId, {
            quality: resolveEffectiveStreamQuality()
          });
          if (nextUrl) {
            const fadeDuration = Math.max(
              1,
              Math.min(crossfade.durationSec, remaining)
            );
            startCrossfade(nextUrl, nextId, fadeDuration);
          }
        }
      }
    }
  }

  function changeAudioDuration(el: HTMLAudioElement) {
    const active = primaryIsA ? audioA : audioB;
    if (el !== active) return;

    if (el && isFinite(el.duration)) {
      audioDurationMs = el.duration * 1000;
    }
  }

  onMount(() => {
    if (audioA) {
      audioA.volume = volume;
      const supportsOgg = audioA.canPlayType("audio/ogg");
      const supportsOpus = audioA.canPlayType('audio/ogg; codecs="opus"');
      supportsOpusStream = Boolean(supportsOpus || supportsOgg);
    }
    if (audioB) {
      audioB.volume = volume;
    }
  });

  onDestroy(() => {
    stopPositionLoop();
    if (seekSyncTimer) clearTimeout(seekSyncTimer);
    cancelCrossfade();
    audioCtx?.close().catch(() => {});
    audioCtx = null;
  });

  // Update volume when it changes
  $effect(() => {
    const active = primaryIsA ? audioA : audioB;
    if (active) {
      active.volume = volume;
      const activeGain = primaryIsA ? gainA : gainB;
      if (activeGain && audioCtx && !crossfadeActive) {
        activeGain.gain.setValueAtTime(volume, audioCtx.currentTime);
      }
    }
  });

  // Update audio when trackId changes
  $effect(() => {
    const active = primaryIsA ? audioA : audioB;
    if (active && $playerState.trackId) {
      const trackChanged = $playerState.trackId !== lastTrackId;
      const pausedChanged = $playerState.paused !== lastPaused;

      if (trackChanged) {
        if (crossfadeActive) {
          cancelCrossfade();
        }

        lastTrackId = $playerState.trackId;
        lastPaused = $playerState.paused;
        crossfadeTriggered = false;

        if (seekSyncTimer) {
          clearTimeout(seekSyncTimer);
          seekSyncTimer = null;
        }

        const url = streamUrl($playerState.trackId, {
          quality: resolveEffectiveStreamQuality()
        });

        if (url) {
          if (currentTrackIdInAudio !== $playerState.trackId) {
            ensureAudioRouting();
            currentTrackIdInAudio = $playerState.trackId;
            active.src = url;
            active.currentTime = $playerState.positionMs / 1000;
            displayPosition = $playerState.positionMs;

            const activeGain = primaryIsA ? gainA : gainB;
            if (activeGain && audioCtx) {
              activeGain.gain.cancelScheduledValues(audioCtx.currentTime);
              activeGain.gain.setValueAtTime(volume, audioCtx.currentTime);
            }
          }
        }

        if (!$playerState.paused) {
          active.play().catch((e) => {
            if (e.name !== "AbortError") console.warn("Audio play failed", e);
          });
          startPositionLoop();
        }
      } else if (pausedChanged) {
        lastPaused = $playerState.paused;

        if ($playerState.paused) {
          audioA?.pause();
          audioB?.pause();
          if (audioCtx?.state === "running") {
            audioCtx.suspend();
          }
          stopPositionLoop();
          displayPosition = active.currentTime * 1000;
        } else {
          if (audioCtx?.state === "suspended") {
            audioCtx.resume();
          }
          if (crossfadeActive) {
            audioA?.play().catch(() => {});
            audioB?.play().catch(() => {});
          } else {
            active.play().catch((e) => {
              if (e.name !== "AbortError") console.warn("Audio play failed", e);
            });
          }
          startPositionLoop();
        }
      }
    }
  });

  // Clear audio when trackId is cleared and reset state
  $effect(() => {
    const active = primaryIsA ? audioA : audioB;
    if (active && !$playerState.trackId && currentTrackIdInAudio) {
      cancelCrossfade();
      active.pause();

      active.src = "";
      currentTrackIdInAudio = "";
      lastTrackId = "";
      lastPaused = true;
      displayPosition = 0;

      stopPositionLoop();
    }
  });
</script>

<audio
  bind:this={audioA}
  ontimeupdate={() => onTimeUpdate(audioA!)}
  onended={() => handleEnded(audioA!)}
  onloadedmetadata={() => changeAudioDuration(audioA!)}
  ondurationchange={() => changeAudioDuration(audioA!)}
  preload="metadata"
  crossorigin="anonymous"
></audio>

<audio
  bind:this={audioB}
  ontimeupdate={() => onTimeUpdate(audioB!)}
  onended={() => handleEnded(audioB!)}
  onloadedmetadata={() => changeAudioDuration(audioB!)}
  ondurationchange={() => changeAudioDuration(audioB!)}
  preload="metadata"
  crossorigin="anonymous"
></audio>
