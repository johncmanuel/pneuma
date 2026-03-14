<script lang="ts">
  import { playerState, type Track, isRemoteTrack } from "../stores/player";
  import { fetchTracksByIDs } from "../stores/library";
  import { resolveLocalTracksByPaths, isLocalId } from "../stores/localLibrary";
  import { closePanel } from "../stores/ui";
  import { formatDuration } from "./TrackRow.svelte";
  import { artworkUrl, connected } from "../utils/api";
  import { wsSend } from "../stores/ws";
  import { addToast } from "../stores/toasts";

  $: queue = $playerState.queue ?? [];
  $: currentIndex = $playerState.queueIndex ?? 0;
  $: nowPlayingTrack = $playerState.track;

  // Cached map of track ID → Track for queue resolution.
  // Populated lazily — only fetches IDs not already in the cache.
  const trackCache = new Map<string, Track>();
  let upNext: Track[] = [];
  let resolving = false;

  // When the queue changes, resolve any new IDs from the backend.
  $: if (queue.length > 0 || currentIndex >= 0) {
    resolveQueue(queue, currentIndex);
  }

  async function resolveQueue(q: string[], idx: number) {
    const ids = q.slice(idx + 1);
    if (ids.length === 0) {
      upNext = [];
      return;
    }

    // Split IDs into cached vs uncached
    const uncachedRemote: string[] = [];
    const uncachedLocal: string[] = [];
    for (const id of ids) {
      if (!trackCache.has(id)) {
        // Local tracks use filesystem paths as IDs (contain /)
        if (id.includes("/")) {
          uncachedLocal.push(id);
        } else {
          uncachedRemote.push(id);
        }
      }
    }

    // Fetch uncached tracks from the backend
    if (uncachedRemote.length > 0 || uncachedLocal.length > 0) {
      resolving = true;
      try {
        const [remoteTracks, localTracks] = await Promise.all([
          uncachedRemote.length > 0 ? fetchTracksByIDs(uncachedRemote) : [],
          uncachedLocal.length > 0
            ? resolveLocalTracksByPaths(uncachedLocal)
            : []
        ]);
        for (const t of remoteTracks) {
          trackCache.set(t.id, t);
        }
        for (const lt of localTracks) {
          trackCache.set(lt.path, {
            id: lt.path,
            path: lt.path,
            title: lt.title,
            artist_id: "",
            album_id: "",
            artist_name: lt.artist,
            album_artist: lt.album_artist,
            album_name: lt.album,
            genre: lt.genre,
            year: lt.year,
            track_number: lt.track_number,
            disc_number: lt.disc_number,
            duration_ms: lt.duration_ms,
            bitrate_kbps: 0,
            replay_gain_track: 0,
            artwork_id: ""
          });
        }
      } finally {
        resolving = false;
      }
    }

    // Build the resolved list from cache
    upNext = ids
      .map((id) => trackCache.get(id))
      .filter((t): t is Track => t != null);
  }

  function close() {
    closePanel();
  }

  function playFromQueue(track: Track, idx: number) {
    const newIndex = currentIndex + 1 + idx;
    const isLocalTrack = isLocalId(track.id);

    // Don't play offline tracks - skip to next available
    if (!isLocalTrack && isRemoteTrack(track.id) && !$connected) {
      addToast("Cannot play offline track", "warning");
      return;
    }

    playerState.update((s) => ({
      ...s,
      trackId: track.id,
      track,
      queueIndex: newIndex,
      positionMs: 0,
      paused: false
    }));
    if (!isLocalTrack && $connected)
      wsSend("playback.play", {
        track_id: track.id,
        position_ms: 0
      });
  }
</script>

<aside class="queue-panel">
  <div class="queue-header">
    <h3>Queue</h3>
    <button class="close-btn" on:click={close} title="Close">&times;</button>
  </div>

  {#if nowPlayingTrack}
    <div class="section-label">Now playing</div>
    <div class="now-playing-item">
      <div class="art-sm">
        <img
          src={artworkUrl(nowPlayingTrack.id)}
          alt=""
          on:error={(e) => {
            // may find a better way to do this but this is temporary
            (e.currentTarget as HTMLImageElement).style.display = "none";
          }}
        />
      </div>
      <div class="track-info">
        <span class="name truncate">{nowPlayingTrack.title}</span>
        <span class="artist truncate text-3"
          >{nowPlayingTrack.artist_name ||
            nowPlayingTrack.album_artist ||
            "Unknown"}</span
        >
      </div>
    </div>
  {/if}

  <div class="section-label">Next up</div>
  <div class="queue-list">
    {#if upNext.length === 0}
      <p class="empty text-3">Nothing in queue</p>
    {:else}
      {#each upNext as track, i (track.id + "-" + i)}
        <button class="queue-item" on:click={() => playFromQueue(track, i)}>
          <div class="track-info">
            <span class="name truncate">{track.title}</span>
            <span class="artist truncate text-3"
              >{track.artist_name || track.album_artist || "Unknown"}</span
            >
          </div>
          <span class="dur text-3">{formatDuration(track.duration_ms)}</span>
        </button>
      {/each}
    {/if}
  </div>
</aside>

<style>
  .queue-panel {
    display: flex;
    flex-direction: column;
    background: var(--surface);
    border-left: 1px solid var(--border);
    height: 100%;
    overflow: hidden;
  }

  .queue-header {
    display: flex;
    align-items: center;
    justify-content: space-between;
    padding: 16px;
    flex-shrink: 0;
  }

  h3 {
    margin: 0;
    font-size: 16px;
    font-weight: 700;
  }

  .close-btn {
    font-size: 20px;
    color: var(--text-3);
    padding: 2px 6px;
    line-height: 1;
  }
  .close-btn:hover {
    color: var(--text-1);
  }

  .section-label {
    font-size: 11px;
    text-transform: uppercase;
    letter-spacing: 0.06em;
    color: var(--text-3);
    padding: 8px 16px 4px;
    font-weight: 600;
  }

  .now-playing-item {
    display: flex;
    align-items: center;
    gap: 10px;
    padding: 8px 16px;
    background: var(--surface-hover);
    border-radius: 4px;
    margin: 0 8px 8px;
  }

  .art-sm {
    width: 40px;
    height: 40px;
    border-radius: 4px;
    overflow: hidden;
    flex-shrink: 0;
    background: var(--surface-2);
  }
  .art-sm img {
    width: 100%;
    height: 100%;
    object-fit: cover;
  }

  .queue-list {
    flex: 1;
    overflow-y: auto;
    padding: 0 8px;
  }

  .queue-item {
    display: flex;
    align-items: center;
    gap: 8px;
    width: 100%;
    text-align: left;
    padding: 6px 8px;
    border-radius: 4px;
  }
  .queue-item:hover {
    background: var(--surface-hover);
  }

  .track-info {
    display: flex;
    flex-direction: column;
    min-width: 0;
    flex: 1;
    gap: 1px;
  }

  .name {
    font-size: 13px;
    font-weight: 500;
  }
  .artist {
    font-size: 11px;
  }
  .dur {
    font-size: 11px;
    flex-shrink: 0;
  }
  .empty {
    padding: 8px;
    font-size: 13px;
  }
</style>
