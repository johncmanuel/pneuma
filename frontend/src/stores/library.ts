import { writable } from "svelte/store"
import type { Track } from "./player"
import { apiBase } from "../lib/api"

export interface Album {
  id: string; title: string; artist_id: string
  year: number; artwork_id: string
  artist_name?: string
}

export const tracks   = writable<Track[]>([])
export const albums   = writable<Album[]>([])
export const loading  = writable(false)
export const searchResults = writable<Track[]>([])

export async function loadTracks() {
  // Only show loading spinner on initial fetch (empty store).
  let isEmpty = true
  tracks.subscribe(v => { isEmpty = v.length === 0 })()
  if (isEmpty) loading.set(true)
  try {
    const r = await fetch(`${apiBase()}/api/library/tracks`)
    const data: Track[] = await r.json()
    // Deduplicate by track ID (guards against scanner/watcher race)
    const seen = new Set<string>()
    tracks.set(data.filter(t => {
      if (seen.has(t.id)) return false
      seen.add(t.id)
      return true
    }))
  } finally { loading.set(false) }
}

export async function loadAlbums() {
  const r = await fetch(`${apiBase()}/api/library/albums`)
  albums.set(await r.json())
}

export async function searchTracks(q: string) {
  const r = await fetch(`${apiBase()}/api/library/search?q=${encodeURIComponent(q)}`)
  const results = await r.json()
  searchResults.set(results ?? [])
}

export async function triggerScan() {
  await fetch(`${apiBase()}/api/library/scan`, { method: "POST" })
}
