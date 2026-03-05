<script lang="ts">
  import { playerState, type Track } from "../stores/player"
  import { tracks } from "../stores/library"
  import { closePanel } from "../stores/ui"
  import { formatDuration } from "./TrackRow.svelte"
  import { serverFetch, artworkUrl, connected } from "./api"

  $: queue = $playerState.queue ?? []
  $: currentIndex = $playerState.queueIndex ?? 0
  $: nowPlayingTrack = $playerState.track

  // Resolve track IDs to full Track objects
  $: trackMap = new Map(($tracks as Track[]).map(t => [t.id, t]))
  $: upNext = queue
    .slice(currentIndex + 1)
    .map(id => trackMap.get(id))
    .filter((t): t is Track => t != null)

  function close() {
    closePanel()
  }

  async function playFromQueue(track: Track, idx: number) {
    if (!$connected) return
    const res = await serverFetch("/api/playback/desktop/play", {
      method: "POST",
      headers: { "Content-Type": "application/json" },
      body: JSON.stringify({ track_id: track.id, position_ms: 0 }),
    })
    if (res.ok) {
      playerState.update(s => ({
        ...s,
        trackId: track.id,
        track,
        queueIndex: currentIndex + 1 + idx,
        positionMs: 0,
        paused: false,
      }))
    }
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
          src="{artworkUrl(nowPlayingTrack.id)}"
          alt=""
          on:error={(e) => { e.currentTarget.style.display = 'none' }}
        />
      </div>
      <div class="track-info">
        <span class="name truncate">{nowPlayingTrack.title}</span>
        <span class="artist truncate text-3">{nowPlayingTrack.album_artist || "Unknown"}</span>
      </div>
    </div>
  {/if}

  <div class="section-label">Next up</div>
  <div class="queue-list">
    {#if upNext.length === 0}
      <p class="empty text-3">Nothing in queue</p>
    {:else}
      {#each upNext as track, i (track.id + '-' + i)}
        <button class="queue-item" on:click={() => playFromQueue(track, i)}>
          <div class="track-info">
            <span class="name truncate">{track.title}</span>
            <span class="artist truncate text-3">{track.album_artist || "Unknown"}</span>
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

  h3 { margin: 0; font-size: 16px; font-weight: 700; }

  .close-btn {
    font-size: 20px;
    color: var(--text-3);
    padding: 2px 6px;
    line-height: 1;
  }
  .close-btn:hover { color: var(--text-1); }

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
  .art-sm img { width: 100%; height: 100%; object-fit: cover; }

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
  .queue-item:hover { background: var(--surface-hover); }

  .track-info {
    display: flex;
    flex-direction: column;
    min-width: 0;
    flex: 1;
    gap: 1px;
  }

  .name { font-size: 13px; font-weight: 500; }
  .artist { font-size: 11px; }
  .dur { font-size: 11px; flex-shrink: 0; }
  .empty { padding: 8px; font-size: 13px; }
</style>
