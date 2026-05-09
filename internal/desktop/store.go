package desktop

import (
	"context"
	"database/sql"
	"fmt"

	"pneuma/internal/store/sqlite/desktopdb"
)

// AppStore is the concrete LocalStore backed by a SQLite database via sqlc.
type AppStore struct {
	db *sql.DB
	dq *desktopdb.Queries
}

// NewAppStore creates an AppStore from an open *sql.DB.
func NewAppStore(db *sql.DB) *AppStore {
	return &AppStore{
		db: db,
		dq: desktopdb.New(db),
	}
}

// closeDB closes the underlying database connection.
func (s *AppStore) closeDB() error {
	if s.db != nil {
		return s.db.Close()
	}
	return nil
}

// checkDB returns an error if the store has not been initialised.
func (s *AppStore) checkDB() error {
	if s.dq == nil {
		return fmt.Errorf("appDB not initialised")
	}
	return nil
}

// checkDBCtx is checkDB paired with a ready-to-use background context.
func (s *AppStore) checkDBCtx() (context.Context, error) {
	if err := s.checkDB(); err != nil {
		return nil, err
	}
	return context.Background(), nil
}
