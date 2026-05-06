import { writable, derived, type Readable, type Writable } from "svelte/store";
import { decodeJWT, type CurrentUser, isLocalID } from "./auth";

export type PlaylistArtMode = "remote" | "local";

export interface ApiClientConfig {
  apiBase: () => string;
  getDeviceId?: () => string;
  getHeaders?: () => HeadersInit;
  onUnauthorized?: () => void;
  useCredentials?: boolean;
  getAuthToken?: () => string;
  getLocalPort?: () => number;
  isLocalTrackId?: (id: string) => boolean;
  playlistArtMode?: PlaylistArtMode;
  includeAutoQualityParam?: boolean;
  useLocationForEmptyBase?: boolean;
}

export interface ApiClient {
  currentUser: Writable<CurrentUser | null>;
  loggedIn: Readable<boolean>;
  apiFetch: (path: string, init?: RequestInit) => Promise<Response>;
  wsBase: () => string;
  localBase: () => string;
  streamUrl: (trackId: string, opts?: StreamUrlOpts) => string;
  artworkUrl: (trackId: string) => string;
  playlistArtUrl: (idOrPath: string, cacheBust?: string) => string;
  uploadPlaylistArtwork: (
    playlistId: string,
    file: File
  ) => Promise<string | null>;
  generateRandomPlaylist: (
    name: string,
    description: string,
    durationMinutes: number
  ) => Promise<{ id: string; name: string; item_count: number } | null>;
  setCurrentUserFromToken: (token?: string) => boolean;
  login: (username: string, password: string) => Promise<string | null>;
  register: (username: string, password: string) => Promise<string | null>;
  logout: () => Promise<void>;
  tryAutoAuth: () => Promise<void>;
}

export interface StreamUrlOpts {
  quality?: string;
  localPath?: string;
}

function buildClient(
  getConfig: () => ApiClientConfig,
  currentUser: Writable<CurrentUser | null>
): ApiClient {
  const loggedIn = derived(currentUser, ($u) => Boolean($u));

  function wsBase(): string {
    const config = getConfig();
    const base = config.apiBase?.() ?? "";
    if (base) return base.replace(/^http/, "ws");
    if (config.useLocationForEmptyBase ?? true) {
      const proto = location.protocol === "https:" ? "wss:" : "ws:";
      return `${proto}//${location.host}`;
    }
    return "";
  }

  function getOrigin(): string {
    const config = getConfig();
    const base = config.apiBase?.() ?? "";

    if (base) return base;
    if (config.useLocationForEmptyBase ?? true) {
      return `${location.protocol}//${location.host}`;
    }

    return "";
  }

  function localBase(): string {
    const p = getConfig().getLocalPort?.() ?? 0;
    return p ? `http://127.0.0.1:${p}` : "";
  }

  async function apiFetch(
    path: string,
    init: RequestInit = {}
  ): Promise<Response> {
    const config = getConfig();
    const headers = new Headers(init.headers);

    if (config.getHeaders) {
      const extraHeaders = config.getHeaders();
      const h = new Headers(extraHeaders);
      h.forEach((value, key) => {
        headers.set(key, value);
      });
    }

    const deviceId = config.getDeviceId?.();
    if (deviceId) {
      headers.set("X-Device-ID", deviceId);
    }

    const token = config.getAuthToken?.();
    if (token) {
      headers.set("Authorization", `Bearer ${token}`);
    }

    // no need to set the Content-Type for FormData since the browser adds the multipart boundary
    if (
      !headers.has("Content-Type") &&
      init.body &&
      !(init.body instanceof FormData)
    ) {
      headers.set("Content-Type", "application/json");
    }

    const requestInit: RequestInit = { ...init, headers };
    if (config.useCredentials ?? true) {
      requestInit.credentials = requestInit.credentials ?? "include";
    }

    const res = await fetch(`${getOrigin()}${path}`, requestInit);

    if (res.status === 401) {
      currentUser.set(null);
      config.onUnauthorized?.();
    }
    return res;
  }

  function streamUrl(trackId: string, opts?: StreamUrlOpts): string {
    const config = getConfig();
    const localPort = config.getLocalPort?.() ?? 0;
    const isLocalTrackId = (config.isLocalTrackId ?? isLocalID)(trackId);
    const quality = opts?.quality;
    const localPath = opts?.localPath;

    if (isLocalTrackId && localPort) {
      return `http://127.0.0.1:${localPort}/local/stream?path=${encodeURIComponent(trackId)}`;
    }

    if (localPath && isLocalTrackId && localPort) {
      return `http://127.0.0.1:${localPort}/local/stream?path=${encodeURIComponent(localPath)}`;
    }

    const base = getOrigin();
    const token = config.getAuthToken?.();

    if (!base) return "";
    if (token === undefined && config.getAuthToken) return "";

    const profile = quality?.trim();
    const includeAuto = config.includeAutoQualityParam ?? true;
    const params = new URLSearchParams();

    if (profile && (includeAuto || profile !== "auto")) {
      params.set("quality", profile);
    }

    if (token) {
      params.set("token", token);
    }

    const query = params.toString();
    const suffix = query ? `?${query}` : "";
    return `${base}/api/stream/tracks/${trackId}${suffix}`;
  }

  function artworkUrl(trackId: string): string {
    const config = getConfig();
    const localPort = config.getLocalPort?.() ?? 0;
    const isLocalTrackId = (config.isLocalTrackId ?? isLocalID)(trackId);

    if (isLocalTrackId && localPort) {
      return `http://127.0.0.1:${localPort}/local/art?path=${encodeURIComponent(trackId)}`;
    }

    const base = getOrigin();
    const token = config.getAuthToken?.();

    if (!base) return "";
    if (token === undefined && config.getAuthToken) return "";

    const query = token ? `?token=${encodeURIComponent(token)}` : "";
    return `${base}/api/library/tracks/${trackId}/art${query}`;
  }

  function playlistArtUrl(idOrPath: string, cacheBust?: string): string {
    if (!idOrPath) return "";

    const config = getConfig();
    const mode = config.playlistArtMode ?? "remote";

    if (mode === "local") {
      const base = localBase();
      if (!base) return "";
      return `${base}/local/playlist-art?file=${encodeURIComponent(idOrPath)}`;
    }

    const base = getOrigin();
    if (!base) return "";

    const params = new URLSearchParams();

    if (cacheBust) {
      params.set("v", cacheBust);
    }

    const token = config.getAuthToken?.();
    if (token) {
      params.set("token", token);
    }

    const query = params.toString();
    const suffix = query ? `?${query}` : "";
    return `${base}/api/playlists/${idOrPath}/art${suffix}`;
  }

  function setCurrentUserFromToken(token?: string): boolean {
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

  async function login(
    username: string,
    password: string
  ): Promise<string | null> {
    const res = await apiFetch("/api/auth/login", {
      method: "POST",
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

  async function register(
    username: string,
    password: string
  ): Promise<string | null> {
    const res = await apiFetch("/api/auth/register", {
      method: "POST",
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

  async function logout() {
    try {
      await apiFetch("/api/auth/logout", { method: "POST" });
    } catch {
      console.warn("Logout request failed");
    }

    currentUser.set(null);
  }

  /**
   * On startup, refresh the cookie-backed session and hydrate the current user.
   */
  async function tryAutoAuth() {
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

  async function uploadPlaylistArtwork(
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

  async function generateRandomPlaylist(
    name: string,
    description: string,
    durationMinutes: number
  ): Promise<{ id: string; name: string; item_count: number } | null> {
    const res = await apiFetch("/api/playlists/generate", {
      method: "POST",
      body: JSON.stringify({ name, description, duration: durationMinutes })
    });

    if (!res.ok) {
      const err = await res.text();
      console.error("Failed to generate playlist:", err);
      return null;
    }

    return await res.json();
  }

  return {
    currentUser,
    loggedIn,
    apiFetch,
    wsBase,
    localBase,
    streamUrl,
    artworkUrl,
    playlistArtUrl,
    uploadPlaylistArtwork,
    generateRandomPlaylist,
    setCurrentUserFromToken,
    login,
    register,
    logout,
    tryAutoAuth
  };
}

export function createApiClient(config: ApiClientConfig): ApiClient {
  return buildClient(() => config, writable<CurrentUser | null>(null));
}

let globalCurrentUser: Writable<CurrentUser | null> = writable(null);
let globalConfig: ApiClientConfig = {
  apiBase: () => ""
};

let globalClient: ApiClient = buildClient(
  () => globalConfig,
  globalCurrentUser
);

export function initApiClient(c: ApiClientConfig) {
  globalConfig = c;
  globalClient = buildClient(() => globalConfig, globalCurrentUser);
  currentUser.set(null);
}

export const currentUser = globalClient.currentUser;
export const loggedIn = globalClient.loggedIn;
export const apiFetch = (path: string, init?: RequestInit) =>
  globalClient.apiFetch(path, init);
export const wsBase = () => globalClient.wsBase();
export const localBase = () => globalClient.localBase();
export const streamUrl = (trackId: string, opts?: StreamUrlOpts) =>
  globalClient.streamUrl(trackId, opts);
export const artworkUrl = (trackId: string) => globalClient.artworkUrl(trackId);
export const playlistArtUrl = (idOrPath: string, cacheBust?: string) =>
  globalClient.playlistArtUrl(idOrPath, cacheBust);
export const uploadPlaylistArtwork = (playlistId: string, file: File) =>
  globalClient.uploadPlaylistArtwork(playlistId, file);
export const generateRandomPlaylist = (
  name: string,
  description: string,
  durationMinutes: number
) => globalClient.generateRandomPlaylist(name, description, durationMinutes);
export const setCurrentUserFromToken = (token?: string) =>
  globalClient.setCurrentUserFromToken(token);
export const login = (username: string, password: string) =>
  globalClient.login(username, password);
export const register = (username: string, password: string) =>
  globalClient.register(username, password);
export const logout = () => globalClient.logout();
export const tryAutoAuth = () => globalClient.tryAutoAuth();
