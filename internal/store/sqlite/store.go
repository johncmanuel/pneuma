package sqlite

import (
	"context"
	"database/sql"
	"errors"
	"fmt"
	"log/slog"
	"os"
	"path/filepath"
	"strings"
	"time"

	"github.com/golang-migrate/migrate/v4"
	migratesqlite "github.com/golang-migrate/migrate/v4/database/sqlite"
	"github.com/golang-migrate/migrate/v4/source/iofs"
	_ "modernc.org/sqlite"
)

// Store wraps a SQLite database connection.
type Store struct {
	db *sql.DB
}

const SqlServerMigrationsDir = "sql/server/migrations"

// OpenRaw opens the SQLite database at path, creates the directory if needed,
// and applies the standard connection pragmas. NOTE: this does not include migrations logic.
// See Open (under package sqlite) for the normal server entrypoint.
func OpenRaw(path string, enableFKs bool) (*sql.DB, error) {
	if err := os.MkdirAll(filepath.Dir(path), 0o755); err != nil {
		return nil, fmt.Errorf("create db dir: %w", err)
	}

	fkState := "ON"
	if !enableFKs {
		fkState = "OFF"
	}

	// Will leave a decent article for reference on sqlite performance below.
	// https://phiresky.github.io/blog/2020/sqlite-performance-tuning/
	pragmas := []string{
		"_pragma=journal_mode(WAL)",                      // ensure concurrent read and writes
		"_pragma=synchronous(NORMAL)",                    // use fewer fsyncs for better performance (obvs tradeoff is less data durability)
		"_pragma=busy_timeout(10000)",                    // 10s retry window
		"_pragma=cache_size(-32000)",                     // ~32 MB page cache in memory
		"_pragma=temp_store(MEMORY)",                     // store temp tables in memory instead of on disk to speed up queries
		fmt.Sprintf("_pragma=foreign_keys(%s)", fkState), // enable/disable foreign key constraints
	}

	dsn := fmt.Sprintf(
		"file:%s?%s",
		path, strings.Join(pragmas, "&"),
	)

	db, err := sql.Open("sqlite", dsn)
	if err != nil {
		return nil, fmt.Errorf("sqlite open %s: %w", path, err)
	}

	// Limit to a single connection so that all goroutines serialise through
	// the same underlying SQLite handle. This prevents SQLITE_BUSY / "database
	// is locked" errors that occur during heavy workloads like uploading large numbers
	// of tracks and albums
	// NOTE: could change this later in the future to allow for more connections, but
	// not sure how to avoid SQLITE_BUSY errors while increasing the number of connections.
	db.SetMaxOpenConns(1)
	db.SetMaxIdleConns(1)

	// for long-lived connections, run PRAGMA optimize=0x10002 on first open
	// https://www.sqlite.org/pragma.html#pragma_optimize
	if _, err := db.Exec("PRAGMA optimize=0x10002"); err != nil {
		db.Close()
		return nil, fmt.Errorf("pragma optimize on open: %w", err)
	}

	return db, nil
}

// Open creates or opens the SQLite database at path and applies all pending
// migrations. This is the normal server entrypoint.
func Open(path string) (*Store, error) {
	// Disable FK enforcement while migrations run.
	//
	// Migration 003 drops and recreates the tracks table; disabling FKs prevents
	// the cascade-constraint error from tables (e.g. playlist_items) that reference
	// tracks.
	db, err := OpenRaw(path, false)
	if err != nil {
		return nil, err
	}

	if err := runMigrations(db); err != nil {
		db.Close()
		return nil, err
	}
	db.Close()

	db, err = OpenRaw(path, true)
	if err != nil {
		return nil, err
	}

	return &Store{db: db}, nil
}

// Close runs PRAGMA optimize then closes the database.
func (s *Store) Close() error {
	_ = s.Optimize()
	return s.db.Close()
}

// Optimize runs PRAGMA optimize
func (s *Store) Optimize() error {
	slog.Info("running PRAGMA optimize...")
	_, err := s.db.Exec("PRAGMA optimize")
	return err
}

// RunOptimizePeriodically calls Optimize on the given interval until ctx is cancelled.
func (s *Store) RunOptimizePeriodically(ctx context.Context, interval time.Duration) {
	ticker := time.NewTicker(interval)
	defer ticker.Stop()
	for {
		select {
		case <-ctx.Done():
			return
		case <-ticker.C:
			if err := s.Optimize(); err != nil {
				slog.Warn("PRAGMA optimize failed", "err", err)
			}
		}
	}
}

// DB returns the underlying *sql.DB (for testing).
func (s *Store) DB() *sql.DB {
	return s.db
}

// NewMigrator builds a *migrate.Migrate instance backed by the embedded
// migration files and the provided DB connection. The caller is responsible
// for calling m.Close() when done.
func NewMigrator(db *sql.DB) (*migrate.Migrate, error) {
	sourceDriver, err := iofs.New(ServerMigrations, SqlServerMigrationsDir)
	if err != nil {
		return nil, fmt.Errorf("migration source: %w", err)
	}

	dbDriver, err := migratesqlite.WithInstance(db, &migratesqlite.Config{})
	if err != nil {
		return nil, fmt.Errorf("migration db driver: %w", err)
	}

	m, err := migrate.NewWithInstance("iofs", sourceDriver, "sqlite", dbDriver)
	if err != nil {
		return nil, fmt.Errorf("migrate new: %w", err)
	}

	return m, nil
}

// runMigrations applies all pending migrations to the database.
func runMigrations(db *sql.DB) error {
	m, err := NewMigrator(db)
	if err != nil {
		return err
	}

	if err := m.Up(); err != nil && !errors.Is(err, migrate.ErrNoChange) {
		return fmt.Errorf("run migrations: %w", err)
	}

	return nil
}
