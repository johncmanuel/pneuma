export const storageKeys = {
  token: "pneuma_token",
  deviceId: "pneuma_device_id",
  session: "pneuma_session",
  localFoldersPrefix: "pneuma_local_folders",
  volume: "pneuma_volume",
  adminTracksPanel: "pneuma_admin_tracks",
  favoritesSyncEnabled: "pneuma_favorites_sync_enabled"
} as const;

export function getScopedLocalFoldersKey(userId: string | null | undefined) {
  return `${storageKeys.localFoldersPrefix}_${userId ?? "default"}`;
}
