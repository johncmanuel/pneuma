package sqlite

import (
	"context"
	"database/sql"
	"encoding/json"
	"fmt"
	"time"

	"pneuma/internal/models"
)

// ─── Playback Sessions ────────────────────────────────────────────────────────

// UpsertPlaybackSession stores or replaces the playback state for a device.
func (s *Store) UpsertPlaybackSession(ctx context.Context, ps *models.PlaybackSession) error {
	queueJSON, err := json.Marshal(ps.Queue)
	if err != nil {
		return fmt.Errorf("playback session marshal queue: %w", err)
	}
	const q = `INSERT INTO playback_sessions (id,device_id,user_id,track_id,position_ms,queue_json,updated_at)
			   VALUES (?,?,?,?,?,?,?)
			   ON CONFLICT(device_id) DO UPDATE SET
			   	track_id=excluded.track_id, position_ms=excluded.position_ms,
			   	queue_json=excluded.queue_json, updated_at=excluded.updated_at`
	_, err = s.db.ExecContext(ctx, q,
		ps.ID, ps.DeviceID, ps.UserID, nullStr(ps.TrackID),
		ps.PositionMS, string(queueJSON),
		ps.UpdatedAt.UTC().Format(time.RFC3339),
	)
	return err
}

// PlaybackSessionByDevice returns the playback session for a device, or nil.
func (s *Store) PlaybackSessionByDevice(ctx context.Context, deviceID string) (*models.PlaybackSession, error) {
	const q = `SELECT id,device_id,user_id,COALESCE(track_id,''),position_ms,queue_json,updated_at
			   FROM playback_sessions WHERE device_id=? LIMIT 1`
	row := s.db.QueryRowContext(ctx, q, deviceID)
	return scanSession(row)
}

// PlaybackSessionsByUser returns all active sessions for a user (for handoff).
func (s *Store) PlaybackSessionsByUser(ctx context.Context, userID string) ([]*models.PlaybackSession, error) {
	const q = `SELECT ps.id,ps.device_id,ps.user_id,COALESCE(ps.track_id,''),ps.position_ms,ps.queue_json,ps.updated_at
			   FROM playback_sessions ps
			   JOIN devices d ON d.id=ps.device_id
			   WHERE ps.user_id=?
			   ORDER BY ps.updated_at DESC`
	rows, err := s.db.QueryContext(ctx, q, userID)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	var out []*models.PlaybackSession
	for rows.Next() {
		ps, err := scanSession(rows)
		if err != nil {
			return nil, err
		}
		out = append(out, ps)
	}
	return out, rows.Err()
}

// ─── Offline Packs ────────────────────────────────────────────────────────────

// UpsertOfflinePack records a locally cached track.
func (s *Store) UpsertOfflinePack(ctx context.Context, op *models.OfflinePack) error {
	const q = `INSERT INTO offline_packs (id,user_id,track_id,local_path,downloaded_at)
			   VALUES (?,?,?,?,?)
			   ON CONFLICT(user_id,track_id) DO UPDATE SET local_path=excluded.local_path, downloaded_at=excluded.downloaded_at`
	_, err := s.db.ExecContext(ctx, q,
		op.ID, op.UserID, op.TrackID, op.LocalPath,
		op.DownloadedAt.UTC().Format(time.RFC3339),
	)
	return err
}

// DeleteOfflinePack removes an offline pack entry.
func (s *Store) DeleteOfflinePack(ctx context.Context, userID, trackID string) error {
	_, err := s.db.ExecContext(ctx, `DELETE FROM offline_packs WHERE user_id=? AND track_id=?`, userID, trackID)
	return err
}

// ListOfflinePacks returns all offline packs for a user.
func (s *Store) ListOfflinePacks(ctx context.Context, userID string) ([]*models.OfflinePack, error) {
	const q = `SELECT id,user_id,track_id,local_path,downloaded_at FROM offline_packs WHERE user_id=? ORDER BY downloaded_at DESC`
	rows, err := s.db.QueryContext(ctx, q, userID)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	var out []*models.OfflinePack
	for rows.Next() {
		var op models.OfflinePack
		if err := rows.Scan(&op.ID, &op.UserID, &op.TrackID, &op.LocalPath, (*timeStr)(&op.DownloadedAt)); err != nil {
			return nil, err
		}
		out = append(out, &op)
	}
	return out, rows.Err()
}

// ─── Helper ───────────────────────────────────────────────────────────────────

func scanSession(row scanner) (*models.PlaybackSession, error) {
	var ps models.PlaybackSession
	var queueJSON string
	if err := row.Scan(
		&ps.ID, &ps.DeviceID, &ps.UserID, &ps.TrackID,
		&ps.PositionMS, &queueJSON, (*timeStr)(&ps.UpdatedAt),
	); err != nil {
		if err == sql.ErrNoRows {
			return nil, nil
		}
		return nil, err
	}
	if err := json.Unmarshal([]byte(queueJSON), &ps.Queue); err != nil {
		ps.Queue = []string{}
	}
	return &ps, nil
}
