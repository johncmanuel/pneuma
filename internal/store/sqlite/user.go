package sqlite

import (
	"context"
	"database/sql"
	"time"

	"pneuma/internal/models"
)

// ─── Users ───────────────────────────────────────────────────────────────────

// CreateUser inserts a new user record. Returns an error if the username exists.
func (s *Store) CreateUser(ctx context.Context, u *models.User) error {
	const q = `INSERT INTO users (id,username,password_hash,is_admin,created_at,updated_at)
			   VALUES (?,?,?,?,?,?)`
	_, err := s.db.ExecContext(ctx, q,
		u.ID, u.Username, u.PasswordHash, boolInt(u.IsAdmin),
		u.CreatedAt.UTC().Format(time.RFC3339),
		u.UpdatedAt.UTC().Format(time.RFC3339),
	)
	return err
}

// UpdateUserPassword replaces the stored password hash for a user.
func (s *Store) UpdateUserPassword(ctx context.Context, userID, hash string) error {
	_, err := s.db.ExecContext(ctx,
		`UPDATE users SET password_hash=?,updated_at=? WHERE id=?`,
		hash, time.Now().UTC().Format(time.RFC3339), userID,
	)
	return err
}

// UserByUsername returns a user by their username, or nil if not found.
func (s *Store) UserByUsername(ctx context.Context, username string) (*models.User, error) {
	const q = `SELECT id,username,password_hash,is_admin,created_at,updated_at FROM users WHERE username=? LIMIT 1`
	row := s.db.QueryRowContext(ctx, q, username)
	return scanUser(row)
}

// UserByID returns a user by ID.
func (s *Store) UserByID(ctx context.Context, id string) (*models.User, error) {
	const q = `SELECT id,username,password_hash,is_admin,created_at,updated_at FROM users WHERE id=? LIMIT 1`
	row := s.db.QueryRowContext(ctx, q, id)
	return scanUser(row)
}

// CountUsers returns the total number of registered users.
func (s *Store) CountUsers(ctx context.Context) (int, error) {
	var n int
	err := s.db.QueryRowContext(ctx, `SELECT COUNT(*) FROM users`).Scan(&n)
	return n, err
}

// ─── Devices ─────────────────────────────────────────────────────────────────

// UpsertDevice inserts or updates a device record.
func (s *Store) UpsertDevice(ctx context.Context, d *models.Device) error {
	const q = `INSERT INTO devices (id,user_id,name,last_seen_at,created_at) VALUES (?,?,?,?,?)
			   ON CONFLICT(id) DO UPDATE SET name=excluded.name, last_seen_at=excluded.last_seen_at`
	var lastSeen *string
	if d.LastSeenAt != nil {
		s := d.LastSeenAt.UTC().Format(time.RFC3339)
		lastSeen = &s
	}
	_, err := s.db.ExecContext(ctx, q,
		d.ID, d.UserID, d.Name, lastSeen,
		d.CreatedAt.UTC().Format(time.RFC3339),
	)
	return err
}

// DevicesByUser returns all devices for a given user.
func (s *Store) DevicesByUser(ctx context.Context, userID string) ([]*models.Device, error) {
	const q = `SELECT id,user_id,name,last_seen_at,created_at FROM devices WHERE user_id=? ORDER BY last_seen_at DESC`
	rows, err := s.db.QueryContext(ctx, q, userID)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	var out []*models.Device
	for rows.Next() {
		d, err := scanDevice(rows)
		if err != nil {
			return nil, err
		}
		out = append(out, d)
	}
	return out, rows.Err()
}

// TouchDevice updates the last_seen_at timestamp for a device.
func (s *Store) TouchDevice(ctx context.Context, deviceID string) error {
	_, err := s.db.ExecContext(ctx,
		`UPDATE devices SET last_seen_at=? WHERE id=?`,
		time.Now().UTC().Format(time.RFC3339), deviceID,
	)
	return err
}

// ─── Helpers ─────────────────────────────────────────────────────────────────

func scanUser(row scanner) (*models.User, error) {
	var u models.User
	var isAdmin int
	if err := row.Scan(
		&u.ID, &u.Username, &u.PasswordHash, &isAdmin,
		(*timeStr)(&u.CreatedAt), (*timeStr)(&u.UpdatedAt),
	); err != nil {
		if err == sql.ErrNoRows {
			return nil, nil
		}
		return nil, err
	}
	u.IsAdmin = isAdmin != 0
	return &u, nil
}

func scanDevice(row scanner) (*models.Device, error) {
	var d models.Device
	var lastSeen sql.NullString
	if err := row.Scan(&d.ID, &d.UserID, &d.Name, &lastSeen, (*timeStr)(&d.CreatedAt)); err != nil {
		return nil, err
	}
	if lastSeen.Valid {
		ts, _ := time.Parse(time.RFC3339, lastSeen.String)
		d.LastSeenAt = &ts
	}
	return &d, nil
}

func boolInt(b bool) int {
	if b {
		return 1
	}
	return 0
}
