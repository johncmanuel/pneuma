import { writable, get } from "svelte/store";
import {
  ScanLocalFolderStream,
  ChooseLocalFolder,
  ClearLocalFolder,
  GetLocalAlbumGroups,
  GetLocalAlbumTracks,
  SearchLocalTracks,
  GetLocalTracksByPaths,
  WatchLocalFolder,
  UnwatchLocalFolder
} from "../../wailsjs/go/desktop/App";
import { EventsOn } from "../../wailsjs/runtime/runtime";
import { db } from "../utils/db";
import { playerState, type Track } from "./player";
import { addToast } from "./toasts";

export interface LocalTrack {
  path: string;
  title: string;
  artist: string;
  album: string;
  album_artist: string;
  genre: string;
  year: number;
  track_number: number;
  disc_number: number;
  duration_ms: number;
  has_artwork: boolean;
}

/**
 * Returns true if the given ID looks like a local filesystem path
 * (Unix absolute path or Windows drive letter, e.g. C:\).
 */
export function isLocalId(id: string): boolean {
  return id.startsWith("/") || /^[a-zA-Z]:[/\\]/.test(id);
}

const KEY_FOLDERS = "local_folders";

let _initialized = false;

export const localFolders = writable<string[]>([]);

localFolders.subscribe((v) => {
  if (_initialized) void db.set(KEY_FOLDERS, JSON.stringify(v));
});

export const localTracks = writable<LocalTrack[]>([]);

/** used for denoting when an initial load or full rescan is in progress. */
export const localLoading = writable(false);

/** Per-file scan progress: null when idle. */
export const scanProgress = writable<{
  folder: string;
  done: number;
  total: number;
} | null>(null);

/**
 * Remove all queue entries whose path matches removedPath exactly (single file)
 * or starts with removedPath+"/" (directory removal). Adjusts queueIndex and
 * stops playback with a toast if the currently-playing track is among the removed.
 */
function purgePathsFromQueue(removedPath: string) {
  playerState.update((s) => {
    const sep = removedPath + "/";
    const isRemoved = (id: string) => id === removedPath || id.startsWith(sep);

    const currentId = s.queue[s.queueIndex] ?? "";
    const currentRemoved = isRemoved(currentId);

    // Filter out removed tracks from both queues
    const newQueue = s.queue.filter((id) => !isRemoved(id));
    const newBase = s.baseQueue.filter((id) => !isRemoved(id));

    if (newQueue.length === 0 || currentRemoved) {
      if (currentRemoved) {
        addToast(
          `"${s.track?.title ?? currentId}" was removed from disk; playback stopped.`,
          "warning"
        );
      }
      return {
        ...s,
        queue: newQueue,
        baseQueue: newBase,
        queueIndex: 0,
        trackId: "",
        track: null,
        positionMs: 0,
        paused: true
      };
    }

    // Count how many removed entries sat before (or at) the current index to
    // compute the corrected index.
    const removedBefore = s.queue
      .slice(0, s.queueIndex)
      .filter((id) => isRemoved(id)).length;

    const newIndex = Math.max(0, s.queueIndex - removedBefore);

    return {
      ...s,
      queue: newQueue,
      baseQueue: newBase,
      queueIndex: Math.min(newIndex, newQueue.length - 1)
    };
  });
}

/**
 * If a local track is currently playing from the same album as `newTrack`,
 * insert `newTrack` into the queue at the correct disc/track-number position.
 * Works for both shuffled and unshuffled queues.
 */
async function injectTrackIntoQueue(newTrack: LocalTrack) {
  const s = get(playerState);
  if (!s.trackId || !s.track) return;

  // Only act on local-track queues (IDs are filesystem paths).
  if (!isLocalId(s.trackId)) return;

  // Match by album name + album artist (same logic as Go getLocalAlbumTracks).
  if (
    s.track.album_name !== newTrack.album ||
    s.track.album_artist !== newTrack.album_artist
  )
    return;

  const newId = newTrack.path;
  if (s.queue.includes(newId)) return;

  // Re-fetch the full sorted track list for this album so we get the canonical
  // disc/track-number ordering without having to keep metadata for every queue entry.
  let albumTracks: LocalTrack[];
  try {
    albumTracks = await fetchLocalAlbumTracks(
      newTrack.album,
      newTrack.album_artist
    );
  } catch {
    return;
  }
  if (!albumTracks.length) return;

  // Build the new baseQueue: all paths from the fresh sorted list that are
  // either already in the current baseQueue OR are the newly added track.
  const allowed = new Set([...s.baseQueue, newId]);
  const newBase = albumTracks.map((t) => t.path).filter((p) => allowed.has(p));

  playerState.update((cur) => {
    const currentId = cur.queue[cur.queueIndex] ?? "";

    let newQueue: string[];
    let newQueueIndex: number;

    // Slip the new track in right after current position so it
    // plays next naturally, without disturbing the rest of the shuffle order.
    if (cur.shuffle) {
      const insertAt = cur.queueIndex + 1;
      newQueue = [
        ...cur.queue.slice(0, insertAt),
        newId,
        ...cur.queue.slice(insertAt)
      ];
      newQueueIndex = cur.queueIndex;
    } else {
      // queue mirrors baseQueue ordering
      newQueue = newBase;
      newQueueIndex = newBase.indexOf(currentId);
      if (newQueueIndex < 0) newQueueIndex = cur.queueIndex;
    }

    return {
      ...cur,
      queue: newQueue,
      baseQueue: newBase,
      queueIndex: newQueueIndex
    };
  });
}

/** Monotonically-increasing scan generation counter. */
let scanGeneration = 0;

/** Debounce timer for batching rapid local:track:removed/added events. */
let removedRefreshTimer: ReturnType<typeof setTimeout> | null = null;

/**
 * Serial promise chain for queue injections. Ensures concurrent local:track:added
 * events are processed sequentially so each injection sees the state the
 * previous one wrote, preventing lost updates when multiple files arrive at once.
 */
let injectionChain: Promise<void> = Promise.resolve();

/**
 * Incremented each time a file is added or removed by the fsnotify watcher.
 */
export const localChangeSeq = writable(0);

export interface LocalAlbumGroup {
  key: string;
  name: string;
  artist: string;
  track_count: number;
  first_track_path: string;
}

const ALBUM_PAGE_SIZE = 50;

export const localAlbumGroups = writable<LocalAlbumGroup[]>([]);
export const localAlbumGroupsTotal = writable(0);
export const localAlbumGroupsOffset = writable(0);
export const localAlbumFilter = writable("");

/** Load a page of local album groups from Go. */
export async function loadLocalAlbumGroups(offset = 0, filter = "") {
  const dirs = get(localFolders);

  if (dirs.length === 0) {
    localAlbumGroups.set([]);
    localAlbumGroupsTotal.set(0);
    localAlbumGroupsOffset.set(0);
    return;
  }

  try {
    const result = await GetLocalAlbumGroups(
      dirs,
      filter,
      offset,
      ALBUM_PAGE_SIZE
    );
    if (result) {
      localAlbumGroups.set(result.albums ?? []);
      localAlbumGroupsTotal.set(result.total ?? 0);
      localAlbumGroupsOffset.set(offset);
    }
  } catch (e) {
    console.warn("Failed to load local album groups:", e);
  }
}

/** Append the next page of album groups. */
export async function loadMoreLocalAlbumGroups(filter = "") {
  const dirs = get(localFolders);
  const current = get(localAlbumGroupsOffset);
  const total = get(localAlbumGroupsTotal);
  const next = current + ALBUM_PAGE_SIZE;

  if (next >= total) return;

  try {
    const result = await GetLocalAlbumGroups(
      dirs,
      filter,
      next,
      ALBUM_PAGE_SIZE
    );
    if (result) {
      localAlbumGroups.update((existing) => [
        ...existing,
        ...(result.albums ?? [])
      ]);
      localAlbumGroupsOffset.set(next);
    }
  } catch (e) {
    console.warn("Failed to load more album groups:", e);
  }
}

/** Fetch tracks for a specific local album group. */
export async function fetchLocalAlbumTracks(
  albumName: string,
  albumArtist: string
): Promise<LocalTrack[]> {
  const dirs = get(localFolders);
  try {
    return (await GetLocalAlbumTracks(dirs, albumName, albumArtist)) ?? [];
  } catch (e) {
    console.warn("Failed to fetch album tracks:", e);
    return [];
  }
}

/** Search and query local tracks. */
export async function searchLocalTracksQuery(
  query: string
): Promise<LocalTrack[]> {
  const dirs = get(localFolders);
  try {
    return (await SearchLocalTracks(dirs, query)) ?? [];
  } catch (e) {
    console.warn("Local search failed:", e);
    return [];
  }
}

/** Search local album groups by name/artist filter. */
export async function searchLocalAlbumGroups(
  query: string
): Promise<LocalAlbumGroup[]> {
  const dirs = get(localFolders);
  if (dirs.length === 0) return [];

  try {
    const result = await GetLocalAlbumGroups(dirs, query, 0, 10);
    return result?.albums ?? [];
  } catch (e) {
    console.warn("Local album search failed:", e);
    return [];
  }
}

/** Resolve local tracks by exact paths (for queue). */
export async function resolveLocalTracksByPaths(
  paths: string[]
): Promise<LocalTrack[]> {
  if (paths.length === 0) return [];
  try {
    return (await GetLocalTracksByPaths(paths)) ?? [];
  } catch (e) {
    console.warn("Failed to resolve local tracks by path:", e);
    return [];
  }
}

/**
 * Load all persisted local-library state from SQLite into the Svelte stores.
 */
export async function initLocalLibrary(): Promise<void> {
  const foldersRaw = await db.get(KEY_FOLDERS);

  _initialized = true;

  let folders: string[] = [];
  try {
    if (foldersRaw) folders = JSON.parse(foldersRaw);
  } catch {
    console.warn("Failed to parse local folders from DB");
  }

  localFolders.set(folders);

  if (folders.length > 0) {
    try {
      await loadLocalAlbumGroups(0, "");
    } catch (e) {
      console.warn("Failed to load album groups from DB:", e);
    }

    // Start watching all registered folders so file removals are picked up.
    for (const dir of folders) {
      WatchLocalFolder(dir).catch((e) =>
        console.warn("WatchLocalFolder failed:", dir, e)
      );
    }
  }

  // Listen for file-removal events emitted by the Go fsnotify watcher.
  // Queue is purged immediately but album groups are debounced to batch rapid events.
  // Payload may be { path: string } (single file/dir) or { paths: string[] } (scan prune batch).
  EventsOn(
    "local:track:removed",
    (data: { path?: string; paths?: string[] }) => {
      if (data.paths) {
        for (const p of data.paths) purgePathsFromQueue(p);
      } else if (data.path) {
        purgePathsFromQueue(data.path);
      }

      if (removedRefreshTimer !== null) clearTimeout(removedRefreshTimer);

      removedRefreshTimer = setTimeout(async () => {
        removedRefreshTimer = null;
        await loadLocalAlbumGroups(0, get(localAlbumFilter));
        localChangeSeq.update((n) => n + 1);
      }, 400);
    }
  );

  // Listen for file-addition events (files copied/moved into a watched folder).
  // Queue is updated immediately if the new track belongs to the playing album;
  // album groups view is debounced to batch rapid events.
  // Injections are serialized via a promise chain so rapid bursts (e.g. two files
  // added at once) don't race against each other on playerState.
  EventsOn("local:track:added", (data: { path: string; track: LocalTrack }) => {
    if (data.track) {
      injectionChain = injectionChain
        .then(() => injectTrackIntoQueue(data.track))
        .catch(() => {
          console.warn("Failed to inject track into queue:", data.track);
        });
    }

    if (removedRefreshTimer !== null) clearTimeout(removedRefreshTimer);

    removedRefreshTimer = setTimeout(async () => {
      removedRefreshTimer = null;
      await loadLocalAlbumGroups(0, get(localAlbumFilter));
      localChangeSeq.update((n) => n + 1);
    }, 400);
  });
}

export async function addLocalFolder(): Promise<string | null> {
  try {
    const dir = await ChooseLocalFolder();
    if (!dir) return null;

    const current = get(localFolders);
    if (current.includes(dir)) return dir;

    localFolders.set([...current, dir]);

    await scanLocalFolders();

    // Start watching the new folder after the initial scan so removals are detected.
    WatchLocalFolder(dir).catch((e) =>
      console.warn("WatchLocalFolder failed:", dir, e)
    );
    return dir;
  } catch {
    return null;
  }
}

export async function removeLocalFolder(dir: string) {
  // Stop watching before clearing to prevent any weird removal events
  UnwatchLocalFolder(dir).catch((e) =>
    console.warn("UnwatchLocalFolder failed:", dir, e)
  );

  localFolders.update((dirs) => dirs.filter((d) => d !== dir));

  try {
    await ClearLocalFolder(dir);
  } catch {
    console.warn("Failed to clear local folder:", dir);
  }
  await scanLocalFolders();
}

export async function scanLocalFolders() {
  const dirs = get(localFolders);
  if (dirs.length === 0) {
    localTracks.set([]);
    return;
  }

  const myGen = ++scanGeneration;
  const existingTracks = get(localTracks);
  const hasCache = existingTracks.length > 0;

  if (!hasCache) localLoading.set(true);

  try {
    // Remove folders that are subdirectories of another listed folder.
    const sorted = [...dirs].sort();
    const effectiveDirs = sorted.filter(
      (d) =>
        !sorted.some(
          (other) => other !== d && (d + "/").startsWith(other + "/")
        )
    );

    const cancelTrackListener = EventsOn("local:track:scanned", (data: any) => {
      if (scanGeneration !== myGen) return;
      scanProgress.set({
        folder: data.folder,
        done: data.done,
        total: data.total
      });
    });

    for (const dir of effectiveDirs) {
      if (scanGeneration !== myGen) break;
      try {
        await ScanLocalFolderStream(dir);
      } catch (e) {
        console.warn("Failed to scan folder:", dir, e);
      }
    }

    cancelTrackListener();
    scanProgress.set(null);

    if (scanGeneration !== myGen) return;

    await loadLocalAlbumGroups(0, get(localAlbumFilter));
  } finally {
    localLoading.set(false);
    scanProgress.set(null);
  }
}

export function localTrackToTrack(t: LocalTrack): Track {
  return {
    id: t.path,
    path: t.path,
    title: t.title,
    artist_id: "",
    album_id: "",
    artist_name: t.artist,
    album_artist: t.album_artist,
    album_name: t.album,
    genre: t.genre,
    year: t.year,
    track_number: t.track_number,
    disc_number: t.disc_number,
    duration_ms: t.duration_ms,
    bitrate_kbps: 0,
    replay_gain_track: 0,
    artwork_id: ""
  };
}
