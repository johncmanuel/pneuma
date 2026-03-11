package sqlite

import (
	"database/sql"
	"errors"
	"fmt"
	"os"
	"path/filepath"

	"github.com/golang-migrate/migrate/v4"
	migratesqlite "github.com/golang-migrate/migrate/v4/database/sqlite"
	"github.com/golang-migrate/migrate/v4/source/iofs"
	_ "modernc.org/sqlite"
)

// Store wraps a SQLite database connection.
type Store struct {
	db *sql.DB
}

// Open creates or opens the SQLite database at path and applies the schema.
func Open(path string) (*Store, error) {
	if err := os.MkdirAll(filepath.Dir(path), 0o755); err != nil {
		return nil, fmt.Errorf("create db dir: %w", err)
	}

	db, err := sql.Open("sqlite", path)
	if err != nil {
		return nil, fmt.Errorf("sqlite open %s: %w", path, err)
	}

	// Limit to a single connection so that all goroutines serialise through
	// the same underlying SQLite handle. This prevents SQLITE_BUSY / "database
	// is locked" errors that occur when database/sql opens multiple concurrent
	// connections and the per-connection pragmas (especially busy_timeout) are
	// not applied to every new connection from the pool.
	// With WAL mode, reads and writes can overlap efficiently even through one
	// connection because Go's database/sql queues callers in-process.
	db.SetMaxOpenConns(1)
	db.SetMaxIdleConns(1)

	// Pragmas must be set on the single connection before any other use.
	// Order matters: WAL must be enabled before synchronous/timeout settings.
	for _, pragma := range []string{
		"PRAGMA journal_mode=WAL",   // WAL: readers never block writers
		"PRAGMA synchronous=NORMAL", // safe with WAL, faster than FULL
		"PRAGMA foreign_keys=ON",
		"PRAGMA busy_timeout=10000", // 10 s retry window (belt-and-suspenders)
		"PRAGMA cache_size=-32000",  // ~32 MB page cache
		"PRAGMA temp_store=MEMORY",  // temp tables in memory
	} {
		if _, err := db.Exec(pragma); err != nil {
			return nil, fmt.Errorf("pragma: %w", err)
		}
	}

	if err := runMigrations(db); err != nil {
		return nil, err
	}

	return &Store{db: db}, nil
}

// Close closes the database.
func (s *Store) Close() error {
	return s.db.Close()
}

// DB returns the underlying *sql.DB (for testing).
func (s *Store) DB() *sql.DB {
	return s.db
}

func runMigrations(db *sql.DB) error {
	sourceDriver, err := iofs.New(ServerMigrations, "sql/server/migrations")
	if err != nil {
		return fmt.Errorf("migration source: %w", err)
	}
	dbDriver, err := migratesqlite.WithInstance(db, &migratesqlite.Config{})
	if err != nil {
		return fmt.Errorf("migration db driver: %w", err)
	}
	m, err := migrate.NewWithInstance("iofs", sourceDriver, "sqlite", dbDriver)
	if err != nil {
		return fmt.Errorf("migrate new: %w", err)
	}
	if err := m.Up(); err != nil && !errors.Is(err, migrate.ErrNoChange) {
		return fmt.Errorf("run migrations: %w", err)
	}
	return nil
}
