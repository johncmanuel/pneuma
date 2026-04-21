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

export function isAudioFile(name: string): boolean {
  const dot = name.lastIndexOf(".");
  if (dot < 0) return false;
  return AUDIO_EXTS.has(name.slice(dot).toLowerCase());
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
      const body = await r.text().catch(() => "Upload failed");
      return { ...item, status: "error", error: body.slice(0, 120) };
    }
  } catch (e: unknown) {
    return { ...item, status: "error", error: e.message ?? "Network error" };
  }
}
