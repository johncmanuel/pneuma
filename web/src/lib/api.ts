import { writable, derived, get } from "svelte/store"

/* ── Auth state (persisted in localStorage) ─────────────────────── */

const stored = typeof localStorage !== "undefined" ? localStorage.getItem("pneuma_token") : null

/** JWT token for the server. */
export const authToken = writable(stored ?? "")

/** Reactive flag: true when a token is present. */
export const loggedIn = derived(authToken, ($t) => $t.length > 0)

// Persist token changes to localStorage
authToken.subscribe((v) => {
  if (typeof localStorage !== "undefined") {
    if (v) localStorage.setItem("pneuma_token", v)
    else localStorage.removeItem("pneuma_token")
  }
})

/* ── Current user derived from JWT claims ───────────────────────── */

export interface UserClaims {
  user_id: string
  username: string
  is_admin: boolean
  can_upload: boolean
  can_edit: boolean
  can_delete: boolean
  exp: number
}

/** Decode JWT payload (no verification — the server does that). */
function decodeJWT(token: string): UserClaims | null {
  try {
    const parts = token.split(".")
    if (parts.length !== 3) return null
    const payload = JSON.parse(atob(parts[1].replace(/-/g, "+").replace(/_/g, "/")))
    return {
      user_id: payload.user_id ?? "",
      username: payload.username ?? "",
      is_admin: !!payload.is_admin,
      can_upload: !!payload.can_upload,
      can_edit: !!payload.can_edit,
      can_delete: !!payload.can_delete,
      exp: payload.exp ?? 0,
    }
  } catch {
    return null
  }
}

/** Reactive current user decoded from the JWT. */
export const currentUser = derived(authToken, ($t) => ($t ? decodeJWT($t) : null))

/* ── URL helpers ────────────────────────────────────────────────── */

/**
 * API base URL.
 * In production the web UI is served from the same origin as the API,
 * so we use a relative path.  During `vite dev` you can set
 * VITE_API_BASE to point at a running server (e.g. http://localhost:8989).
 */
function apiBase(): string {
  // @ts-ignore import.meta.env
  return (import.meta.env?.VITE_API_BASE as string) ?? ""
}

/** WebSocket base URL (ws:// or wss://) derived from the current page. */
export function wsBase(): string {
  const base = apiBase()
  if (base) return base.replace(/^http/, "ws")
  const proto = location.protocol === "https:" ? "wss:" : "ws:"
  return `${proto}//${location.host}`
}

/* ── Auth-aware fetch wrapper ───────────────────────────────────── */

export async function apiFetch(
  path: string,
  init: RequestInit = {},
): Promise<Response> {
  const token = get(authToken)
  const headers = new Headers(init.headers)
  if (token) headers.set("Authorization", `Bearer ${token}`)
  // Don't set Content-Type for FormData — the browser adds the multipart boundary.
  if (!headers.has("Content-Type") && init.body && !(init.body instanceof FormData)) {
    headers.set("Content-Type", "application/json")
  }
  const res = await fetch(`${apiBase()}${path}`, { ...init, headers })
  // Auto-logout on 401
  if (res.status === 401) {
    authToken.set("")
  }
  return res
}

/* ── Auth actions ───────────────────────────────────────────────── */

export async function login(username: string, password: string): Promise<string | null> {
  const res = await fetch(`${apiBase()}/api/auth/login`, {
    method: "POST",
    headers: { "Content-Type": "application/json" },
    body: JSON.stringify({ username, password }),
  })
  if (!res.ok) {
    const data = await res.json().catch(() => ({}))
    return data.message ?? "Login failed"
  }
  const data = await res.json()
  authToken.set(data.token)
  return null // success
}

export async function register(username: string, password: string): Promise<string | null> {
  const res = await fetch(`${apiBase()}/api/auth/register`, {
    method: "POST",
    headers: { "Content-Type": "application/json" },
    body: JSON.stringify({ username, password }),
  })
  if (!res.ok) {
    const data = await res.json().catch(() => ({}))
    return data.message ?? "Registration failed"
  }
  const data = await res.json()
  authToken.set(data.token)
  return null
}

export function logout() {
  authToken.set("")
}

/**
 * Attempt auto-auth for dev: tries to register admin/admin (first user becomes
 * admin), falling back to login if the account already exists.
 * Returns null on success, error string on failure.
 */
export async function tryAutoAuth(): Promise<string | null> {
  // If a token is present but missing username (old token format), drop it.
  const existing = get(authToken)
  if (existing) {
    const claims = decodeJWT(existing)
    if (!claims || !claims.username) {
      authToken.set("") // force re-auth with new token format
    } else {
      return null // valid token with correct fields
    }
  }

  // Try register first (first user = admin)
  let err = await register("admin", "admin")
  if (!err) return null

  // If user already exists, try login
  err = await login("admin", "admin")
  return err
}

/* ── Stream / artwork URL helpers ───────────────────────────────── */

export function streamUrl(trackId: string): string {
  const token = get(authToken)
  return `${apiBase()}/api/stream/tracks/${trackId}?token=${encodeURIComponent(token)}`
}

export function artworkUrl(trackId: string): string {
  const token = get(authToken)
  return `${apiBase()}/api/library/tracks/${trackId}/art?token=${encodeURIComponent(token)}`
}
