import { writable, derived } from "svelte/store";
import { decodeJWT, type CurrentUser } from "./auth";

export const currentUser = writable<CurrentUser | null>(null);
export const loggedIn = derived(currentUser, ($u) => Boolean($u));

export interface ApiClientConfig {
  apiBase: () => string;
  getHeaders?: () => HeadersInit;
  onUnauthorized?: () => void;
}

let config: ApiClientConfig = {
  apiBase: () => ""
};

export function initApiClient(c: ApiClientConfig) {
  config = c;
}

export function wsBase(): string {
  const base = config.apiBase();
  if (base) return base.replace(/^http/, "ws");
  const proto = location.protocol === "https:" ? "wss:" : "ws:";
  return `${proto}//${location.host}`;
}

export async function apiFetch(
  path: string,
  init: RequestInit = {}
): Promise<Response> {
  const headers = new Headers(init.headers);

  if (config.getHeaders) {
    const extraHeaders = config.getHeaders();
    const h = new Headers(extraHeaders);
    h.forEach((value, key) => {
      headers.set(key, value);
    });
  }

  // no need to set the Content-Type for FormData since the browser adds the multipart boundary
  if (
    !headers.has("Content-Type") &&
    init.body &&
    !(init.body instanceof FormData)
  ) {
    headers.set("Content-Type", "application/json");
  }

  const res = await fetch(`${config.apiBase()}${path}`, {
    ...init,
    credentials: "include",
    headers
  });

  if (res.status === 401) {
    currentUser.set(null);
    if (config.onUnauthorized) config.onUnauthorized();
  }
  return res;
}

export function setCurrentUserFromToken(token?: string): boolean {
  if (!token) {
    currentUser.set(null);
    return false;
  }

  const claims = decodeJWT(token);
  if (!claims?.user_id) {
    currentUser.set(null);
    return false;
  }

  currentUser.set({
    id: claims.user_id,
    username: claims.username,
    is_admin: claims.is_admin,
    can_upload: claims.can_upload,
    can_edit: claims.can_edit,
    can_delete: claims.can_delete
  });

  return true;
}

export async function login(
  username: string,
  password: string
): Promise<string | null> {
  const res = await fetch(`${config.apiBase()}/api/auth/login`, {
    method: "POST",
    credentials: "include",
    headers: { "Content-Type": "application/json" },
    body: JSON.stringify({ username, password })
  });

  if (!res.ok) {
    const data = await res.json().catch(() => ({}));
    return data.message ?? "Login failed";
  }

  const data = (await res.json()) as { user?: CurrentUser; token?: string };
  if (data.user) {
    currentUser.set(data.user);
  } else {
    setCurrentUserFromToken(data.token);
  }

  return null;
}

export async function register(
  username: string,
  password: string
): Promise<string | null> {
  const res = await fetch(`${config.apiBase()}/api/auth/register`, {
    method: "POST",
    credentials: "include",
    headers: { "Content-Type": "application/json" },
    body: JSON.stringify({ username, password })
  });

  if (!res.ok) {
    const data = await res.json().catch(() => ({}));
    return data.message ?? "Registration failed";
  }

  const data = (await res.json()) as { user?: CurrentUser; token?: string };
  if (data.user) {
    currentUser.set(data.user);
  } else {
    setCurrentUserFromToken(data.token);
  }

  return null;
}

export async function logout() {
  try {
    await fetch(`${config.apiBase()}/api/auth/logout`, {
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

    const data = (await refreshed.json()) as { token?: string };
    setCurrentUserFromToken(data.token);
  } catch {
    console.warn("Auto-auth refresh failed");
    currentUser.set(null);
  }
}
