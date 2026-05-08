package desktop

import (
	"context"
	"log/slog"
	"os"
	"path/filepath"
	"strings"

	"pneuma/internal/media"

	"github.com/dhowden/tag"
	wailsruntime "github.com/wailsapp/wails/v2/pkg/runtime"
)

// Scanner traverses the local file system, parses audio metadata, and
// hands each track to an AppStore via a TrackConsumer callback.
// It also emits Wails events so the frontend can display scan progress.
type Scanner struct {
	ctx   context.Context
	store *AppStore
}

// NewScanner creates a Scanner with the given Wails context and AppStore.
func NewScanner(ctx context.Context, store *AppStore) *Scanner {
	return &Scanner{ctx: ctx, store: store}
}

// ScanFolderStream recursively scans a directory for audio files,
// reads embedded tags, persists each track to the LocalStore, and
// emits the following Wails events for frontend progress display:
//
//	"local:scan:start"    -> { folder string, total int }
//	"local:track:scanned" -> { folder string, done int, total int, track LocalTrack }
//	"local:scan:done"     -> { folder string, count int }
func (sc *Scanner) ScanFolderStream(dir string) error {
	// count audio files first so we know the total
	var total int
	filepath.Walk(dir, func(path string, info os.FileInfo, err error) error {
		if err != nil || info.IsDir() {
			return nil
		}
		if media.IsSupportedAudio(strings.ToLower(filepath.Ext(path))) {
			total++
		}
		return nil
	})

	wailsruntime.EventsEmit(sc.ctx, "local:scan:start", map[string]any{
		"folder": dir,
		"total":  total,
	})

	done := 0
	livePaths := make(map[string]struct{}, total)

	err := filepath.Walk(dir, func(path string, info os.FileInfo, err error) error {
		if err != nil || info.IsDir() {
			return nil
		}

		if !media.IsSupportedAudio(strings.ToLower(filepath.Ext(path))) {
			return nil
		}

		lt := LocalTrack{Path: path, Title: filepath.Base(path)}
		livePaths[path] = struct{}{}

		f, err := os.Open(path)
		if err != nil {
			done++
			_ = sc.store.upsertLocalTrack(lt, dir)
			wailsruntime.EventsEmit(sc.ctx, "local:track:scanned", map[string]any{
				"folder": dir, "done": done, "total": total, "track": lt,
			})
			return nil
		}

		m, tagErr := tag.ReadFrom(f)
		f.Close()

		if tagErr == nil {
			if m.Title() != "" {
				lt.Title = m.Title()
			}
			lt.Artist = m.Artist()
			lt.Album = m.Album()
			lt.AlbumArtist = m.AlbumArtist()
			lt.Genre = m.Genre()
			lt.Year = m.Year()
			tn, _ := m.Track()
			lt.TrackNumber = tn
			dn, _ := m.Disc()
			lt.DiscNumber = dn
			lt.HasArtwork = m.Picture() != nil
		}

		if info, statErr := os.Stat(path); statErr == nil {
			probeLocalDuration(path, info, &lt)
			if lt.DurationMs == 0 {
				parseDurationFallbackLocal(path, info, &lt)
			}
		}

		_ = sc.store.upsertLocalTrack(lt, dir)
		done++
		wailsruntime.EventsEmit(sc.ctx, "local:track:scanned", map[string]any{
			"folder": dir, "done": done, "total": total, "track": lt,
		})
		return nil
	})

	// prune DB entries for files that no longer exist on disk
	if stalePaths, pruneErr := sc.store.pruneStaleLocalTracks(dir, livePaths); pruneErr != nil {
		slog.Warn("scan: failed to prune stale tracks", "folder", dir, "err", pruneErr)
	} else if len(stalePaths) > 0 {
		wailsruntime.EventsEmit(sc.ctx, "local:track:removed", map[string]any{
			"paths": stalePaths,
		})
	}

	wailsruntime.EventsEmit(sc.ctx, "local:scan:done", map[string]any{
		"folder": dir,
		"count":  done,
	})
	return err
}

// ScanAndUpsertSingleFile reads metadata for one audio file, persists it
// to the LocalStore, and returns the populated LocalTrack.
// Used by the fsnotify watcher on Create events.
func (sc *Scanner) ScanAndUpsertSingleFile(path, folder string) (LocalTrack, error) {
	lt := LocalTrack{Path: path, Title: filepath.Base(path)}

	f, err := os.Open(path)
	if err != nil {
		return lt, sc.store.upsertLocalTrack(lt, folder)
	}
	m, tagErr := tag.ReadFrom(f)
	f.Close()

	if tagErr == nil {
		if m.Title() != "" {
			lt.Title = m.Title()
		}
		lt.Artist = m.Artist()
		lt.Album = m.Album()
		lt.AlbumArtist = m.AlbumArtist()
		lt.Genre = m.Genre()
		lt.Year = m.Year()
		tn, _ := m.Track()
		lt.TrackNumber = tn
		dn, _ := m.Disc()
		lt.DiscNumber = dn
		lt.HasArtwork = m.Picture() != nil
	}

	if info, err := os.Stat(path); err == nil {
		probeLocalDuration(path, info, &lt)
		if lt.DurationMs == 0 {
			parseDurationFallbackLocal(path, info, &lt)
		}
	}

	return lt, sc.store.upsertLocalTrack(lt, folder)
}
