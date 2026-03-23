import { writable, derived, get } from "svelte/store";

const TOKEN_KEY = "pneuma_token";

const stored =
  typeof localStorage !== "undefined" ? localStorage.getItem(TOKEN_KEY) : null;

/** JWT token */
export const authToken = writable(stored ?? "");

export const loggedIn = derived(authToken, ($t) => $t.length > 0);

// Persist token changes to localStorage
authToken.subscribe((v) => {
  if (typeof localStorage !== "undefined") {
    if (v) localStorage.setItem(TOKEN_KEY, v);
    else localStorage.removeItem(TOKEN_KEY);
  }
});

export interface UserClaims {
  user_id: string;
  username: string;
  is_admin: boolean;
  can_upload: boolean;
  can_edit: boolean;
  can_delete: boolean;
  exp: number;
}

/** Decode and read JWT payload. Offload verification to the server. */
function decodeJWT(token: string): UserClaims | null {
  try {
    const parts = token.split(".");
    if (parts.length !== 3) return null;
    const payload = JSON.parse(
      atob(parts[1].replace(/-/g, "+").replace(/_/g, "/"))
    );
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
