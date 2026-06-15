<script lang="ts">
  import { onMount, onDestroy } from "svelte";
  import { playerState, seekRequest } from "../../lib/stores/playback";
  import { streamUrl, apiFetch } from "../../lib/api";
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

  // Pending crossfade duration set when we trigger onEnded to advance the queue.
  // The track-change $effect reads and consumes this to start the actual gain ramps.
  let pendingCrossfadeDuration = $state<number | null>(null);

  // Timer that removes the old (secondary) element after the fade finishes
  let crossfadeTeardownTimer: ReturnType<typeof setTimeout> | null = null;

  let nextTrackId = $derived(
    $playerState.repeat === 2
      ? $playerState.trackId
      : $playerState.queue[$playerState.queueIndex + 1] ||
          ($playerState.repeat === 1 ? $playerState.queue[0] : null)
  );
  let preloadedTrackId = $state("");

  let telemetryTrackId = "";
  let telemetryQuality = "";
  let playRequestTime = 0; // performance.now() when play() is called
  let latencyMs = 0; // time from play() to first audio output
  let latencyRecorded = false;
  let stutterCount = 0;
  let stutterDurationMs = 0;
  let waitingStartTime = 0; // performance.now() when a 'waiting' event fires

  // Keep parent's audio prop bound to the currently active element
  $effect(() => {
    const active = primaryIsA ? audioA : audioB;
    if (active) {
      audio = active;
    }
  });

  $effect(() => {
    const sr = $seekRequest;
    if (sr !== null) {
      const active = primaryIsA ? audioA : audioB;
      if (active) active.currentTime = sr / 1000;
      playerState.update((s) => ({ ...s, positionMs: sr }));
      displayPosition = sr;
      wsSend("playback.seek", { position_ms: sr });
      seekRequest.set(null);
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

  function resetTelemetry(trackId: string, quality: string) {
    telemetryTrackId = trackId;
    telemetryQuality = quality;
    playRequestTime = performance.now();
    latencyMs = 0;
    latencyRecorded = false;
    stutterCount = 0;
    stutterDurationMs = 0;
    waitingStartTime = 0;
  }

  function flushTelemetry() {
    if (!telemetryTrackId || !latencyRecorded) return;

    const body = {
      track_id: telemetryTrackId,
      stream_quality: telemetryQuality || "original",
      latency_ms: Math.round(latencyMs),
      stutter_count: stutterCount,
      stutter_duration_ms: Math.round(stutterDurationMs)
    };

    apiFetch("/api/telemetry/stream", {
      method: "POST",
      body: JSON.stringify(body)
    }).catch((e) => console.warn("Telemetry submit failed", e));

    telemetryTrackId = "";
  }

  function onAudioPlaying(el: HTMLAudioElement) {
    const active = primaryIsA ? audioA : audioB;
    if (el !== active) return;

    if (!latencyRecorded && playRequestTime > 0) {
      latencyMs = performance.now() - playRequestTime;
      latencyRecorded = true;
    }
  }

  function onAudioWaiting(el: HTMLAudioElement) {
    const active = primaryIsA ? audioA : audioB;
    if (el !== active) return;

    // Only count as a stutter after initial playback has started
    if (latencyRecorded) {
      stutterCount++;
      waitingStartTime = performance.now();
    }
  }

  function onAudioCanPlay(el: HTMLAudioElement) {
    const active = primaryIsA ? audioA : audioB;
    if (el !== active) return;

    if (waitingStartTime > 0) {
      stutterDurationMs += performance.now() - waitingStartTime;
      waitingStartTime = 0;
    }
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
    pendingCrossfadeDuration = null;

    if (crossfadeTeardownTimer) {
      clearTimeout(crossfadeTeardownTimer);
      crossfadeTeardownTimer = null;
    }

    const incoming = primaryIsA ? audioB : audioA;
    if (incoming) {
      incoming.pause();
      incoming.src = "";
    }
  }

  function beginCrossfadeRamps(durationSec: number) {
    ensureAudioRouting();
    const ctx = audioCtx!;
    crossfadeActive = true;

    const newGain = primaryIsA ? gainA : gainB;
    const oldGain = primaryIsA ? gainB : gainA;

    const now = ctx.currentTime;
    const end = now + durationSec;

    // Fade out old track then fade in new track
    oldGain?.gain.setValueAtTime(oldGain?.gain.value ?? volume, now);
    oldGain?.gain.linearRampToValueAtTime(0, end);
    newGain?.gain.setValueAtTime(0, now);
    newGain?.gain.linearRampToValueAtTime(volume, end);

    crossfadeTeardownTimer = setTimeout(() => {
      if (!crossfadeActive) return;

      const old = primaryIsA ? audioB : audioA;
      if (old) {
        old.pause();
        old.src = "";
      }

      crossfadeActive = false;
      crossfadeTriggered = false;
      crossfadeTeardownTimer = null;
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

    if (!seeking) {
      playerState.update((s) => ({
        ...s,
        positionMs: active.currentTime * 1000
      }));
    }

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

      // Preload the next track into the idle audio element
      if (
        remaining <= 15.0 &&
        nextTrackId &&
        preloadedTrackId !== nextTrackId
      ) {
        const idle = primaryIsA ? audioB : audioA;
        if (idle && !crossfadeActive) {
          const preloadUrl = streamUrl(nextTrackId, {
            quality: resolveEffectiveStreamQuality()
          });
          if (preloadUrl) {
            idle.src = preloadUrl;
            preloadedTrackId = nextTrackId;
          }
        }
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

        const fadeDuration = Math.max(
          1,
          Math.min(crossfade.durationSec, remaining)
        );

        // Flip primary so the new track loads into the new primary element.
        // The oldtrack keeps playing in the now-secondary element.
        primaryIsA = !primaryIsA;
        pendingCrossfadeDuration = fadeDuration;

        onEnded();
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

    const onBeforeUnload = () => {
      flushTelemetry();
    };

    window.addEventListener("beforeunload", onBeforeUnload);

    return () => {
      window.removeEventListener("beforeunload", onBeforeUnload);
    };
  });

  onDestroy(() => {
    flushTelemetry();
    stopPositionLoop();

    if (seekSyncTimer) clearTimeout(seekSyncTimer);
    if (crossfadeTeardownTimer) clearTimeout(crossfadeTeardownTimer);

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
      if (
        activeGain &&
        audioCtx &&
        !crossfadeActive &&
        !pendingCrossfadeDuration
      ) {
        activeGain.gain.setValueAtTime(volume, audioCtx.currentTime);
      }
    }
  });

  // Update audio when trackId changes
  $effect(() => {
    let active = primaryIsA ? audioA : audioB;
    if (active && $playerState.trackId) {
      const trackChanged = $playerState.trackId !== lastTrackId;
      const pausedChanged = $playerState.paused !== lastPaused;

      if (trackChanged) {
        // Flush telemetry for the previous track before switching
        flushTelemetry();

        // If the user manually skipped during an active crossfade, hard-cancel it
        if (crossfadeActive && !pendingCrossfadeDuration) {
          cancelCrossfade();
        }

        lastTrackId = $playerState.trackId;
        lastPaused = $playerState.paused;
        crossfadeTriggered = false;

        if (seekSyncTimer) {
          clearTimeout(seekSyncTimer);
          seekSyncTimer = null;
        }

        const fadeDuration = pendingCrossfadeDuration;
        pendingCrossfadeDuration = null;

        const url = streamUrl($playerState.trackId, {
          quality: resolveEffectiveStreamQuality()
        });

        if (url) {
          if (currentTrackIdInAudio !== $playerState.trackId) {
            const wasPreloaded = preloadedTrackId === $playerState.trackId;
            if (wasPreloaded) {
              primaryIsA = !primaryIsA;
              active = primaryIsA ? audioA : audioB;
              preloadedTrackId = "";
            }

            if (crossfade.enabled) {
              ensureAudioRouting();
            }

            currentTrackIdInAudio = $playerState.trackId;
            resetTelemetry(
              $playerState.trackId,
              resolveEffectiveStreamQuality()
            );

            if (!wasPreloaded) {
              active!.src = url;
            }
            active!.currentTime = $playerState.positionMs / 1000;
            displayPosition = $playerState.positionMs;

            const activeGain = primaryIsA ? gainA : gainB;
            if (activeGain && audioCtx) {
              activeGain.gain.cancelScheduledValues(audioCtx.currentTime);
              // If crossfading, the gain starts at 0 (note: beginCrossfadeRamps will ramp it up).
              // Otherwise, set it to the volume immediately.
              if (!fadeDuration) {
                activeGain.gain.setValueAtTime(volume, audioCtx.currentTime);
              }
            }
          }
        }

        if (!$playerState.paused) {
          active.play().catch((e) => {
            if (e.name !== "AbortError") console.warn("Audio play failed", e);
          });
          startPositionLoop();
        }

        if (fadeDuration) {
          beginCrossfadeRamps(fadeDuration);
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
      flushTelemetry();
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
  onplaying={() => onAudioPlaying(audioA!)}
  onwaiting={() => onAudioWaiting(audioA!)}
  oncanplay={() => onAudioCanPlay(audioA!)}
  preload="metadata"
  crossorigin="anonymous"
></audio>

<audio
  bind:this={audioB}
  ontimeupdate={() => onTimeUpdate(audioB!)}
  onended={() => handleEnded(audioB!)}
  onloadedmetadata={() => changeAudioDuration(audioB!)}
  ondurationchange={() => changeAudioDuration(audioB!)}
  onplaying={() => onAudioPlaying(audioB!)}
  onwaiting={() => onAudioWaiting(audioB!)}
  oncanplay={() => onAudioCanPlay(audioB!)}
  preload="metadata"
  crossorigin="anonymous"
></audio>
