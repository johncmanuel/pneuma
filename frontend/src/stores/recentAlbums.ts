import { writable } from "svelte/store";
import { artworkUrl, localBase } from "../utils/api";
import { db } from "../utils/db";
import type { desktop } from "../../wailsjs/go/models";

type RecentAlbumModel = desktop.RecentAlbum;
type RecentPlaylistModel = desktop.RecentPlaylist;

export interface RecentAlbum {
  key: string;
  name: string;
  artist: string;
  isLocal: boolean;
  firstTrackId: string;
  firstLocalPath: string;
  playedAt?: number;
}

export interface RecentPlaylist {
  id: string;
  name: string;
  artworkPath: string;
  playedAt: number;
}

let _initialized = false;
let _plInitialized = false;

export const recentAlbums = writable<RecentAlbum[]>([]);
export const recentPlaylists = writable<RecentPlaylist[]>([]);

recentAlbums.subscribe((v) => {
  if (!_initialized) return;
  for (const album of v) {
    void db.setRecentAlbum(toModelAlbum(album));
  }
});

recentPlaylists.subscribe((v) => {
  if (!_plInitialized) return;
  for (const playlist of v) {
    void db.setRecentPlaylist(toModelPlaylist(playlist));
  }
});

function toModelAlbum(album: RecentAlbum): RecentAlbumModel {
  return {
    Key: album.key,
    Name: album.name,
    Artist: album.artist,
    IsLocal: album.isLocal,
    FirstTrackID: album.firstTrackId,
    FirstLocalPath: album.firstLocalPath,
    PlayedAt: album.playedAt ?? Date.now()
  };
}

function toModelPlaylist(playlist: RecentPlaylist): RecentPlaylistModel {
  return {
    ID: playlist.id,
    Name: playlist.name,
    ArtworkPath: playlist.artworkPath,
    PlayedAt: playlist.playedAt
  };
}

function fromModelAlbum(model: RecentAlbumModel): RecentAlbum {
  return {
    key: model.Key,
    name: model.Name,
    artist: model.Artist,
    isLocal: model.IsLocal,
    firstTrackId: model.FirstTrackID,
    firstLocalPath: model.FirstLocalPath,
    playedAt: model.PlayedAt
  };
}

function fromModelPlaylist(model: RecentPlaylistModel): RecentPlaylist {
  return {
    id: model.ID,
    name: model.Name,
    artworkPath: model.ArtworkPath,
    playedAt: model.PlayedAt
  };
}

export async function initRecentAlbums(): Promise<void> {
  _initialized = true;
  _plInitialized = true;
  const [albums, playlists] = await Promise.all([
    db.getRecentAlbums(),
    db.getRecentPlaylists()
  ]);
  recentAlbums.set(albums.map(fromModelAlbum));
  recentPlaylists.set(playlists.map(fromModelPlaylist));
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
