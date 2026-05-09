package scanner

import (
	"context"
	"log/slog"
	"os"
	"time"
)

// Scheduler performs periodic full-directory reconciliation scans to catch any
// files that were missed by inotify (e.g. files added while the server was
// offline, or network-mounted paths that don't generate events).
//
// It delegates all scanning logic to the manager, keeping itself a thin
// orchestration layer responsible only for timing and directory enumeration.
type Scheduler struct {
	// mgr is the scanner's manager that performs the actual scanning.
	mgr *Manager
	// dirs is the list of directories to scan.
	dirs []string
	// interval is the interval between scans.
	interval time.Duration
	// log is the logger for the scheduler.
	log *slog.Logger
}

// NewScheduler creates a Scheduler that will scan dirs every interval,
// delegating each directory to the Manager's SyncFolder method.
func NewScheduler(mgr *Manager, dirs []string, interval time.Duration) *Scheduler {
	return &Scheduler{
		mgr:      mgr,
		dirs:     dirs,
		interval: interval,
		log:      slog.Default().With("component", "scheduler"),
	}
}

// Start runs an immediate scan then repeats on interval until ctx is cancelled.
func (sc *Scheduler) Start(ctx context.Context) {
	sc.scan(ctx)
	ticker := time.NewTicker(sc.interval)
	defer ticker.Stop()

	for {
		select {
		case <-ctx.Done():
			return
		case <-ticker.C:
			sc.scan(ctx)
		}
	}
}

// ScanNow triggers an ad-hoc scan of all directories synchronously.
func (sc *Scheduler) ScanNow(ctx context.Context) {
	sc.scan(ctx)
}

// ScanAll triggers a scan using a background context (satisfies scanTrigger interface).
func (sc *Scheduler) ScanAll() {
	sc.scan(context.Background())
}

// ScanPath delegates single-file scanning to the Manager.
func (sc *Scheduler) ScanPath(path string) {
	sc.mgr.ScanPath(path)
}

// scan performs a full scan of all configured directories by delegating to
// the Manager's SyncFolder.
func (sc *Scheduler) scan(ctx context.Context) {
	sc.mgr.bus.Publish("scan.started", nil)
	start := time.Now()
	totalAdded, totalUpdated, totalRemoved := 0, 0, 0

	for _, dir := range sc.dirs {
		if _, err := os.Stat(dir); err != nil {
			sc.log.Warn("watch dir unavailable", "dir", dir, "err", err)
			continue
		}

		added, updated, removed := sc.mgr.SyncFolder(ctx, dir)
		totalAdded += added
		totalUpdated += updated
		totalRemoved += removed
	}

	sc.log.Info("scan complete",
		"duration", time.Since(start).Round(time.Millisecond),
		"added", totalAdded, "updated", totalUpdated, "removed", totalRemoved,
	)
	sc.mgr.bus.Publish("scan.completed", map[string]int{
		"added": totalAdded, "updated": totalUpdated, "removed": totalRemoved,
	})
}
