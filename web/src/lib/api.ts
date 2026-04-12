import { writable, derived, get } from "svelte/store";
import { decodeJWT, getOrCreateDeviceID, storageKeys } from "@pneuma/shared";

const TOKEN_KEY = storageKeys.token;

const stored = localStorage.getItem(TOKEN_KEY);

/** JWT token */
export const authToken = writable(stored ?? "");

export const loggedIn = derived(authToken, ($t) => $t.length > 0);

export const deviceId = getOrCreateDeviceID();

// Persist token changes to localStorage
authToken.subscribe((v) => {
  if (v) localStorage.setItem(TOKEN_KEY, v);
  else localStorage.removeItem(TOKEN_KEY);
});

/** Reactive current user decoded from the JWT. */
export const currentUser = derived(authToken, ($t) =>
  $t ? decodeJWT($t) : null
);

function apiBase(): string {
  return (import.meta.env?.VITE_API_BASE as string) ?? "";
}

/** WebSocket base URL (ws:// or wss://) derived from the current page. */
export function wsBase(): string {
  const base = apiBase();
  if (base) return base.replace(/^http/, "ws");
  const proto = location.protocol === "https:" ? "wss:" : "ws:";
  return `${proto}//${location.host}`;
}

export async function apiFetch(
  path: string,
  init: RequestInit = {}
): Promise<Response> {
  const token = get(authToken);
  const headers = new Headers(init.headers);

  if (token) headers.set("Authorization", `Bearer ${token}`);
  headers.set("X-Device-ID", deviceId);

  // Don't set Content-Type for FormData, the browser adds the multipart boundary
  if (
    !headers.has("Content-Type") &&
    init.body &&
    !(init.body instanceof FormData)
  ) {
    headers.set("Content-Type", "application/json");
  }

  const res = await fetch(`${apiBase()}${path}`, { ...init, headers });

  if (res.status === 401) {
    logout();
  }
  return res;
}

export async function login(
  username: string,
  password: string
): Promise<string | null> {
  const res = await fetch(`${apiBase()}/api/auth/login`, {
    method: "POST",
    headers: { "Content-Type": "application/json" },
    body: JSON.stringify({ username, password })
  });

  if (!res.ok) {
    const data = await res.json().catch(() => ({}));
    return data.message ?? "Login failed";
  }

  const data = await res.json();
  authToken.set(data.token);
  return null;
}

export async function register(
  username: string,
  password: string
): Promise<string | null> {
  const res = await fetch(`${apiBase()}/api/auth/register`, {
    method: "POST",
    headers: { "Content-Type": "application/json" },
    body: JSON.stringify({ username, password })
  });

  if (!res.ok) {
    const data = await res.json().catch(() => ({}));
    return data.message ?? "Registration failed";
  }

  const data = await res.json();
  authToken.set(data.token);
  return null;
}

export function logout() {
  authToken.set("");
}

/**
 * On startup, validate any stored token against the server by calling the
 * refresh endpoint. If valid, the server returns a new token, extending the
 * session.
 */
export async function tryAutoAuth() {
  const existing = get(authToken);
  if (!existing) return;

  // log out if token is malformed or missing expected claims
  const claims = decodeJWT(existing);
  if (!claims || !claims.username) {
    logout();
    return;
  }

  // attempt to refresh
  try {
    const res = await apiFetch("/api/auth/refresh", { method: "POST" });
    if (res.ok) {
      const data = await res.json();
      if (data.token) authToken.set(data.token);
    }
  } catch {
    console.warn("Auto-auth refresh failed");
  }
}

/** Short-lived stream token for <audio> elements (60s TTL). */
export async function getStreamToken(): Promise<string> {
  const res = await apiFetch("/api/auth/stream-token");
  if (res.ok) {
    const data = await res.json();
    return data.token ?? "";
  }
  return "";
}

export function streamUrl(trackId: string, token: string): string {
  const base = apiBase();
  return `${base}/api/stream/tracks/${trackId}?token=${encodeURIComponent(token)}`;
}

export function artworkUrl(trackId: string): string {
  const token = get(authToken);
  const base = apiBase();
  return `${base}/api/library/tracks/${trackId}/art?token=${encodeURIComponent(token)}`;
}

export function playlistArtUrl(playlistId: string, cacheBust?: string): string {
  const token = get(authToken);
  const base = apiBase();
  const v = cacheBust ? `&v=${encodeURIComponent(cacheBust)}` : "";
  return `${base}/api/playlists/${playlistId}/art?token=${encodeURIComponent(token)}${v}`;
}

export async function uploadPlaylistArtwork(
  playlistId: string,
  file: File
): Promise<string | null> {
  const formData = new FormData();
  formData.append("file", file);

  const res = await apiFetch(`/api/playlists/${playlistId}/artwork`, {
    method: "POST",
    body: formData
  });

  if (!res.ok) return null;

  const data = await res.json();
  return data.artwork_path ?? null;
}

export async function generateRandomPlaylist(
  name: string,
  description: string,
  durationMinutes: number
): Promise<{ id: string; name: string; item_count: number } | null> {
  const res = await apiFetch("/api/playlists/generate", {
    method: "POST",
    headers: { "Content-Type": "application/json" },
    body: JSON.stringify({ name, description, duration: durationMinutes })
  });

  if (!res.ok) {
    const err = await res.text();
    console.error("Failed to generate playlist:", err);
    return null;
  }

  return await res.json();
}
