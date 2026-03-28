package desktop

import (
	"context"
	"database/sql"
	"encoding/json"
	"time"

	"pneuma/internal/store/sqlite/dbconv"
	"pneuma/internal/store/sqlite/desktopdb"
)

// LocalPlaybackSession mirrors the playback_session table row.
type LocalPlaybackSession struct {
	TrackID    string   `json:"track_id"`
	PositionMS int64    `json:"position_ms"`
	Queue      []string `json:"queue"`
	QueueIndex int      `json:"queue_index"`
	RepeatMode int      `json:"repeat_mode"`
	Shuffle    bool     `json:"shuffle"`
	Playing    bool     `json:"playing"`
	UpdatedAt  string   `json:"updated_at"`
}

// SavePlaybackSession persists the current playback state to local SQLite.
func (a *App) SavePlaybackSession(session LocalPlaybackSession) {
	if a.dq == nil {
		return
	}

	queueJSON, _ := json.Marshal(session.Queue)

	_ = a.dq.UpsertPlaybackSession(context.Background(), desktopdb.UpsertPlaybackSessionParams{
		TrackID:    dbconv.NullStr(session.TrackID),
		PositionMs: sql.NullInt64{Int64: session.PositionMS, Valid: true},
		QueueJson:  sql.NullString{String: string(queueJSON), Valid: true},
		QueueIndex: sql.NullInt64{Int64: int64(session.QueueIndex), Valid: true},
		RepeatMode: sql.NullInt64{Int64: int64(session.RepeatMode), Valid: true},
		Shuffle:    sql.NullInt64{Int64: dbconv.BoolInt(session.Shuffle), Valid: true},
		Playing:    sql.NullInt64{Int64: dbconv.BoolInt(session.Playing), Valid: true},
		UpdatedAt:  dbconv.FormatTime(time.Now()),
	})
}

// LoadPlaybackSession restores playback state from local SQLite.
// Returns the session and true if found, or zero value and false if not.
func (a *App) LoadPlaybackSession() (LocalPlaybackSession, bool) {
	if a.dq == nil {
		return LocalPlaybackSession{}, false
	}

	row, err := a.dq.GetPlaybackSession(context.Background())
	if err != nil {
		return LocalPlaybackSession{}, false
	}

	var queue []string
	if row.QueueJson.Valid {
		_ = json.Unmarshal([]byte(row.QueueJson.String), &queue)
	}
	if queue == nil {
		queue = []string{}
	}

	return LocalPlaybackSession{
		TrackID:    row.TrackID,
		PositionMS: row.PositionMs.Int64,
		Queue:      queue,
		QueueIndex: int(row.QueueIndex.Int64),
		RepeatMode: int(row.RepeatMode.Int64),
		Shuffle:    row.Shuffle.Int64 != 0,
		Playing:    row.Playing.Int64 != 0,
		UpdatedAt:  row.UpdatedAt,
	}, true
}
