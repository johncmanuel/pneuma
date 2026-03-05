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
  scanLocalFolders()
}

/** Scan all registered local folders and merge results. */
export async function scanLocalFolders() {
  const dirs = get(localFolders)
  if (dirs.length === 0) {
    localTracks.set([])
    return
  }
  localLoading.set(true)
  try {
    const results: LocalTrack[] = []
    for (const dir of dirs) {
      try {
        const tracks = await ScanLocalFolder(dir)
        if (tracks) results.push(...tracks)
      } catch (e) {
        console.warn("Failed to scan folder:", dir, e)
      }
    }
    // Deduplicate by path
    const seen = new Set<string>()
    localTracks.set(
      results.filter((t) => {
        if (seen.has(t.path)) return false
        seen.add(t.path)
        return true
      }),
    )
  } finally {
    localLoading.set(false)
  }
}
