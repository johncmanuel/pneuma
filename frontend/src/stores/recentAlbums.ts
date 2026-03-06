import { writable } from "svelte/store"
import { artworkUrl, localBase } from "../lib/api"

export interface RecentAlbum {
  key: string
  name: string
  artist: string
  isLocal: boolean
  firstTrackId: string   // for remote artwork
  firstLocalPath: string // for local artwork
}

const MAX_RECENT = 10
const RECENT_ALBUMS_KEY = "pneuma_recent_albums"

const stored: RecentAlbum[] = (() => {
  try {
    const raw = localStorage.getItem(RECENT_ALBUMS_KEY)
    return raw ? JSON.parse(raw) : []
  } catch { return [] }
})()

export const recentAlbums = writable<RecentAlbum[]>(stored)

recentAlbums.subscribe(v => {
  try { localStorage.setItem(RECENT_ALBUMS_KEY, JSON.stringify(v)) } catch {}
})

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
