<script lang="ts" context="module">
  export interface Track {
    id: string
    path: string
    title: string
    artist_id: string
    album_id: string
    album_artist: string
    album_name: string
    genre: string
    year: number
    track_number: number
    disc_number: number
    duration_ms: number
    bitrate_kbps: number
    artwork_id: string
  }

  export function formatDuration(ms: number): string {
    const s = Math.floor(ms / 1000)
    const m = Math.floor(s / 60)
    return `${m}:${String(s % 60).padStart(2, "0")}`
  }
</script>

<script lang="ts">
  import { createEventDispatcher } from "svelte"

  export let track: Track | null = null
  export let active: boolean = false

  const dispatch = createEventDispatcher()
</script>

<button
  class="track-row"
  class:active
  on:dblclick={() => dispatch("play")}
  on:click={() => dispatch("select")}
>
  <span class="num text-3">{track?.track_number || "–"}</span>
  <span class="title truncate">{track?.title ?? "Unknown"}</span>
  <span class="artist truncate text-2">{track?.album_artist || "–"}</span>
  <span class="album truncate text-2">{track?.album_name || "–"}</span>
  <span class="duration text-3">{formatDuration(track?.duration_ms ?? 0)}</span>
</button>

<style>
  .track-row {
    display: grid;
    grid-template-columns: 32px 2fr 1fr 1fr 56px;
    align-items: center;
    gap: 0 12px;
    padding: 6px 12px;
    width: 100%;
    text-align: left;
    border-radius: var(--r-sm);
    color: var(--text-1);
    transition: background 0.1s;
  }
  .track-row:hover { background: var(--surface-hover); }
  .track-row.active { color: var(--accent); }
  .num { font-size: 12px; text-align: right; }
  .duration { font-size: 12px; text-align: right; }
</style>
