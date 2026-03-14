import {
  AppDBGet,
  AppDBSet,
  AppDBDelete,
  GetRecentAlbums,
  GetRecentPlaylists,
  SetRecentAlbum,
  SetRecentPlaylist
} from "../../wailsjs/go/desktop/App";
import type { desktop } from "../../wailsjs/go/models";

type RecentAlbum = desktop.RecentAlbum;
type RecentPlaylist = desktop.RecentPlaylist;

/**
 * Async key-value store backed by the app's local SQLite database.
 * Helps with interactions with database.
 *
 * All methods are fire-safe: errors are swallowed so a DB hiccup never
 * crashes the UI.  The Go backend returns "" for missing keys, which is
 * translated to `null` here so callers can distinguish "not stored" from
 * an explicitly empty string.
 *
 * When running outside Wails, the window.go bridge is absent and every call
 * falls through to the `catch` blocks, returning null or doing nothing.
 * The app will still work, but without persistence.
 */
export const db = {
  async get(key: string): Promise<string | null> {
    try {
      const v = await AppDBGet(key);
      return v === "" ? null : v;
    } catch {
      return null;
    }
  },

  async set(key: string, value: string): Promise<void> {
    try {
      await AppDBSet(key, value);
    } catch {}
  },

  async del(key: string): Promise<void> {
    try {
      await AppDBDelete(key);
    } catch {}
  },

  async getRecentAlbums(): Promise<RecentAlbum[]> {
    try {
      return await GetRecentAlbums();
    } catch {
      return [];
    }
  },

  async setRecentAlbum(album: RecentAlbum): Promise<void> {
    try {
      await SetRecentAlbum(album);
    } catch {}
  },

  async getRecentPlaylists(): Promise<RecentPlaylist[]> {
    try {
      return await GetRecentPlaylists();
    } catch {
      return [];
    }
  },

  async setRecentPlaylist(playlist: RecentPlaylist): Promise<void> {
    try {
      await SetRecentPlaylist(playlist);
    } catch {}
  }
};
