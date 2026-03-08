<script lang="ts">
  import { searchTracks, searchAlbumGroups, clearSearch as clearSearchStore, type RemoteAlbumGroup } from "../stores/library"
  import { searchLocalTracksQuery, searchLocalAlbumGroups, fetchLocalAlbumTracks, type LocalAlbumGroup, type LocalTrack } from "../stores/localLibrary"
  import { playerState } from "../stores/player"
  import type { Track } from "../stores/player"
  import { connected, serverFetch, artworkUrl, localBase } from "../utils/api"
  import { wsSend } from "../stores/ws"
  import { pushNav } from "../stores/ui"
  import { tick } from "svelte"
  import { onDestroy } from "svelte"

  // ── Public API ──────────────────────────────────────────────────────────────
  export let query = ""
  export function focus() { inputEl?.focus(); inputEl?.select() }
  export const hasResults = () => query.trim().length >= 2

  // ── State ───────────────────────────────────────────────────────────────────
  let debounce: number
  let reqSeq = 0           // monotonic token — stale async responses are dropped
  let inputEl: HTMLInputElement
  let resultsEl: HTMLDivElement
  let focused = false
  let activeResultKey: string | null = null

  // Minimum ms between successive nav moves while a key is held down.
  const NAV_INTERVAL_MS = 80
  let lastNavAt = 0

  // ── Context menu ────────────────────────────────────────────────────────────
  let menuTrack: TaggedTrack | null = null
  let menuX = 0
  let menuY = 0
  let showMenu = false
  let closeMenuListener: (() => void) | null = null

  function portal(node: HTMLElement) {
    document.body.appendChild(node)
    return { destroy() { node.remove() } }
  }

  function onTrackContext(e: MouseEvent, track: TaggedTrack) {
    e.preventDefault()
    menuTrack = track
    menuX = e.clientX
    menuY = e.clientY
    showMenu = true
    if (closeMenuListener) window.removeEventListener("click", closeMenuListener)
    closeMenuListener = () => { showMenu = false; window.removeEventListener("click", closeMenuListener!); closeMenuListener = null }
    // defer so this very click doesn't immediately close it
    setTimeout(() => window.addEventListener("click", closeMenuListener!), 0)
  }

  function handleMenuAddToQueue() {
    if (menuTrack) addToQueue(menuTrack)
    showMenu = false
    // Clicking the portal menu moves focus outside the container, collapsing
    // results. Refocus the input so the user can keep adding songs.
    inputEl?.focus()
  }

  onDestroy(() => {
    showMenu = false
    if (closeMenuListener) window.removeEventListener("click", closeMenuListener)
  })

  interface TaggedTrack extends Track { _source: "remote" | "local" }

  let combinedResults: TaggedTrack[] = []
  let remoteAlbumResults: RemoteAlbumGroup[] = []
  let localAlbumResults: LocalAlbumGroup[] = []

  function onInput() {
    clearTimeout(debounce)
    debounce = window.setTimeout(async () => {
      const q = query.trim()
      if (q.length < 2) {
        clearSearchStore()
        combinedResults = []
        remoteAlbumResults = []
        localAlbumResults = []
        return
      }
      const id = ++reqSeq
      try {
        const [remoteResults, localResults, remoteAlbums, localAlbums] = await Promise.all([
          searchTracks(q),
          searchLocalTracksQuery(q),
          searchAlbumGroups(q),
          searchLocalAlbumGroups(q),
        ])
        // Discard results from a superseded request
        if (id !== reqSeq) return
        buildCombined(remoteResults ?? [], localResults ?? [])
        remoteAlbumResults = remoteAlbums ?? []
        localAlbumResults = localAlbums ?? []
      } catch (e) {
        console.warn("Search error:", e)
      }
    }, 300)
  }

  function buildCombined(remoteResults: Track[], localResults: LocalTrack[]) {
    const remote: TaggedTrack[] = remoteResults.map(t => ({ ...t, _source: "remote" as const }))
    const local: TaggedTrack[] = localResults
      .slice(0, 20)
      .map(t => ({
        id: t.path,
        path: t.path,
        title: t.title,
        artist_id: "",
        album_id: "",
        artist_name: t.artist,
        album_artist: t.album_artist,
        album_name: t.album,
        genre: t.genre,
        year: t.year,
        track_number: t.track_number,
        disc_number: t.disc_number,
        duration_ms: t.duration_ms,
        bitrate_kbps: 0,
        replay_gain_track: 0,
        artwork_id: "",
        _source: "local" as const,
      }))
    combinedResults = [...remote, ...local]
  }

  function clearSearch() {
    query = ""
    clearSearchStore()
    combinedResults = []
    remoteAlbumResults = []
    localAlbumResults = []
    activeResultKey = null
  }

  function localTrackToTrack(t: LocalTrack): Track {
    return {
      id: t.path, path: t.path, title: t.title,
      artist_id: "", album_id: "",
      artist_name: t.artist, album_artist: t.album_artist,
      album_name: t.album, genre: t.genre, year: t.year,
      track_number: t.track_number, disc_number: t.disc_number,
      duration_ms: t.duration_ms, bitrate_kbps: 0,
      replay_gain_track: 0, artwork_id: "",
    }
  }

  async function playTrack(track: TaggedTrack) {
    if (track._source === "local") {
      let albumTracks: Track[] = []
      try {
        const locals = await fetchLocalAlbumTracks(track.album_name ?? "", track.album_artist ?? "")
        albumTracks = locals
          .sort((a, b) => (a.disc_number ?? 0) - (b.disc_number ?? 0) || (a.track_number ?? 0) - (b.track_number ?? 0))
          .map(localTrackToTrack)
      } catch {}
      if (albumTracks.length === 0) albumTracks = combinedResults.filter(t => t._source === "local")
      const idx = albumTracks.findIndex(t => t.id === track.id)
      const queue = albumTracks.slice(Math.max(0, idx)).map(t => t.id)
      const baseQueue = albumTracks.map(t => t.id)
      playerState.update(s => ({ ...s, trackId: track.id, track, queue, baseQueue, queueIndex: 0, positionMs: 0, paused: false }))
      return
    }

    if (!$connected) return

    let albumTracks: Track[] = []
    try {
      const params = new URLSearchParams()
      params.set("album_name", track.album_name ?? "")
      if (track.album_artist) params.set("album_artist", track.album_artist)
      const r = await serverFetch(`/api/library/tracks?${params}`)
      if (r.ok) {
        const data = await r.json()
        const fetched: Track[] = Array.isArray(data) ? data : (data.tracks ?? [])
        albumTracks = fetched.sort((a, b) =>
          (a.disc_number ?? 0) - (b.disc_number ?? 0) || (a.track_number ?? 0) - (b.track_number ?? 0)
        )
      }
    } catch {}
    if (albumTracks.length === 0) albumTracks = combinedResults.filter(t => t._source === "remote")
    const idx = albumTracks.findIndex(t => t.id === track.id)
    const queue = albumTracks.slice(Math.max(0, idx)).map(t => t.id)
    const baseQueue = albumTracks.map(t => t.id)
    playerState.update(s => ({ ...s, trackId: track.id, track, queue, baseQueue, queueIndex: 0, positionMs: 0, paused: false }))
    wsSend("playback.queue", { device_id: "desktop", track_ids: queue, start_index: 0 })
    wsSend("playback.play",  { device_id: "desktop", track_id: track.id, position_ms: 0 })
  }

  function addToQueue(track: TaggedTrack) {
    playerState.update(s => {
      const insertAt = s.queueIndex + 1
      return { ...s, queue: [...s.queue.slice(0, insertAt), track.id, ...s.queue.slice(insertAt)] }
    })
  }

  function openRemoteAlbum(album: RemoteAlbumGroup) {
    pushNav({ view: "library", tab: "library", subTab: "albums", albumKey: album.key })
    clearSearch()
  }

  function openLocalAlbum(album: LocalAlbumGroup) {
    pushNav({ view: "library", tab: "local", subTab: "albums", albumKey: album.key })
    clearSearch()
  }

  function localAlbumArtUrl(album: LocalAlbumGroup): string {
    const base = localBase()
    if (!base || !album.first_track_path) return ""
    return `${base}/local/art?path=${encodeURIComponent(album.first_track_path)}`
  }

  $: hasAlbumResults = remoteAlbumResults.length > 0 || localAlbumResults.length > 0
  $: hasTrackResults = combinedResults.length > 0
  $: hasAnyResults = hasAlbumResults || hasTrackResults
  $: showResults = focused && query.trim().length >= 2

  // After results refresh, restore focus to the same item if the user was
  // navigating and it still exists; otherwise release the active key.
  $: if (combinedResults || remoteAlbumResults || localAlbumResults) {
    const key = activeResultKey
    if (key) {
      tick().then(() => {
        const el = resultsEl?.querySelector(`[data-result-key="${CSS.escape(key)}"]`) as HTMLElement | null
        if (el) { el.focus({ preventScroll: true }); scrollResultIntoView(el) }
        else activeResultKey = null
      })
    }
  }

  // ── Focus-driven navigation ──────────────────────────────────────────────────

  /** All navigable result buttons in the dropdown. */
  function resultButtons(): HTMLElement[] {
    if (!resultsEl) return []
    return Array.from(resultsEl.querySelectorAll<HTMLElement>('button[data-result-key]'))
  }

  /** Scroll a button into view within the results container only (no page scroll). */
  function scrollResultIntoView(el: HTMLElement) {
    if (!resultsEl) return
    const top = el.offsetTop
    const bottom = top + el.offsetHeight
    if (top < resultsEl.scrollTop) {
      resultsEl.scrollTop = top
    } else if (bottom > resultsEl.scrollTop + resultsEl.clientHeight) {
      resultsEl.scrollTop = bottom - resultsEl.clientHeight
    }
  }

  function navDown() {
    const now = Date.now()
    if (now - lastNavAt < NAV_INTERVAL_MS) return
    lastNavAt = now
    const btns = resultButtons()
    if (!btns.length) return
    const idx = btns.indexOf(document.activeElement as HTMLElement)
    const next = idx < btns.length - 1 ? btns[idx + 1] : btns[btns.length - 1]
    next.focus({ preventScroll: true })
    scrollResultIntoView(next)
  }

  function navUp() {
    const now = Date.now()
    if (now - lastNavAt < NAV_INTERVAL_MS) return
    lastNavAt = now
    const btns = resultButtons()
    if (!btns.length) return
    const idx = btns.indexOf(document.activeElement as HTMLElement)
    if (idx === 0) {
      inputEl?.focus()  // back to search input from first result
    } else if (idx > 0) {
      const prev = btns[idx - 1]
      prev.focus({ preventScroll: true })
      scrollResultIntoView(prev)
    }
    // idx === -1 means input is already focused — do nothing
  }

  function activateFocused() {
    const active = document.activeElement as HTMLElement | null
    if (active?.dataset?.resultKey) { active.click(); return }
    // Nothing focused yet — activate the first result
    resultButtons()[0]?.click()
  }

  /** Single keydown handler on the container so input and result buttons share it. */
  function handleKeydown(e: KeyboardEvent) {
    if (e.key === "ArrowDown") { e.preventDefault(); navDown() }
    else if (e.key === "ArrowUp") { e.preventDefault(); navUp() }
    else if (e.key === "Enter") { e.preventDefault(); activateFocused() }
    else if (e.key === "Escape") { clearSearch(); inputEl?.blur() }
  }

  function handleContainerFocusOut(e: FocusEvent) {
    // If the context menu is open the focus departure is intentional and
    // temporary (menu is portalled outside the container). Don't collapse.
    if (showMenu) return
    if (!(e.currentTarget as HTMLElement).contains(e.relatedTarget as Node)) {
      focused = false
    }
  }
</script>

<!-- svelte-ignore a11y_no_static_element_interactions: would like to fix this later -->
<div
  class="search-container"
  on:focusin={() => focused = true}
  on:focusout={handleContainerFocusOut}
  on:keydown={handleKeydown}
>
  <div class="search-bar">
    <svg class="search-icon" viewBox="0 0 24 24" width="16" height="16" fill="none" stroke="currentColor" stroke-width="2">
      <circle cx="11" cy="11" r="8"/><path d="M21 21l-4.35-4.35"/>
    </svg>
    <input
      type="search"
      placeholder="Search tracks, artists, albums…"
      bind:value={query}
      bind:this={inputEl}
      on:input={onInput}
    />
    {#if query.length > 0}
      <button class="clear-btn" on:click={clearSearch}>&times;</button>
    {/if}
  </div>

  {#if showResults}
    <div class="search-results" bind:this={resultsEl}>
      {#if hasAnyResults}
        {#if hasAlbumResults}
          <p class="section-label">Albums</p>
          {#each localAlbumResults as album (album.key + "-local")}
            {@const key = album.key + "-local"}
            <button
              class="album-row"
              data-result-key={key}
              on:click={() => openLocalAlbum(album)}
              on:focus={() => { activeResultKey = key }}
            >
              <div class="album-thumb">
                <img src={localAlbumArtUrl(album)} alt="" on:error={(e) => { (e.currentTarget as HTMLImageElement).style.display = 'none' }} />
                <span class="album-thumb-ph">♫</span>
              </div>
              <div class="album-info">
                <span class="album-name">{album.name || "Unorganized"}</span>
                <span class="album-meta">{album.artist || "Unknown Artist"} · {album.track_count} tracks</span>
              </div>
            </button>
          {/each}
          {#each remoteAlbumResults as album (album.key + "-remote")}
            {@const key = album.key + "-remote"}
            <button
              class="album-row"
              data-result-key={key}
              on:click={() => openRemoteAlbum(album)}
              on:focus={() => { activeResultKey = key }}
            >
              <div class="album-thumb">
                <img src={artworkUrl(album.first_track_id)} alt="" on:error={(e) => { (e.currentTarget as HTMLImageElement).style.display = 'none' }} />
                <span class="album-thumb-ph">♫</span>
              </div>
              <div class="album-info">
                <span class="album-name">{album.name || "Unorganized"}</span>
                <span class="album-meta">{album.artist || "Unknown Artist"} · {album.track_count} tracks</span>
              </div>
            </button>
          {/each}
        {/if}

        {#if hasTrackResults}
          {#if hasAlbumResults}<p class="section-label">Tracks</p>{/if}
          {#each combinedResults as track (track._source + ':' + track.id)}
            {@const key = track._source + ':' + track.id}
            <button
              class="track-row"
              class:active={$playerState.trackId === track.id}
              data-result-key={key}
              on:click={() => playTrack(track)}
              on:contextmenu={(e) => onTrackContext(e, track)}
              on:focus={() => { activeResultKey = key }}
            >
              <span class="track-title">{track.title ?? "Unknown"}</span>
              <span class="track-artist">{track.artist_name || track.album_artist || ""}</span>
            </button>
          {/each}
        {/if}
      {:else}
        <p class="no-results">No results for "{query}"</p>
      {/if}
    </div>
  {/if}
</div>

{#if showMenu}
  <div class="sr-ctx-menu" use:portal style="left:{menuX}px;top:{menuY}px">
    <button on:click={handleMenuAddToQueue}>Add to queue</button>
  </div>
{/if}

<style>
  .search-container {
    position: relative;
    width: 100%;
  }

  .search-bar {
    display: flex;
    align-items: center;
    gap: 8px;
    background: var(--surface);
    border: 1px solid var(--border);
    border-radius: 20px;
    padding: 6px 14px;
    max-width: 420px;
    width: 100%;
    transition: border-color 0.15s;
  }
  .search-bar:focus-within { border-color: var(--accent); }
  .search-icon { color: var(--text-3); flex-shrink: 0; }
  input {
    flex: 1;
    background: none;
    border: none;
    color: var(--fg);
    font-size: 13px;
    outline: none;
    padding: 0;
  }
  input::placeholder { color: var(--text-3); }
  input[type="search"]::-webkit-search-cancel-button { display: none; }
  .clear-btn {
    background: none;
    border: none;
    color: var(--text-3);
    font-size: 18px;
    cursor: pointer;
    padding: 0 2px;
    line-height: 1;
  }
  .clear-btn:hover { color: var(--fg); }

  .search-results {
    position: absolute;
    top: 100%;
    left: 0;
    right: 0;
    max-height: calc(100vh - 120px);
    overflow-y: auto;
    overscroll-behavior: contain;
    background: var(--surface);
    border: 1px solid var(--border);
    border-radius: 8px;
    margin-top: 4px;
    box-shadow: 0 8px 24px rgba(0,0,0,0.4);
    z-index: 100;
  }

  .no-results {
    padding: 16px;
    color: var(--text-3);
    font-size: 13px;
  }

  .section-label {
    font-size: 10px;
    font-weight: 700;
    text-transform: uppercase;
    letter-spacing: 0.08em;
    color: var(--text-3);
    padding: 10px 14px 4px;
    margin: 0;
  }

  .album-row {
    display: flex;
    align-items: center;
    gap: 10px;
    width: 100%;
    padding: 6px 14px;
    background: none;
    border: none;
    color: inherit;
    cursor: pointer;
    text-align: left;
    outline: none;
  }
  .album-row:hover, .album-row:focus { background: var(--surface-hover); outline: none; }

  .album-thumb {
    width: 36px;
    height: 36px;
    border-radius: 4px;
    background: var(--surface-2);
    flex-shrink: 0;
    overflow: hidden;
    position: relative;
    display: flex;
    align-items: center;
    justify-content: center;
  }
  .album-thumb img { position: absolute; width: 100%; height: 100%; object-fit: cover; z-index: 1; }
  .album-thumb-ph { font-size: 14px; color: var(--text-3); }

  .album-info { display: flex; flex-direction: column; min-width: 0; flex: 1; gap: 1px; }
  .album-name { font-size: 13px; font-weight: 600; white-space: nowrap; overflow: hidden; text-overflow: ellipsis; }
  .album-meta { font-size: 11px; color: var(--text-3); white-space: nowrap; overflow: hidden; text-overflow: ellipsis; }

  .track-row {
    display: flex;
    flex-direction: column;
    gap: 2px;
    width: 100%;
    padding: 7px 14px;
    background: none;
    border: none;
    color: inherit;
    cursor: pointer;
    text-align: left;
    outline: none;
  }
  .track-row:hover, .track-row:focus, .track-row.active { background: var(--surface-hover); outline: none; }
  .track-row.active .track-title, .track-row:focus .track-title { color: var(--accent); }
  .track-title { font-size: 13px; font-weight: 500; white-space: nowrap; overflow: hidden; text-overflow: ellipsis; }
  .track-artist { font-size: 11px; color: var(--text-3); white-space: nowrap; overflow: hidden; text-overflow: ellipsis; }

  :global(.sr-ctx-menu) {
    position: fixed;
    z-index: 9999;
    background: var(--surface-2);
    border: 1px solid var(--border);
    border-radius: var(--r-md, 6px);
    padding: 4px 0;
    box-shadow: 0 8px 24px rgba(0,0,0,0.5);
    min-width: 160px;
  }
  :global(.sr-ctx-menu button) {
    display: block;
    width: 100%;
    text-align: left;
    padding: 8px 14px;
    font-size: 13px;
    color: var(--text-1);
    background: none;
    border: none;
    cursor: pointer;
  }
  :global(.sr-ctx-menu button:hover) { background: var(--surface-hover); }
</style>
