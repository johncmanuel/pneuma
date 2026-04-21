<script lang="ts">
  import { Music } from "@lucide/svelte";
  import type { Track } from "@pneuma/shared";

  interface Props {
    track: Track | null | undefined;
    trackArtSrc: string;
    onHideArtworkAndRememberMissing: (e: Event, trackID?: string) => void;
    onResetArtworkVisibility: (e: Event) => void;
    onJumpFromNowPlaying: () => void;
  }

  let {
    track,
    trackArtSrc,
    onHideArtworkAndRememberMissing,
    onResetArtworkVisibility,
    onJumpFromNowPlaying
  }: Props = $props();
</script>

<div class="now-playing">
  <div class="art">
    {#if track}
      {#if trackArtSrc}
        <img
          src={trackArtSrc}
          alt={track.title}
          onerror={(e) => onHideArtworkAndRememberMissing(e, track.id)}
          onload={onResetArtworkVisibility}
        />
      {/if}
      <div class="art-placeholder" style="position:absolute">
        <Music size={16} />
      </div>
    {:else}
      <div class="art-placeholder"><Music size={18} /></div>
    {/if}
  </div>
  <div class="info">
    {#if track}
      <button
        class="title truncate title-link"
        onclick={onJumpFromNowPlaying}
        title="Go to song source"
      >
        {track.title}
      </button>
      <span class="artist truncate text-2">
        {track.artist_name || track.album_artist || "Unknown Artist"}
      </span>
    {:else}
      <span class="text-3">No track selected</span>
    {/if}
  </div>
</div>

<style>
  .now-playing {
    display: flex;
    align-items: center;
    gap: 12px;
    min-width: 0;
  }
  .art {
    width: 56px;
    height: 56px;
    border-radius: 4px;
    overflow: hidden;
    flex-shrink: 0;
    background: var(--surface-2);
    display: flex;
    align-items: center;
    justify-content: center;
    position: relative;
  }
  .art img {
    width: 100%;
    height: 100%;
    object-fit: cover;
    position: relative;
    z-index: 1;
  }
  .art-placeholder {
    font-size: 24px;
    color: var(--text-3);
  }

  .info {
    display: flex;
    flex-direction: column;
    min-width: 0;
    gap: 2px;
  }
  .title {
    font-size: 13px;
    font-weight: 600;
  }
  .title-link {
    cursor: pointer;
    text-align: left;
    padding: 0;
    background: none;
    border: none;
    color: inherit;
    font: inherit;
    font-weight: 600;
  }
  .title-link:hover {
    text-decoration: underline;
  }
  .artist {
    font-size: 12px;
  }
</style>
