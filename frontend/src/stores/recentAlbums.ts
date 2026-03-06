import { writable } from "svelte/store"
import { artworkUrl, localBase } from "../utils/api"
import { db } from "../utils/db"

export interface RecentAlbum {
  key: string
  name: string
  artist: string
  isLocal: boolean
  firstTrackId: string   // for remote artwork
  firstLocalPath: string // for local artwork
}

const MAX_RECENT = 10
const DB_KEY = "recent_albums"
const LS_KEY  = "pneuma_recent_albums"

let _initialized = false

export const recentAlbums = writable<RecentAlbum[]>([])

recentAlbums.subscribe(v => {
  if (!_initialized) return
  void db.set(DB_KEY, JSON.stringify(v))
})

/** Migrate a single key from localStorage into the SQLite KV store (one-time). */
async function migrateFromLS(lsKey: string, dbKey: string): Promise<void> {
  const existing = await db.get(dbKey)
  if (existing !== null) return // already migrated
  try {
    const raw = localStorage.getItem(lsKey)
    if (raw) await db.set(dbKey, raw)
    localStorage.removeItem(lsKey)
  } catch {}
}

/** Call once at app startup (inside initApi) before any subscribers write. */
export async function initRecentAlbums(): Promise<void> {
  await migrateFromLS(LS_KEY, DB_KEY)
  _initialized = true
  const raw = await db.get(DB_KEY)
  recentAlbums.set(raw ? (JSON.parse(raw) as RecentAlbum[]) : [])
}

export function recordRecentAlbum(album: RecentAlbum) {
  recentAlbums.update(list => {
    const filtered = list.filter(a => a.key !== album.key)
    return [album, ...filtered].slice(0, MAX_RECENT)
  })
}

export function getRecentAlbumArtUrl(album: RecentAlbum): string {
  if (album.isLocal && album.firstLocalPath) {
    const base = localBase()
    if (!base) return ""
    return `${base}/local/art?path=${encodeURIComponent(album.firstLocalPath)}`
  }
  return artworkUrl(album.firstTrackId)
}
