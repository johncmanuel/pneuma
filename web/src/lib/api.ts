import {
  getOrCreateDeviceID,
  initApiClient,
  apiFetch,
  type StreamQuality
} from "@pneuma/shared";

export {
  currentUser,
  loggedIn,
  apiFetch,
  wsBase,
  login,
  register,
  logout,
  tryAutoAuth
} from "@pneuma/shared";

export const deviceId = getOrCreateDeviceID();

export function apiBase(): string {
  return (import.meta.env?.VITE_API_BASE as string) ?? "";
}

initApiClient({
  apiBase,
  getHeaders: () => ({ "X-Device-ID": deviceId })
});

export function streamUrl(trackId: string, quality?: StreamQuality): string {
  const base = apiBase();
  const profile = quality?.trim();
  if (!profile) {
    return `${base}/api/stream/tracks/${trackId}`;
  }
  return `${base}/api/stream/tracks/${trackId}?quality=${encodeURIComponent(profile)}`;
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
