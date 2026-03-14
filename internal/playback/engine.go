package playback

import (
	"context"
	"database/sql"
	"encoding/json"
	"fmt"
	"log/slog"
	"math/rand"
	"sort"
	"sync"
	"time"

	"pneuma/internal/library"
	"pneuma/internal/models"
	"pneuma/internal/store/sqlite/dbconv"
	"pneuma/internal/store/sqlite/serverdb"
)

// RepeatMode determines queue repeat behaviour.
type RepeatMode int

const (
	RepeatOff RepeatMode = iota
	RepeatQueue
	RepeatOne
)

// EventBus can publish events (satisfied by *ws.Hub).
type EventBus interface {
	Publish(eventType string, payload any)
}

// State represents the current playback state of a user.
type State struct {
	UserID     string     `json:"-"`
	Playing    bool       `json:"playing"`
	TrackID    string     `json:"track_id"`
	PositionMS int64      `json:"position_ms"`
	Queue      []string   `json:"queue"`
	QueueIndex int        `json:"queue_index"`
	Repeat     RepeatMode `json:"repeat"`
	Shuffle    bool       `json:"shuffle"`
}

// Engine tracks live playback state for every active user.
type Engine struct {
	mu       sync.Mutex
	sessions map[string]*State // keyed by user ID
	q        *serverdb.Queries
	lib      *library.Service
	bus      EventBus
	log      *slog.Logger
}

// New creates a playback Engine.
func New(q *serverdb.Queries, bus EventBus, lib *library.Service) *Engine {
	return &Engine{
		sessions: make(map[string]*State),
		q:        q,
		lib:      lib,
		bus:      bus,
		log:      slog.Default().With("component", "engine"),
	}
}

// GetState returns the current playback state for a user.
func (e *Engine) GetState(userID string) (State, error) {
	e.mu.Lock()
	defer e.mu.Unlock()
	if s, ok := e.sessions[userID]; ok {
		return *s, nil
	}
	return State{}, fmt.Errorf("no active session for user %q", userID)
}

// Play starts or resumes playback.
func (e *Engine) Play(ctx context.Context, userID, trackID string, positionMS int64) error {
	e.mu.Lock()
	defer e.mu.Unlock()
	s := e.getOrCreate(userID)
	s.Playing = true
	if trackID != "" && trackID != s.TrackID {
		s.TrackID = trackID
		s.PositionMS = 0
		// Keep the server's queue_index in sync with the new track so that
		// playback.changed echoes the correct index back to all clients.
		for i, id := range s.Queue {
			if id == trackID {
				s.QueueIndex = i
				break
			}
		}
	} else if trackID != "" {
		s.TrackID = trackID
	}
	if positionMS > 0 {
		s.PositionMS = positionMS
	}
	return e.persist(ctx, userID, s)
}

// Pause sets paused state. If positionMS > 0 the stored position is updated
// to the caller's current playhead so the echoed state is accurate.
func (e *Engine) Pause(ctx context.Context, userID string, paused bool, positionMS int64) error {
	e.mu.Lock()
	defer e.mu.Unlock()
	s := e.getOrCreate(userID)
	s.Playing = !paused
	if positionMS > 0 {
		s.PositionMS = positionMS
	}
	return e.persist(ctx, userID, s)
}

// Seek sets the playback position (in milliseconds).
func (e *Engine) Seek(ctx context.Context, userID string, positionMS int64) error {
	e.mu.Lock()
	defer e.mu.Unlock()
	s := e.getOrCreate(userID)
	s.PositionMS = positionMS
	return e.persist(ctx, userID, s)
}

// SetQueue replaces the playback queue.
func (e *Engine) SetQueue(ctx context.Context, userID string, trackIDs []string, startIndex int) error {
	e.mu.Lock()
	defer e.mu.Unlock()
	s := e.getOrCreate(userID)
	s.Queue = trackIDs
	s.QueueIndex = startIndex
	if startIndex >= 0 && startIndex < len(trackIDs) {
		s.TrackID = trackIDs[startIndex]
		s.PositionMS = 0
	}
	return e.persist(ctx, userID, s)
}

// Next advances to the next track; returns the new track ID and queue index.
func (e *Engine) Next(ctx context.Context, userID string) (string, int, error) {
	e.mu.Lock()
	defer e.mu.Unlock()
	s := e.getOrCreate(userID)
	if len(s.Queue) == 0 {
		return s.TrackID, s.QueueIndex, nil
	}
	switch s.Repeat {
	case RepeatOne:
		s.PositionMS = 0
	case RepeatQueue:
		s.QueueIndex = (s.QueueIndex + 1) % len(s.Queue)
		s.TrackID = s.Queue[s.QueueIndex]
		s.PositionMS = 0
	default:
		if s.QueueIndex+1 < len(s.Queue) {
			s.QueueIndex++
			s.TrackID = s.Queue[s.QueueIndex]
			s.PositionMS = 0
		} else {
			s.Playing = false
		}
	}
	err := e.persist(ctx, userID, s)
	return s.TrackID, s.QueueIndex, err
}

// Prev goes back to the previous track; returns the new track ID and queue index.
func (e *Engine) Prev(ctx context.Context, userID string) (string, int, error) {
	e.mu.Lock()
	defer e.mu.Unlock()
	s := e.getOrCreate(userID)
	if len(s.Queue) == 0 {
		return s.TrackID, s.QueueIndex, nil
	}
	if s.QueueIndex > 0 {
		s.QueueIndex--
	} else if s.Repeat == RepeatQueue {
		s.QueueIndex = len(s.Queue) - 1
	}
	s.TrackID = s.Queue[s.QueueIndex]
	s.PositionMS = 0
	err := e.persist(ctx, userID, s)
	return s.TrackID, s.QueueIndex, err
}

// SetRepeat sets the repeat mode for a user.
func (e *Engine) SetRepeat(ctx context.Context, userID string, mode RepeatMode) error {
	e.mu.Lock()
	defer e.mu.Unlock()
	s := e.getOrCreate(userID)
	s.Repeat = mode
	return e.persist(ctx, userID, s)
}

// SetShuffle toggles shuffle for a user. When enabled, the queue is
// randomised with the current track pinned to index 0. When disabled, the
// queue is re-sorted by album name → disc number → track number.
func (e *Engine) SetShuffle(ctx context.Context, userID string, enabled bool) error {
	e.mu.Lock()
	defer e.mu.Unlock()
	s := e.getOrCreate(userID)
	s.Shuffle = enabled
	if enabled && len(s.Queue) > 1 {
		// Build a new queue: current track first, then the rest shuffled.
		current := s.TrackID
		rest := make([]string, 0, len(s.Queue)-1)
		for _, id := range s.Queue {
			if id != current {
				rest = append(rest, id)
			}
		}
		rand.Shuffle(len(rest), func(i, j int) { rest[i], rest[j] = rest[j], rest[i] })
		s.Queue = append([]string{current}, rest...)
		s.QueueIndex = 0
	} else if !enabled && len(s.Queue) > 1 && e.lib != nil {
		// Restore album order: sort by album_name → disc_number → track_number
		tracks, err := e.lib.TracksByIDs(ctx, s.Queue)
		if err == nil && len(tracks) > 0 {
			trackMap := make(map[string]*models.Track, len(tracks))
			for _, t := range tracks {
				trackMap[t.ID] = t
			}
			sort.SliceStable(s.Queue, func(i, j int) bool {
				ti, tj := trackMap[s.Queue[i]], trackMap[s.Queue[j]]
				if ti == nil || tj == nil {
					return false
				}
				if ti.AlbumName != tj.AlbumName {
					return ti.AlbumName < tj.AlbumName
				}
				if ti.DiscNumber != tj.DiscNumber {
					return ti.DiscNumber < tj.DiscNumber
				}
				return ti.TrackNumber < tj.TrackNumber
			})
			// Update queue index to point to current track in sorted order
			for i, id := range s.Queue {
				if id == s.TrackID {
					s.QueueIndex = i
					break
				}
			}
		}
	}
	return e.persist(ctx, userID, s)
}

// LoadSession restores a session from the database into memory.
func (e *Engine) LoadSession(ctx context.Context, userID string) (State, error) {
	row, err := e.q.PlaybackSessionByUser(ctx, userID)
	if err != nil {
		if err == sql.ErrNoRows {
			return State{}, nil
		}
		return State{}, err
	}
	sess := dbconv.SessionByUserToModel(row)
	e.mu.Lock()
	defer e.mu.Unlock()
	s := &State{
		TrackID:    sess.TrackID,
		PositionMS: sess.PositionMS,
		Queue:      sess.Queue,
	}
	e.sessions[userID] = s
	return *s, nil
}

func (e *Engine) getOrCreate(userID string) *State {
	if s, ok := e.sessions[userID]; ok {
		if userID != "" {
			s.UserID = userID
		}
		return s
	}
	s := &State{UserID: userID}
	e.sessions[userID] = s
	return s
}

// persist saves the session to the database and publishes state to WS clients.
func (e *Engine) persist(ctx context.Context, userID string, s *State) error {
	// Always notify connected clients, regardless of DB persistence result.
	defer e.bus.Publish("playback.changed", map[string]any{
		"track_id":    s.TrackID,
		"playing":     s.Playing,
		"position_ms": s.PositionMS,
		"queue":       s.Queue,
		"queue_index": s.QueueIndex,
		"repeat":      s.Repeat,
		"shuffle":     s.Shuffle,
	})

	// Cannot persist without a valid user — the FK on users(id) requires it.
	if userID == "" {
		return nil
	}

	queueJSON, err := json.Marshal(s.Queue)
	if err != nil {
		return fmt.Errorf("persist session: marshal queue: %w", err)
	}
	if err := e.q.UpsertPlaybackSession(ctx, serverdb.UpsertPlaybackSessionParams{
		ID:         userID,
		UserID:     userID,
		TrackID:    dbconv.NullStr(s.TrackID),
		PositionMs: sql.NullInt64{Int64: s.PositionMS, Valid: true},
		QueueJson:  sql.NullString{String: string(queueJSON), Valid: true},
		UpdatedAt:  dbconv.FormatTime(time.Now()),
	}); err != nil {
		e.log.Error("persist session", "user", userID, "err", err)
		return err
	}

	return nil
}
