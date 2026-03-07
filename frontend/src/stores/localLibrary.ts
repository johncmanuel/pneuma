import { writable, get } from "svelte/store"
import { ScanLocalFolderStream, ChooseLocalFolder, GetLocalTracks, ClearLocalFolder, FindLocalDuplicates, GetLocalAlbumGroups, GetLocalAlbumTracks, SearchLocalTracks, GetLocalTracksByPaths } from "../../wailsjs/go/main/App"
import { EventsOn } from "../../wailsjs/runtime/runtime"
import { db } from "../utils/db"

/* ── Types ──────────────────────────────────────────────────────── */

export interface LocalTrack {
  path: string
  title: string
  artist: string
  album: string
  album_artist: string
  genre: string
  year: number
  track_number: number
  disc_number: number
  duration_ms: number
  has_artwork: boolean
}

export interface DuplicateGroup {
  key: string             // "title|album|album_artist" (lower-cased)
  tracks: LocalTrack[]    // 2+ copies
}

/* ── DB keys (KV table — settings only, tracks are relational now) ── */

const KEY_FOLDERS    = "local_folders"
const KEY_DISMISSED  = "dismissed_duplicates"
const KEY_AUTO_DUPE  = "auto_dupe_check"

/* ── Stores ─────────────────────────────────────────────────────── */

let _initialized = false

/** List of local folder paths the user has added. */
export const localFolders = writable<string[]>([])
localFolders.subscribe((v) => {
  if (_initialized) void db.set(KEY_FOLDERS, JSON.stringify(v))
})

/** All local tracks combined from all scanned folders. */
export const localTracks = writable<LocalTrack[]>([])

/** Loading flag — true while an initial load or full rescan is in progress. */
export const localLoading = writable(false)

/** Per-file scan progress: null when idle. */
export const scanProgress = writable<{ folder: string; done: number; total: number } | null>(null)

/** Paths the user has dismissed from the duplicates view. */
export const dismissedDuplicates = writable<Set<string>>(new Set())
dismissedDuplicates.subscribe((v) => {
  if (_initialized) void db.set(KEY_DISMISSED, JSON.stringify([...v]))
})

/** Detected duplicate groups, computed after each scan. */
export const localDuplicates = writable<DuplicateGroup[]>([])

/** True while a duplicate check is in progress. */
export const scanningDuplicates = writable(false)

/** User preference: auto-run duplicate checks on startup. */
export const autoDupeCheck = writable<boolean>(true)
autoDupeCheck.subscribe((v) => {
  if (_initialized) void db.set(KEY_AUTO_DUPE, String(v))
})

/** Monotonically-increasing scan generation counter. */
let scanGeneration = 0

/* ── Album Groups (paginated, computed in Go SQL) ──────────────────────────── */

export interface LocalAlbumGroup {
  key: string
  name: string
  artist: string
  track_count: number
  first_track_path: string
}

const ALBUM_PAGE_SIZE = 50

/** Paginated album groups — only the current page is held in memory. */
export const localAlbumGroups = writable<LocalAlbumGroup[]>([])
export const localAlbumGroupsTotal = writable(0)
export const localAlbumGroupsOffset = writable(0)
export const localAlbumFilter = writable("")

/** Load a page of local album groups from Go. */
export async function loadLocalAlbumGroups(offset = 0, filter = "") {
  const dirs = get(localFolders)
  if (dirs.length === 0) {
    localAlbumGroups.set([])
    localAlbumGroupsTotal.set(0)
    localAlbumGroupsOffset.set(0)
    return
  }
  try {
    const result = await GetLocalAlbumGroups(dirs, filter, offset, ALBUM_PAGE_SIZE)
    if (result) {
      localAlbumGroups.set(result.albums ?? [])
      localAlbumGroupsTotal.set(result.total ?? 0)
      localAlbumGroupsOffset.set(offset)
    }
  } catch (e) {
    console.warn("Failed to load local album groups:", e)
  }
}

/** Append the next page of album groups. */
export async function loadMoreLocalAlbumGroups(filter = "") {
  const dirs = get(localFolders)
  const current = get(localAlbumGroupsOffset)
  const total = get(localAlbumGroupsTotal)
  const next = current + ALBUM_PAGE_SIZE
  if (next >= total) return
  try {
    const result = await GetLocalAlbumGroups(dirs, filter, next, ALBUM_PAGE_SIZE)
    if (result) {
      localAlbumGroups.update(existing => [...existing, ...(result.albums ?? [])])
      localAlbumGroupsOffset.set(next)
    }
  } catch (e) {
    console.warn("Failed to load more album groups:", e)
  }
}

/** Fetch tracks for a specific local album group. */
export async function fetchLocalAlbumTracks(albumName: string, albumArtist: string): Promise<LocalTrack[]> {
  const dirs = get(localFolders)
  try {
    return (await GetLocalAlbumTracks(dirs, albumName, albumArtist)) ?? []
  } catch (e) {
    console.warn("Failed to fetch album tracks:", e)
    return []
  }
}

/** Search local tracks via Go LIKE query. */
export async function searchLocalTracksQuery(query: string): Promise<LocalTrack[]> {
  const dirs = get(localFolders)
  try {
    return (await SearchLocalTracks(dirs, query)) ?? []
  } catch (e) {
    console.warn("Local search failed:", e)
    return []
  }
}

/** Search local album groups by name/artist filter (non-destructive, doesn't touch main store). */
export async function searchLocalAlbumGroups(query: string): Promise<LocalAlbumGroup[]> {
  const dirs = get(localFolders)
  if (dirs.length === 0) return []
  try {
    const result = await GetLocalAlbumGroups(dirs, query, 0, 10)
    return result?.albums ?? []
  } catch (e) {
    console.warn("Local album search failed:", e)
    return []
  }
}

/** Resolve local tracks by exact paths (for queue). */
export async function resolveLocalTracksByPaths(paths: string[]): Promise<LocalTrack[]> {
  if (paths.length === 0) return []
  try {
    return (await GetLocalTracksByPaths(paths)) ?? []
  } catch (e) {
    console.warn("Failed to resolve local tracks by path:", e)
    return []
  }
}

/* ── Initialisation ─────────────────────────────────────────────── */

async function migrateFromLS(lsKey: string, dbKey: string): Promise<string | null> {
  const existing = await db.get(dbKey)
  if (existing !== null) return existing
  try {
    const lsVal = localStorage.getItem(lsKey)
    if (lsVal) {
      await db.set(dbKey, lsVal)
      localStorage.removeItem(lsKey)
      return lsVal
    }
  } catch { /* ignore */ }
  return null
}

/**
 * Load all persisted local-library state from SQLite into the Svelte stores.
 * Tracks are loaded from the relational `local_tracks` table (indexed, fast).
 */
export async function initLocalLibrary(): Promise<void> {
  const [foldersRaw, dismissedRaw, autoDupeRaw] = await Promise.all([
    migrateFromLS("pneuma_local_folders",       KEY_FOLDERS),
    migrateFromLS("pneuma_dismissed_duplicates", KEY_DISMISSED),
    migrateFromLS("pneuma_auto_dupe_check",      KEY_AUTO_DUPE),
  ])
  // Clear legacy LS caches.
  try { localStorage.removeItem("pneuma_local_tracks_cache") } catch { /* ignore */ }
  try { localStorage.removeItem("pneuma_local_dupes_cache") } catch { /* ignore */ }

  _initialized = true

  let folders: string[] = []
  try {
    if (foldersRaw) folders = JSON.parse(foldersRaw)
  } catch { /* corrupt — keep default */ }
  localFolders.set(folders)

  try {
    if (dismissedRaw) dismissedDuplicates.set(new Set(JSON.parse(dismissedRaw)))
  } catch { /* corrupt */ }

  if (autoDupeRaw !== null) autoDupeCheck.set(autoDupeRaw !== "false")

  // Hydrate the track list from the indexed relational table — instant.
  // Now we only load album groups (paginated) instead of all tracks.
  if (folders.length > 0) {
    try {
      await loadLocalAlbumGroups(0, "")
    } catch (e) {
      console.warn("Failed to load album groups from DB:", e)
    }
  }
}

/* ── Actions ────────────────────────────────────────────────────── */

export function dismissDuplicate(path: string) {
  dismissedDuplicates.update((s) => new Set([...s, path]))
  localTracks.update((tracks) => tracks.filter((t) => t.path !== path))
}

export function restoreDuplicate(path: string) {
  dismissedDuplicates.update((s) => {
    const next = new Set(s)
    next.delete(path)
    return next
  })
}

export function cancelDuplicateScan() {
  scanGeneration++
  scanningDuplicates.set(false)
  localLoading.set(false)
  scanProgress.set(null)
}

export async function addLocalFolder(): Promise<string | null> {
  try {
    const dir = await ChooseLocalFolder()
    if (!dir) return null
    const current = get(localFolders)
    if (current.includes(dir)) return dir
    localFolders.set([...current, dir])
    await scanLocalFolders()
    return dir
  } catch {
    return null
  }
}

export async function removeLocalFolder(dir: string) {
  localFolders.update((dirs) => dirs.filter((d) => d !== dir))
  try { await ClearLocalFolder(dir) } catch { /* best-effort */ }
  await scanLocalFolders()
}

/**
 * Scan all registered local folders, streaming per-file progress.
 *
 * Each folder is scanned via ScanLocalFolderStream which emits Wails events:
 *   "local:scan:start"    → { folder, total }
 *   "local:track:scanned" → { folder, done, total, track }
 *   "local:scan:done"     → { folder, count }
 *
 * Tracks are persisted to the relational local_tracks table by Go as they
 * are scanned — no JSON blob serialisation needed.
 */
export async function scanLocalFolders() {
  const dirs = get(localFolders)
  if (dirs.length === 0) {
    localTracks.set([])
    return
  }

  const myGen = ++scanGeneration

  // Show existing data immediately while the scan runs in the background.
  const existingTracks = get(localTracks)
  const hasCache = existingTracks.length > 0

  if (!hasCache)        localLoading.set(true)

  try {
    // Remove folders that are subdirectories of another listed folder.
    const sorted = [...dirs].sort()
    const effectiveDirs = sorted.filter(
      (d) => !sorted.some((other) => other !== d && (d + "/").startsWith(other + "/"))
    )

    // Set up a per-file progress listener.
    const cancelTrackListener = EventsOn("local:track:scanned", (data: any) => {
      if (scanGeneration !== myGen) return
      scanProgress.set({ folder: data.folder, done: data.done, total: data.total })
    })

    // Scan each folder sequentially (Go does the heavy lifting).
    for (const dir of effectiveDirs) {
      if (scanGeneration !== myGen) break
      try {
        await ScanLocalFolderStream(dir)
      } catch (e) {
        console.warn("Failed to scan folder:", dir, e)
      }
    }

    // Clean up event listener.
    cancelTrackListener()
    scanProgress.set(null)

    if (scanGeneration !== myGen) return

    // Refresh album groups from the database (paginated).
    await loadLocalAlbumGroups(0, get(localAlbumFilter))
  } finally {
    localLoading.set(false)
    scanProgress.set(null)
  }
}

/**
 * Check for duplicate local tracks using metadata-only SQL query.
 * Fast — all grouping done in SQLite via CTE.
 */
export async function checkLocalDuplicates() {
  const dirs = get(localFolders)
  if (dirs.length === 0) {
    localDuplicates.set([])
    return
  }

  scanningDuplicates.set(true)
  try {
    const groups = await FindLocalDuplicates(dirs)
    localDuplicates.set(
      (groups || []).map((g: any) => ({
        key: g.key,
        tracks: g.tracks as LocalTrack[],
      }))
    )
  } catch (e) {
    console.warn("Failed to check local duplicates:", e)
    localDuplicates.set([])
  } finally {
    scanningDuplicates.set(false)
  }
}

