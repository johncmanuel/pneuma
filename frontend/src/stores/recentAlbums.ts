import { writable } from "svelte/store";
import { artworkUrl, localBase } from "../utils/api";
import { db } from "../utils/db";

export interface RecentAlbum {
  key: string;
  name: string;
  artist: string;
  isLocal: boolean;
  firstTrackId: string; // for remote artwork
  firstLocalPath: string; // for local artwork
  playedAt?: number; // unix ms, for chronological sort with playlists
}

export interface RecentPlaylist {
  id: string;
  name: string;
  artworkPath: string;
  playedAt: number;
}

const DB_KEY = "recent_albums";
const LS_KEY = "pneuma_recent_albums";
const PL_DB_KEY = "recent_playlists";

let _initialized = false;
let _plInitialized = false;

export const recentAlbums = writable<RecentAlbum[]>([]);
export const recentPlaylists = writable<RecentPlaylist[]>([]);

recentAlbums.subscribe((v) => {
  if (!_initialized) return;
  void db.set(DB_KEY, JSON.stringify(v));
});

recentPlaylists.subscribe((v) => {
  if (!_plInitialized) return;
  void db.set(PL_DB_KEY, JSON.stringify(v));
});

/** Migrate a single key from localStorage into the SQLite KV store (one-time). */
async function migrateFromLS(lsKey: string, dbKey: string): Promise<void> {
  const existing = await db.get(dbKey);
  if (existing !== null) return; // already migrated
  try {
    const raw = localStorage.getItem(lsKey);
    if (raw) await db.set(dbKey, raw);
    localStorage.removeItem(lsKey);
  } catch {}
}

/** Call once at app startup (inside initApi) before any subscribers write. */
export async function initRecentAlbums(): Promise<void> {
  await migrateFromLS(LS_KEY, DB_KEY);
  _initialized = true;
  _plInitialized = true;
  const [raw, plRaw] = await Promise.all([db.get(DB_KEY), db.get(PL_DB_KEY)]);
  recentAlbums.set(raw ? (JSON.parse(raw) as RecentAlbum[]) : []);
  recentPlaylists.set(plRaw ? (JSON.parse(plRaw) as RecentPlaylist[]) : []);
}

export function recordRecentAlbum(album: RecentAlbum) {
  recentAlbums.update((list) => {
    const filtered = list.filter((a) => a.key !== album.key);
    return [{ ...album, playedAt: Date.now() }, ...filtered];
  });
}

export function recordRecentPlaylist(pl: {
  id: string;
  name: string;
  artworkPath: string;
}) {
  recentPlaylists.update((list) => {
    const filtered = list.filter((r) => r.id !== pl.id);
    return [{ ...pl, playedAt: Date.now() }, ...filtered].slice(0, 20);
  });
}

export function getRecentAlbumArtUrl(album: RecentAlbum): string {
  if (album.isLocal && album.firstLocalPath) {
    const base = localBase();
    if (!base) return "";
    return `${base}/local/art?path=${encodeURIComponent(album.firstLocalPath)}`;
  }
  return artworkUrl(album.firstTrackId);
}

export function getRecentPlaylistArtUrl(artworkPath: string): string {
  if (!artworkPath) return "";
  const base = localBase();
  if (!base) return "";
  return `${base}/local/playlist-art?file=${encodeURIComponent(artworkPath)}`;
}
