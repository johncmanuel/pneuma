// Package scanner is the single authority for server-side library
// synchronization. It coordinates filesystem scanning, real-time file
// watching, and metadata ingestion to keep the track database in sync
// with the audio files on disk.
//
// The [Manager] is the central authority for library sync. It handles the
// heavy lifting such as parsing metadata and pruning missing tracks,
// allowing the [Scheduler] and [Watcher] to remain simple triggers that only
// coordinate when scans happen.
package scanner
