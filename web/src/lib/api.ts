import { getOrCreateDeviceID, initApiClient } from "@pneuma/shared";

export {
  currentUser,
  loggedIn,
  apiFetch,
  wsBase,
  login,
  register,
  logout,
  tryAutoAuth,
  streamUrl,
  artworkUrl,
  playlistArtUrl,
  uploadPlaylistArtwork,
  generateRandomPlaylist
} from "@pneuma/shared";

export const deviceId = getOrCreateDeviceID();

export function apiBase(): string {
  return (import.meta.env?.VITE_API_BASE as string) ?? "";
}

initApiClient({
  apiBase,
  getDeviceId: () => deviceId
});
