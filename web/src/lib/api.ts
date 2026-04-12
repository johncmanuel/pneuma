import { derived, writable } from "svelte/store";
import { getOrCreateDeviceID, type CurrentUser } from "@pneuma/shared";

export const currentUser = writable<CurrentUser | null>(null);
export const loggedIn = derived(currentUser, ($u) => Boolean($u));

export const deviceId = getOrCreateDeviceID();

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
  const headers = new Headers(init.headers);

  headers.set("X-Device-ID", deviceId);

  // Don't set Content-Type for FormData, the browser adds the multipart boundary
  if (
    !headers.has("Content-Type") &&
    init.body &&
    !(init.body instanceof FormData)
  ) {
    headers.set("Content-Type", "application/json");
  }

  const res = await fetch(`${apiBase()}${path}`, {
    ...init,
    credentials: "include",
    headers
  });

  if (res.status === 401) {
    currentUser.set(null);
  }
  return res;
}

async function hydrateCurrentUser(): Promise<boolean> {
  const res = await apiFetch("/api/auth/me");
  if (!res.ok) {
    currentUser.set(null);
    return false;
  }

  const data = (await res.json()) as { user?: CurrentUser };
  if (!data.user) {
    currentUser.set(null);
    return false;
  }

  currentUser.set(data.user);
  return true;
}

export async function login(
  username: string,
  password: string
): Promise<string | null> {
  const res = await fetch(`${apiBase()}/api/auth/login`, {
    method: "POST",
    credentials: "include",
    headers: { "Content-Type": "application/json" },
    body: JSON.stringify({ username, password })
  });

  if (!res.ok) {
    const data = await res.json().catch(() => ({}));
    return data.message ?? "Login failed";
  }

  const data = (await res.json()) as { user?: CurrentUser };
  if (data.user) {
    currentUser.set(data.user);
  } else {
    await hydrateCurrentUser();
  }

  return null;
}

export async function register(
  username: string,
  password: string
): Promise<string | null> {
  const res = await fetch(`${apiBase()}/api/auth/register`, {
    method: "POST",
    credentials: "include",
    headers: { "Content-Type": "application/json" },
    body: JSON.stringify({ username, password })
  });

  if (!res.ok) {
    const data = await res.json().catch(() => ({}));
    return data.message ?? "Registration failed";
  }

  const data = (await res.json()) as { user?: CurrentUser };
  if (data.user) {
    currentUser.set(data.user);
  } else {
    await hydrateCurrentUser();
  }

  return null;
}

export async function logout() {
  try {
    await fetch(`${apiBase()}/api/auth/logout`, {
      method: "POST",
      credentials: "include"
    });
  } catch {
    console.warn("Logout request failed");
  }

  currentUser.set(null);
}

/**
 * On startup, refresh the cookie-backed session and hydrate the current user.
 */
export async function tryAutoAuth() {
  try {
    const refreshed = await apiFetch("/api/auth/refresh", { method: "POST" });
    if (!refreshed.ok) {
      currentUser.set(null);
      return;
    }

    await hydrateCurrentUser();
  } catch {
    console.warn("Auto-auth refresh failed");
    currentUser.set(null);
  }
}

export function streamUrl(trackId: string): string {
  const base = apiBase();
  return `${base}/api/stream/tracks/${trackId}`;
}

export function artworkUrl(trackId: string): string {
  const base = apiBase();
  return `${base}/api/library/tracks/${trackId}/art`;
}

export function playlistArtUrl(playlistId: string, cacheBust?: string): string {
  const base = apiBase();
  const v = cacheBust ? `?v=${encodeURIComponent(cacheBust)}` : "";
  return `${base}/api/playlists/${playlistId}/art${v}`;
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
