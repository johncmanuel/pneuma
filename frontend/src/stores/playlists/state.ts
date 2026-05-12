import { writable, get } from "svelte/store";
import { ResolvePlaylistItems } from "../../../wailsjs/go/desktop/App";
import type { LocalPlaylistSummary, LocalPlaylistItem } from "@pneuma/shared";

export type { LocalPlaylistSummary as PlaylistSummary } from "@pneuma/shared";
export type { LocalPlaylistItem as PlaylistItem } from "@pneuma/shared";

export const playlists = writable<LocalPlaylistSummary[]>([]);
export const selectedPlaylistId = writable<string | null>(null);
export const selectedPlaylistItems = writable<LocalPlaylistItem[]>([]);
export const selectedPlaylist = writable<LocalPlaylistSummary | null>(null);
export const playlistsLoading = writable(false);

export const playingPlaylistId = writable<string | null>(null);

export function setPlayingPlaylistContext(playlistId: string | null) {
  playingPlaylistId.set(playlistId);
}

export const favoriteTrackIDs = writable<Set<string>>(new Set());
export const favoritesPlaylistId = writable<string | null>(null);
export const favoritesRemotePlaylistId = writable<string | null>(null);

export async function selectPlaylist(id: string) {
  selectedPlaylistId.set(id);
  playlistsLoading.set(true);

  try {
    const list = get(playlists);
    const summary = list.find((p) => p.id === id) ?? null;
    selectedPlaylist.set(summary);
    const items = (await ResolvePlaylistItems(id)) as
      | LocalPlaylistItem[]
      | null;
    selectedPlaylistItems.set(items ?? []);
  } catch (e) {
    console.error("selectPlaylist:", e);
  } finally {
    playlistsLoading.set(false);
  }
}
