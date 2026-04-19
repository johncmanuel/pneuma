import { writable } from "svelte/store";

export const missingTrackArtIDs = writable<Record<string, true>>({});
export const missingPlaylistArtIDs = writable<Record<string, true>>({});

export function markMissingTrackArtID(trackID: string) {
  const id = trackID.trim();
  if (!id) return;

  missingTrackArtIDs.update((ids) => {
    if (ids[id]) return ids;
    return { ...ids, [id]: true };
  });
}

export function clearMissingTrackArtID(trackID: string) {
  const id = trackID.trim();
  if (!id) return;

  missingTrackArtIDs.update((ids) => {
    if (!ids[id]) return ids;

    const next = { ...ids };
    delete next[id];
    return next;
  });
}

export function resetMissingTrackArtIDs() {
  missingTrackArtIDs.set({});
}

export function markMissingPlaylistArtID(playlistID: string) {
  const id = playlistID.trim();
  if (!id) return;

  missingPlaylistArtIDs.update((ids) => {
    if (ids[id]) return ids;
    return { ...ids, [id]: true };
  });
}

export function clearMissingPlaylistArtID(playlistID: string) {
  const id = playlistID.trim();
  if (!id) return;

  missingPlaylistArtIDs.update((ids) => {
    if (!ids[id]) return ids;

    const next = { ...ids };
    delete next[id];
    return next;
  });
}
