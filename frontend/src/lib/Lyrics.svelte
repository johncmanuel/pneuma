<script lang="ts">
  import { playerState, seekRequest } from "../stores/player";
  import { serverFetch, artworkUrl, localBase } from "../utils/api";
  import { isLocalID, type LrcLine, parseLrc } from "@pneuma/shared";
  import { Music } from "@lucide/svelte";

  let trackId = $derived($playerState.trackId ?? "");
  let track = $derived($playerState.track);
  let positionMs = $derived($playerState.positionMs);

  let lines: LrcLine[] = $state([]);
  let loadError = $state(false);
  let loading = $state(false);
  let lastFetchedId = $state("");

  // The lyric line currently active based on playhead position
  let activeIndex = $derived(
    lines.findLastIndex((line) => line.timeMs <= positionMs)
  );

  async function fetchLyrics(id: string) {
    if (!id) {
      lines = [];
      loadError = false;
      lastFetchedId = id;
      return;
    }

    loading = true;
    loadError = false;
    lines = [];

    try {
      const isLocal = isLocalID(id);
      const url = isLocal
        ? `${localBase()}/local/lyrics?path=${encodeURIComponent(id)}`
        : `/api/library/tracks/${id}/lyrics`;

      const res = isLocal ? await fetch(url) : await serverFetch(url);
      if (!res.ok) {
        loadError = true;
        return;
      }

      const raw = await res.text();
      const parsed = parseLrc(raw);

      // If the file had content but produced no valid lines, treat as malformed
      if (raw.trim().length > 0 && parsed.length === 0) {
        console.warn("LRC file appears malformed, ignoring");
        loadError = true;
        return;
      }

      lines = parsed;
    } catch {
      console.warn("Failed to fetch lyrics for track:", id);
      loadError = true;
    } finally {
      loading = false;
      lastFetchedId = id;
    }
  }

  // Fetch lyrics only when this component is mounted and trackId changes
  $effect(() => {
    if (trackId && trackId !== lastFetchedId) {
      fetchLyrics(trackId);
    }
  });

  let userScrolled = $state(false);
  let hasScrolledInitial = $state(false);

  $effect(() => {
    if (trackId) {
      userScrolled = false;
      hasScrolledInitial = false;
    }
  });

  // Auto-scroll active line into view
  let lyricsContainer: HTMLDivElement | undefined = $state();

  $effect(() => {
    if (activeIndex < 0 || !lyricsContainer || userScrolled) return;
    const el = lyricsContainer.querySelector(
      `[data-line-index="${activeIndex}"]`
    );
    if (el) {
      requestAnimationFrame(() => {
        el.scrollIntoView({
          behavior: hasScrolledInitial ? "smooth" : "auto",
          block: "center"
        });
        hasScrolledInitial = true;
      });
    }
  });

  function handleUserScroll() {
    userScrolled = true;
  }
</script>

<div class="lyrics-view">
  {#if !trackId}
    <div class="lyrics-empty">
      <p class="text-3">No track selected</p>
    </div>
  {:else if loading}
    <div class="lyrics-empty">
      <p class="text-3">Loading lyrics…</p>
    </div>
  {:else if loadError || lines.length === 0}
    <div class="lyrics-empty">
      <div class="empty-art">
        {#if track}
          <img
            src={artworkUrl(track.id)}
            alt={track.title}
            onerror={(e) => {
              (e.currentTarget as HTMLImageElement).style.display = "none";
            }}
          />
        {/if}
        <div class="empty-art-fallback">
          <Music size={48} />
        </div>
      </div>
      <p class="empty-title">{track?.title ?? "Unknown"}</p>
      <p class="empty-artist text-3">
        {track?.artist_name || track?.album_artist || "Unknown Artist"}
      </p>
      <p class="no-lyrics text-3">No lyrics available</p>
    </div>
  {:else}
    <!-- svelte-ignore a11y_no_static_element_interactions -->
    <div
      class="lyrics-body"
      bind:this={lyricsContainer}
      onwheel={handleUserScroll}
      ontouchmove={handleUserScroll}
    >
      <div class="lyrics-spacer"></div>
      {#each lines as line, i (i)}
        <button
          class="lyric-line"
          class:active={i === activeIndex}
          class:past={i < activeIndex}
          data-line-index={i}
          onclick={() => {
            seekRequest.set(line.timeMs);
            userScrolled = false;
          }}
        >
          {line.text || "♪"}
        </button>
      {/each}
      <div class="lyrics-spacer"></div>
    </div>
    {#if userScrolled}
      <button class="sync-btn" onclick={() => (userScrolled = false)}>
        Sync to Music
      </button>
    {/if}
  {/if}
</div>

<style>
  .lyrics-view {
    position: relative;
    display: flex;
    flex-direction: column;
    height: 100%;
    overflow: hidden;
  }

  .lyrics-empty {
    flex: 1;
    display: flex;
    flex-direction: column;
    align-items: center;
    justify-content: center;
    gap: 8px;
    padding: 24px;
    text-align: center;
  }

  .empty-art {
    width: 180px;
    height: 180px;
    border-radius: 12px;
    overflow: hidden;
    background: var(--surface-2);
    position: relative;
    display: flex;
    align-items: center;
    justify-content: center;
    margin-bottom: 16px;
    box-shadow: 0 8px 32px rgba(0, 0, 0, 0.15);
  }
  .empty-art img {
    width: 100%;
    height: 100%;
    object-fit: cover;
    position: relative;
    z-index: 1;
  }
  .empty-art-fallback {
    position: absolute;
    color: var(--text-3);
  }

  .empty-title {
    font-size: 20px;
    font-weight: 700;
    margin: 0;
    color: var(--text-1);
  }

  .empty-artist {
    font-size: 14px;
    margin: 0;
  }

  .no-lyrics {
    margin-top: 12px;
    font-size: 13px;
    opacity: 0.5;
  }

  .lyrics-body {
    flex: 1;
    overflow-y: auto;
    padding: 0 15%;
    scroll-behavior: smooth;
  }

  .lyrics-spacer {
    height: 40vh;
    flex-shrink: 0;
  }

  .lyric-line {
    font-size: 28px;
    font-weight: 700;
    line-height: 1.5;
    padding: 6px 0;
    margin: 0;
    color: var(--text-3);
    opacity: 0.3;
    transition:
      color 0.3s ease,
      opacity 0.3s ease,
      transform 0.3s ease;
    cursor: pointer;
    background: none;
    border: none;
    text-align: left;
    display: block;
    width: 100%;
    font-family: inherit;
  }

  .lyric-line.active {
    color: var(--text-1);
    opacity: 1;
    transform: scale(1.02);
    transform-origin: left center;
  }

  .lyric-line.past {
    opacity: 0.2;
  }

  .sync-btn {
    position: absolute;
    bottom: 32px;
    left: 50%;
    transform: translateX(-50%);
    background: var(--surface-3);
    color: var(--text-1);
    border: 1px solid var(--border);
    padding: 8px 16px;
    border-radius: 32px;
    font-size: 14px;
    font-weight: 500;
    cursor: pointer;
    box-shadow: 0 4px 12px rgba(0, 0, 0, 0.2);
    transition:
      background 0.2s ease,
      transform 0.2s ease;
    z-index: 10;
  }

  .sync-btn:hover {
    background: var(--surface-hover);
    transform: translateX(-50%) scale(1.05);
  }
</style>
