import { writable, derived, get } from "svelte/store"
import { ScanLocalFolder, ChooseLocalFolder } from "../../wailsjs/go/main/App"

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
  fingerprint: string          // SHA-256 content hash
  acoustic_fingerprint: string // Chromaprint acoustic fingerprint
}

export interface DuplicateGroup {
  fingerprint: string       // shared fingerprint value
  kind: "exact" | "acoustic" // which method matched
  tracks: LocalTrack[]      // 2+ copies
}

/* ── Stores ─────────────────────────────────────────────────────── */

/** List of local folder paths the user has added. Persisted in localStorage. */
const storedFolders: string[] = (() => {
  try {
    const raw = localStorage.getItem("pneuma_local_folders")
    return raw ? JSON.parse(raw) : []
  } catch {
    return []
  }
})()

export const localFolders = writable<string[]>(storedFolders)

// Persist on change
localFolders.subscribe((v) => {
  try {
    localStorage.setItem("pneuma_local_folders", JSON.stringify(v))
  } catch { /* ignore */ }
})

/** All local tracks combined from all scanned folders. */
export const localTracks = writable<LocalTrack[]>([])

/** Loading flag. */
export const localLoading = writable(false)

/** localStorage key for caching scan results between sessions. */
const TRACK_CACHE_KEY = "pneuma_local_tracks_cache"

/** localStorage key for caching computed duplicate groups between sessions. */
const DUPES_CACHE_KEY = "pneuma_local_dupes_cache"

/** localStorage key for user-dismissed duplicate paths. */
const DISMISSED_DUPES_KEY = "pneuma_dismissed_duplicates"

/** Paths the user has dismissed from the duplicates view. */
const loadDismissed = (): Set<string> => {
  try {
    const raw = localStorage.getItem(DISMISSED_DUPES_KEY)
    return raw ? new Set(JSON.parse(raw)) : new Set()
  } catch { return new Set() }
}
export const dismissedDuplicates = writable<Set<string>>(loadDismissed())
dismissedDuplicates.subscribe((v) => {
  try {
    localStorage.setItem(DISMISSED_DUPES_KEY, JSON.stringify([...v]))
  } catch { /* ignore */ }
})

/** Dismiss a duplicate path — hides it from the track list. */
export function dismissDuplicate(path: string) {
  dismissedDuplicates.update((s) => new Set([...s, path]))
  // Also remove it from the live track list immediately.
  localTracks.update((tracks) => tracks.filter((t) => t.path !== path))
}

/** Restore a previously dismissed path (make it visible again). */
export function restoreDuplicate(path: string) {
  dismissedDuplicates.update((s) => {
    const next = new Set(s)
    next.delete(path)
    return next
  })
  // The track will reappear on next scan.
}

/** Detected duplicate groups, computed after each scan. */
export const localDuplicates = writable<DuplicateGroup[]>((() => {
  // Restore cached groups immediately so the Duplicates view is populated
  // on startup without waiting for the background scan to finish.
  try {
    const raw = localStorage.getItem(DUPES_CACHE_KEY)
    return raw ? JSON.parse(raw) : []
  } catch { return [] }
})())

/** True while a folder scan (including fingerprinting) is in progress.
 * Unlike localLoading, this is always set even when cache exists. */
export const scanningDuplicates = writable(false)

/** User preference: auto-run duplicate checks on startup. */
const AUTO_DUPE_KEY = "pneuma_auto_dupe_check"
const loadAutoDupe = (): boolean => {
  try {
    const raw = localStorage.getItem(AUTO_DUPE_KEY)
    return raw === null ? true : raw === "true"
  } catch { return true }
}
export const autoDupeCheck = writable<boolean>(loadAutoDupe())
autoDupeCheck.subscribe((v) => {
  try { localStorage.setItem(AUTO_DUPE_KEY, String(v)) } catch { /* ignore */ }
})

/** Monotonically-increasing scan generation counter.
 * Each new scan increments this; any in-flight scan that sees a mismatch
 * on return knows it has been superseded (or cancelled) and discards results. */
let scanGeneration = 0

/** Cancel an in-progress duplicate/folder scan.
 * Immediately hides the spinner; any still-running Go work is discarded
 * when it eventually completes. */
export function cancelDuplicateScan() {
  scanGeneration++ // invalidate the current generation
  scanningDuplicates.set(false)
  localLoading.set(false)
}

/* ── Actions ────────────────────────────────────────────────────── */

/** Add a new local folder via native directory picker. */
export async function addLocalFolder(): Promise<string | null> {
  try {
    const dir = await ChooseLocalFolder()
    if (!dir) return null
    const current = get(localFolders)
    if (current.includes(dir)) return dir // already added
    localFolders.set([...current, dir])
    // Scan immediately
    await scanLocalFolders()
    return dir
  } catch {
    return null
  }
}

/** Remove a folder from the list and rescan. */
export function removeLocalFolder(dir: string) {
  localFolders.update((dirs) => dirs.filter((d) => d !== dir))
  localStorage.removeItem(TRACK_CACHE_KEY)
  scanLocalFolders()
}

/** Scan all registered local folders and merge results.
 *
 * @param checkDuplicates - When false, track files are scanned and the
 *   track list is updated, but the expensive duplicate-group computation
 *   is skipped.  Defaults to true (full scan).  Pass `get(autoDupeCheck)`
 *   from call-sites that respect the user preference.
 */
export async function scanLocalFolders({ checkDuplicates = true }: { checkDuplicates?: boolean } = {}) {
  const dirs = get(localFolders)
  if (dirs.length === 0) {
    localTracks.set([])
    localStorage.removeItem(TRACK_CACHE_KEY)
    return
  }

  // Capture generation at start — if cancelled, the counter will have advanced.
  const myGen = ++scanGeneration

  // Restore cached results immediately so the UI is instant on startup.
  // The background scan below will refresh and overwrite the cache.
  let hasCache = false
  try {
    const cached = localStorage.getItem(TRACK_CACHE_KEY)
    if (cached) {
      localTracks.set(JSON.parse(cached))
      hasCache = true
    }
  } catch { /* ignore corrupt cache */ }

  // Only show the loading spinner if we have no cached data to display.
  if (!hasCache) localLoading.set(true)

  // Only signal the duplicates spinner when we're actually going to check.
  if (checkDuplicates) scanningDuplicates.set(true)

  try {
    // Remove folders that are subdirectories of another listed folder —
    // the parent's recursive walk already covers them.
    const sorted = [...dirs].sort()
    const effectiveDirs = sorted.filter(
      (d) => !sorted.some((other) => other !== d && (d + "/").startsWith(other + "/"))
    )

    const results: LocalTrack[] = []
    for (const dir of effectiveDirs) {
      if (scanGeneration !== myGen) return // cancelled or superseded
      try {
        const tracks = await ScanLocalFolder(dir)
        if (scanGeneration !== myGen) return // cancelled while Go was running
        if (tracks) results.push(...tracks)
      } catch (e) {
        console.warn("Failed to scan folder:", dir, e)
      }
    }

    // One final check before committing results.
    if (scanGeneration !== myGen) return

    // Deduplicate by path
    const seen = new Set<string>()
    const deduped = results.filter((t) => {
      if (seen.has(t.path)) return false
      seen.add(t.path)
      return true
    })

    // Build duplicate groups from fingerprints (only when requested).
    if (checkDuplicates) {
      const groups: DuplicateGroup[] = []
      const exactMap = new Map<string, LocalTrack[]>()
      const acousticMap = new Map<string, LocalTrack[]>()
      const inExactGroup = new Set<string>() // paths already in an exact group

      for (const t of deduped) {
        if (t.fingerprint) {
          const arr = exactMap.get(t.fingerprint) || []
          arr.push(t)
          exactMap.set(t.fingerprint, arr)
        }
        if (t.acoustic_fingerprint) {
          const arr = acousticMap.get(t.acoustic_fingerprint) || []
          arr.push(t)
          acousticMap.set(t.acoustic_fingerprint, arr)
        }
      }

      for (const [fp, tracks] of exactMap) {
        if (tracks.length >= 2) {
          groups.push({ fingerprint: fp, kind: "exact", tracks })
          tracks.forEach((t) => inExactGroup.add(t.path))
        }
      }
      for (const [fp, tracks] of acousticMap) {
        // Skip if all members are already covered by an exact-match group.
        const uncovered = tracks.filter((t) => !inExactGroup.has(t.path))
        if (tracks.length >= 2 && uncovered.length > 0) {
          groups.push({ fingerprint: fp, kind: "acoustic", tracks })
        }
      }

      localDuplicates.set(groups)

      // Persist duplicate groups for instant restore on next launch.
      try {
        localStorage.setItem(DUPES_CACHE_KEY, JSON.stringify(groups))
      } catch { /* non-fatal */ }
    }

    // Filter out dismissed paths from the visible track list.
    const dismissed = get(dismissedDuplicates)
    const visible = dismissed.size > 0
      ? deduped.filter((t) => !dismissed.has(t.path))
      : deduped

    localTracks.set(visible)

    // Persist for instant restore on next launch.
    try {
      localStorage.setItem(TRACK_CACHE_KEY, JSON.stringify(deduped))
    } catch { /* quota exceeded — non-fatal */ }
  } finally {
    localLoading.set(false)
    if (checkDuplicates) scanningDuplicates.set(false)
  }
}
