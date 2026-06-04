import { apiFetch } from "../../api";

export interface UploadItem {
  file: File;
  status:
    | "pending"
    | "uploading"
    | "done"
    | "duplicate"
    | "unsupported"
    | "error";
  error?: string;
}

const AUDIO_EXTS = new Set([
  ".mp3",
  ".flac",
  ".ogg",
  ".opus",
  ".m4a",
  ".aac",
  ".wav",
  ".weba",
  ".aiff"
]);
export const AUDIO_ACCEPT = Array.from(AUDIO_EXTS).join(",");

/** Accept string for the general upload input (audio + .lrc). */
export const UPLOAD_ACCEPT = AUDIO_ACCEPT + ",.lrc";

export function isAudioFile(name: string): boolean {
  const dot = name.lastIndexOf(".");
  if (dot < 0) return false;
  return AUDIO_EXTS.has(name.slice(dot).toLowerCase());
}

export function isLrcFile(name: string): boolean {
  return name.toLowerCase().endsWith(".lrc");
}

export function isUploadableFile(name: string): boolean {
  return isAudioFile(name) || isLrcFile(name);
}

export async function collectFilesFromEntries(
  entries: FileSystemEntry[]
): Promise<File[]> {
  const results = await Promise.all(
    entries.map(async (entry) => {
      if (entry.isFile)
        return [await readFileEntry(entry as FileSystemFileEntry)];
      if (entry.isDirectory)
        return await readDirectoryEntries(entry as FileSystemDirectoryEntry);
      return [];
    })
  );
  return results.flat();
}

function readFileEntry(entry: FileSystemFileEntry): Promise<File> {
  return new Promise((resolve) => {
    entry.file(resolve, () => resolve(null as unknown as File));
  });
}

async function readDirectoryEntries(
  entry: FileSystemDirectoryEntry
): Promise<File[]> {
  const files: File[] = [];
  const reader = entry.createReader();
  while (true) {
    const batch = await readEntriesBatch(reader);
    if (batch.length === 0) break;
    const subFiles = await collectFilesFromEntries(batch);
    files.push(...subFiles);
  }
  return files;
}

function readEntriesBatch(
  reader: FileSystemDirectoryReader
): Promise<FileSystemEntry[]> {
  return new Promise((resolve) => {
    reader.readEntries(resolve, () => resolve([]));
  });
}

export async function processUploadItem(item: UploadItem): Promise<UploadItem> {
  if (isLrcFile(item.file.name)) {
    return processLrcUploadItem(item);
  }

  try {
    const form = new FormData();
    form.append("file", item.file);
    const r = await apiFetch("/api/library/tracks/upload", {
      method: "POST",
      body: form,
      headers: {} // delegate this to browser
    });

    if (r.status === 409) {
      return { ...item, status: "duplicate", error: "Duplicate file" };
    } else if (r.ok) {
      return { ...item, status: "done" };
    } else {
      return { ...item, status: "error", error: "Upload failed" };
    }
  } catch (e: unknown) {
    return { ...item, status: "error", error: "Upload failed" };
  }
}

/**
 * Process an .lrc file upload by posting it to the server's filename-matching
 * lyrics endpoint. The server matches the .lrc basename against track titles
 * and path basenames. Retries on 404 to handle the race condition where the
 * corresponding audio file is still being ingested until max retries are met.
 */
async function processLrcUploadItem(item: UploadItem): Promise<UploadItem> {
  const maxRetries = 5;
  const retryDelayMs = 2000;

  for (let attempt = 0; attempt <= maxRetries; attempt++) {
    try {
      const form = new FormData();
      form.append("file", item.file);

      const res = await apiFetch("/api/library/lyrics/upload", {
        method: "POST",
        body: form
      });

      if (res.ok) {
        return { ...item, status: "done" };
      }

      if (res.status === 404 && attempt < maxRetries) {
        await new Promise((r) => setTimeout(r, retryDelayMs));
        continue;
      }

      return { ...item, status: "error", error: "Upload failed" };
    } catch (e: unknown) {
      return { ...item, status: "error", error: e.message ?? "Upload failed" };
    }
  }

  return { ...item, status: "error", error: "Track not found after retries" };
}
