import { writable, derived, get } from "svelte/store";
import { decodeJWT, storageKeys } from "@pneuma/shared";

export type { UserClaims } from "@pneuma/shared";

const TOKEN_KEY = storageKeys.token;

const stored = localStorage.getItem(TOKEN_KEY);

/** JWT token */
export const authToken = writable(stored ?? "");

export const loggedIn = derived(authToken, ($t) => $t.length > 0);

// Persist token changes to localStorage
authToken.subscribe((v) => {
  if (v) localStorage.setItem(TOKEN_KEY, v);
  else localStorage.removeItem(TOKEN_KEY);
});

/** Reactive current user decoded from the JWT. */
export const currentUser = derived(authToken, ($t) =>
  $t ? decodeJWT($t) : null
);

/**
 * API base URL.
 * In production, the web UI is served from the same origin as the API,
 * so use a relative path. During `vite dev` you can set
 * VITE_API_BASE to point at a running server (e.g. http://localhost:8989).
 */
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
    // log out if 401 returned, implying token is invalid/expired.
    // This'll show the login form for the user afterwards
    // authToken.set("");
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
 * session (if, not obviously show the login form)
 * This means users stay logged in across browser sessions and server restarts
 * (provided the server's JWT secret is stable, which it is after first run).
 */
export async function tryAutoAuth() {
  const existing = get(authToken);
  if (!existing) return;

  // log out if token is malformed or missing expected claims. token can be this way via
  // user tampering, token format changed, or a variety of other reasons.
  const claims = decodeJWT(existing);
  if (!claims || !claims.username) {
    // authToken.set("");
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

export function streamUrl(trackId: string): string {
  const token = get(authToken);
  return `${apiBase()}/api/stream/tracks/${trackId}?token=${encodeURIComponent(token)}`;
}

export function artworkUrl(trackId: string): string {
  const token = get(authToken);
  return `${apiBase()}/api/library/tracks/${trackId}/art?token=${encodeURIComponent(token)}`;
}
