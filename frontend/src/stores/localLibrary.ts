import { writable, get } from "svelte/store"
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

/** Scan all registered local folders and merge results. */
export async function scanLocalFolders() {
  const dirs = get(localFolders)
  if (dirs.length === 0) {
    localTracks.set([])
    localStorage.removeItem(TRACK_CACHE_KEY)
    return
  }

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

  try {
    // Remove folders that are subdirectories of another listed folder —
    // the parent's recursive walk already covers them.
    const sorted = [...dirs].sort()
    const effectiveDirs = sorted.filter(
      (d) => !sorted.some((other) => other !== d && (d + "/").startsWith(other + "/"))
    )

    const results: LocalTrack[] = []
    for (const dir of effectiveDirs) {
      try {
        const tracks = await ScanLocalFolder(dir)
        if (tracks) results.push(...tracks)
      } catch (e) {
        console.warn("Failed to scan folder:", dir, e)
      }
    }

    // Deduplicate by path
    const seen = new Set<string>()
    const deduped = results.filter((t) => {
      if (seen.has(t.path)) return false
      seen.add(t.path)
      return true
    })

    localTracks.set(deduped)

    // Persist for instant restore on next launch.
    try {
      localStorage.setItem(TRACK_CACHE_KEY, JSON.stringify(deduped))
    } catch { /* quota exceeded — non-fatal */ }
  } finally {
    localLoading.set(false)
  }
}
