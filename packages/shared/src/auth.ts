import type { Track } from "./types";
import { storageKeys } from "./storage";

export interface UserClaims {
  user_id: string;
  username: string;
  is_admin: boolean;
  can_upload: boolean;
  can_edit: boolean;
  can_delete: boolean;
  exp: number;
}

function parseJWTPart<T>(part: string): T | null {
  try {
    return JSON.parse(atob(part.replace(/-/g, "+").replace(/_/g, "/"))) as T;
  } catch {
    return null;
  }
}

export function decodeJWT(token: string): UserClaims | null {
  try {
    const parts = token.split(".");
    if (parts.length !== 3) return null;

    const payload = parseJWTPart<Partial<UserClaims>>(parts[1]);
    if (!payload) return null;

    return {
      user_id: payload.user_id ?? "",
      username: payload.username ?? "",
      is_admin: !!payload.is_admin,
      can_upload: !!payload.can_upload,
      can_edit: !!payload.can_edit,
      can_delete: !!payload.can_delete,
      exp: payload.exp ?? 0
    };
  } catch {
    return null;
  }
}

export function decodeJWTUserId(token: string): string | null {
  if (!token) return null;

  const claims = decodeJWT(token);
  return claims?.user_id ?? null;
}

export function getOrCreateDeviceID(): string {
  const existing = globalThis.localStorage?.getItem(storageKeys.deviceId);
  if (existing) return existing;

  const next =
    crypto.randomUUID?.() ??
    `${Date.now()}-${Math.random().toString(36).slice(2)}`;

  globalThis.localStorage?.setItem(storageKeys.deviceId, next);
  return next;
}

export function isLocalID(id: string): boolean {
  return (
    id.startsWith("missing-") ||
    id.startsWith("/") ||
    /^[a-zA-Z]:[/\\]/.test(id)
  );
}

export function isRemoteTrackID(id: string): boolean {
  return !id.startsWith("/") && !/^[a-zA-Z]:[/\\]/.test(id);
}

export function isRemoteTrack(id: string): boolean {
  return isRemoteTrackID(id);
}

export function localTrackToSharedTrack(track: {
  path: string;
  title: string;
  artist: string;
  album: string;
  album_artist: string;
  genre: string;
  year: number;
  track_number: number;
  disc_number: number;
  duration_ms: number;
}): Track {
  return {
    id: track.path,
    path: track.path,
    title: track.title,
    artist_id: "",
    album_id: "",
    artist_name: track.artist,
    album_artist: track.album_artist,
    album_name: track.album,
    genre: track.genre,
    year: track.year,
    track_number: track.track_number,
    disc_number: track.disc_number,
    duration_ms: track.duration_ms,
    bitrate_kbps: 0,
    artwork_id: ""
  };
}
