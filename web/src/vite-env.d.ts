/// <reference types="svelte" />
/// <reference types="vite/client" />

/* ── File System Access API (drag-and-drop folder support) ────────── */

interface FileSystemEntry {
  readonly isFile: boolean
  readonly isDirectory: boolean
  readonly name: string
  readonly fullPath: string
}

interface FileSystemFileEntry extends FileSystemEntry {
  readonly isFile: true
  readonly isDirectory: false
  file(successCb: (file: File) => void, errorCb?: (err: DOMException) => void): void
}

interface FileSystemDirectoryEntry extends FileSystemEntry {
  readonly isFile: false
  readonly isDirectory: true
  createReader(): FileSystemDirectoryReader
}

interface FileSystemDirectoryReader {
  readEntries(
    successCb: (entries: FileSystemEntry[]) => void,
    errorCb?: (err: DOMException) => void,
  ): void
}

interface DataTransferItem {
  webkitGetAsEntry(): FileSystemEntry | null
}
