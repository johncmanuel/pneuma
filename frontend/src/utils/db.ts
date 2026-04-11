import {
  GetRecentAlbums,
  GetRecentPlaylists,
  SetRecentAlbum,
  SetRecentPlaylist,
  ClearAllRecent
} from "../../wailsjs/go/desktop/App";
import type { desktop } from "../../wailsjs/go/models";

type RecentAlbum = desktop.RecentAlbum;
type RecentPlaylist = desktop.RecentPlaylist;

/**
 * helper functions backed by the app's local SQLite database for recent
 * albums/playlists.
 *
 * All methods are fire-safe by ensuring errors are swallowed so a DB hiccup never
 * crashes the UI.
 */
export const db = {
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
    } catch (e) {
      console.warn("Failed to save recent album:", e);
    }
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
    } catch (e) {
      console.warn("Failed to save recent playlist:", e);
    }
  },

  async clearAllRecent(): Promise<void> {
    try {
      await ClearAllRecent();
    } catch (e) {
      console.warn("Failed to clear recent records:", e);
    }
  }
};
