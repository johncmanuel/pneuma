package playback

import (
	"context"
	"database/sql"
	"encoding/json"
	"errors"
	"fmt"
	"log/slog"
	"math/rand/v2"
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
	PublishToUser(userID string, eventType string, payload any)
	PublishToDevice(userID string, deviceID string, eventType string, payload any)
}

// State represents the current playback state of a user.
type State struct {
	UserID     string        `json:"-"`
	DeviceID   string        `json:"-"`
	Playing    bool          `json:"playing"`
	TrackID    string        `json:"track_id"`
	Track      *models.Track `json:"track,omitempty"`
	PositionMS int64         `json:"position_ms"`
	Queue      []string      `json:"queue"`
	QueueIndex int           `json:"queue_index"`
	Repeat     RepeatMode    `json:"repeat"`
	Shuffle    bool          `json:"shuffle"`
}

type wsPlaybackTrack struct {
	ID          string `json:"id"`
	Title       string `json:"title"`
	AlbumArtist string `json:"album_artist"`
	AlbumName   string `json:"album_name"`
	DurationMS  int64  `json:"duration_ms"`
}

func compactPlaybackTrack(track *models.Track) *wsPlaybackTrack {
	if track == nil {
		return nil
	}

	return &wsPlaybackTrack{
		ID:          track.ID,
		Title:       track.Title,
		AlbumArtist: track.AlbumArtist,
		AlbumName:   track.AlbumName,
		DurationMS:  track.DurationMS,
	}
}

// ErrNoActiveSession is returned when there is no active playback session for a user/device.
var ErrNoActiveSession = errors.New("no active session")

// Engine tracks live playback state for every active user.
type Engine struct {
	mu       sync.Mutex
	sessions map[string]*State // keyed by device ID
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

// GetState returns the current playback state for a user/device.
func (e *Engine) GetState(userID, deviceID string) (State, error) {
	e.mu.Lock()
	defer e.mu.Unlock()
	if s, ok := e.sessions[deviceID]; ok && s.UserID == userID {
		if s.Track == nil {
			s.Track = e.trackByID(context.Background(), s.TrackID)
		}
		return *s, nil
	}
	return State{}, ErrNoActiveSession
}

// Play starts or resumes playback for a track.
func (e *Engine) Play(ctx context.Context, userID, deviceID, trackID string, positionMS int64) error {
	e.mu.Lock()
	defer e.mu.Unlock()

	s := e.getOrCreate(userID, deviceID)
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
	return e.persist(ctx, userID, deviceID, s)
}

// Pause sets paused state. If positionMS > 0 the stored position is updated
// to the caller's current playhead so the echoed state is accurate.
func (e *Engine) Pause(ctx context.Context, userID, deviceID string, paused bool, positionMS int64) error {
	e.mu.Lock()
	defer e.mu.Unlock()

	s := e.getOrCreate(userID, deviceID)
	s.Playing = !paused

	if positionMS > 0 {
		s.PositionMS = positionMS
	}
	return e.persist(ctx, userID, deviceID, s)
}

// Seek sets the playback position (in milliseconds).
func (e *Engine) Seek(ctx context.Context, userID, deviceID string, positionMS int64) error {
	e.mu.Lock()
	defer e.mu.Unlock()

	s := e.getOrCreate(userID, deviceID)
	s.PositionMS = positionMS
	return e.persist(ctx, userID, deviceID, s)
}

// SetQueue replaces the current playback queue.
func (e *Engine) SetQueue(ctx context.Context, userID, deviceID string, trackIDs []string, startIndex int) error {
	e.mu.Lock()
	defer e.mu.Unlock()

	s := e.getOrCreate(userID, deviceID)
	s.Queue = trackIDs
	s.QueueIndex = startIndex

	if startIndex >= 0 && startIndex < len(trackIDs) {
		s.TrackID = trackIDs[startIndex]
		s.PositionMS = 0
	}
	return e.persist(ctx, userID, deviceID, s)
}

// Next advances to the next track; returns the new track ID and queue index.
func (e *Engine) Next(ctx context.Context, userID, deviceID string) (string, int, error) {
	e.mu.Lock()
	defer e.mu.Unlock()

	s := e.getOrCreate(userID, deviceID)

	if len(s.Queue) == 0 {
		return s.TrackID, s.QueueIndex, nil
	}

	switch s.Repeat {
	case RepeatOne:
		s.PositionMS = 0
	case RepeatQueue:
		s.QueueIndex++
		// if we've reached the end of the queue, wrap around
		if s.QueueIndex >= len(s.Queue) {
			s.QueueIndex = 0
			// if shuffle is on, reshuffle the queue
			if s.Shuffle && len(s.Queue) > 1 {
				lastTrackID := s.TrackID
				rand.Shuffle(len(s.Queue), func(i, j int) { s.Queue[i], s.Queue[j] = s.Queue[j], s.Queue[i] })
				// don't want the same track from playing twice in a row
				if s.Queue[0] == lastTrackID && len(s.Queue) > 1 {
					s.Queue[0], s.Queue[1] = s.Queue[1], s.Queue[0]
				}
			}
		}
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
	err := e.persist(ctx, userID, deviceID, s)
	return s.TrackID, s.QueueIndex, err
}

// Prev goes back to the previous track; returns the new track ID and queue index.
func (e *Engine) Prev(ctx context.Context, userID, deviceID string) (string, int, error) {
	e.mu.Lock()
	defer e.mu.Unlock()

	s := e.getOrCreate(userID, deviceID)

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
	err := e.persist(ctx, userID, deviceID, s)
	return s.TrackID, s.QueueIndex, err
}

// SetRepeat sets the repeat mode for a user/device.
func (e *Engine) SetRepeat(ctx context.Context, userID, deviceID string, mode RepeatMode) error {
	e.mu.Lock()
	defer e.mu.Unlock()

	s := e.getOrCreate(userID, deviceID)
	s.Repeat = mode
	return e.persist(ctx, userID, deviceID, s)
}

// SetShuffle toggles shuffle for a user/device. When enabled, the queue is
// randomised with the current track pinned to index 0. When disabled, the
// queue is re-sorted in this order:
// 1. album name
// 2. disc number
// 3. track number.
func (e *Engine) SetShuffle(ctx context.Context, userID, deviceID string, enabled bool) error {
	e.mu.Lock()
	defer e.mu.Unlock()

	s := e.getOrCreate(userID, deviceID)
	s.Shuffle = enabled

	// Build a new queue with the current track first, then the rest shuffled.
	if enabled && len(s.Queue) > 1 {
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
		// if disabled, re-sort the queue by album/disc/track
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
	return e.persist(ctx, userID, deviceID, s)
}

// LoadSession restores a session from the database into memory.
func (e *Engine) LoadSession(ctx context.Context, userID, deviceID string) (State, error) {
	row, err := e.q.PlaybackSessionByDevice(ctx, deviceID)
	if err != nil {
		if err == sql.ErrNoRows {
			return State{}, nil
		}
		return State{}, err
	}

	sess := dbconv.SessionByDeviceToModel(row)
	if sess.UserID != userID {
		return State{}, fmt.Errorf("session belongs to different user")
	}

	e.mu.Lock()
	defer e.mu.Unlock()

	s := &State{
		UserID:     sess.UserID,
		DeviceID:   deviceID,
		TrackID:    sess.TrackID,
		Track:      e.trackByID(ctx, sess.TrackID),
		PositionMS: sess.PositionMS,
		Queue:      sess.Queue,
		QueueIndex: sess.QueueIndex,
		Repeat:     RepeatMode(sess.RepeatMode),
		Shuffle:    sess.Shuffle,
		Playing:    sess.Playing,
	}
	e.sessions[deviceID] = s
	return *s, nil
}

// loadFromDB loads a session from the database without taking a lock.
func (e *Engine) loadFromDB(deviceID string) (State, error) {
	row, err := e.q.PlaybackSessionByDevice(context.Background(), deviceID)
	if err != nil {
		return State{}, err
	}
	sess := dbconv.SessionByDeviceToModel(row)
	return State{
		UserID:     sess.UserID,
		DeviceID:   deviceID,
		TrackID:    sess.TrackID,
		Track:      e.trackByID(context.Background(), sess.TrackID),
		PositionMS: sess.PositionMS,
		Queue:      sess.Queue,
		QueueIndex: sess.QueueIndex,
		Repeat:     RepeatMode(sess.RepeatMode),
		Shuffle:    sess.Shuffle,
		Playing:    sess.Playing,
	}, nil
}

// getOrCreate returns the session for a user/device, creating a new one if needed.
// On first access, attempt to restore the session from the database.
func (e *Engine) getOrCreate(userID, deviceID string) *State {
	if s, ok := e.sessions[deviceID]; ok {
		if userID != "" {
			s.UserID = userID
		}
		return s
	}

	if deviceID != "" {
		if s, err := e.loadFromDB(deviceID); err == nil && s.TrackID != "" {
			e.sessions[deviceID] = &s
			return &s
		}
	}

	s := &State{UserID: userID, DeviceID: deviceID}
	e.sessions[deviceID] = s
	return s
}

// persist saves the session to the database and publishes state to WS clients.
func (e *Engine) persist(ctx context.Context, userID, deviceID string, s *State) error {
	trackPayload := e.trackByID(ctx, s.TrackID)
	s.Track = trackPayload

	e.bus.PublishToDevice(userID, deviceID, "playback.changed", map[string]any{
		"track_id":    s.TrackID,
		"track":       compactPlaybackTrack(trackPayload),
		"playing":     s.Playing,
		"position_ms": s.PositionMS,
		"queue":       s.Queue,
		"queue_index": s.QueueIndex,
		"repeat":      s.Repeat,
		"shuffle":     s.Shuffle,
	})

	// require valid users to persist sessions;
	if userID == "" || deviceID == "" {
		return nil
	}

	queueJSON, err := json.Marshal(s.Queue)
	if err != nil {
		return fmt.Errorf("persist session: marshal queue: %w", err)
	}

	now := time.Now()
	nowStr := dbconv.FormatTime(now)

	// Ensure the device exists before writing the session tying to it (this addresses the FK constraint error)
	if err := e.q.UpsertDevice(ctx, serverdb.UpsertDeviceParams{
		ID:         deviceID,
		UserID:     userID,
		Name:       "some client", // random fallback name
		CreatedAt:  nowStr,
		LastActive: nowStr,
	}); err != nil {
		e.log.Error("upsert device", "device", deviceID, "err", err)
	}

	if err := e.q.UpsertPlaybackSession(ctx, serverdb.UpsertPlaybackSessionParams{
		ID:         deviceID,
		DeviceID:   deviceID,
		UserID:     userID,
		TrackID:    dbconv.NullStr(s.TrackID),
		PositionMs: sql.NullInt64{Int64: s.PositionMS, Valid: true},
		QueueJson:  sql.NullString{String: string(queueJSON), Valid: true},
		QueueIndex: sql.NullInt64{Int64: int64(s.QueueIndex), Valid: true},
		RepeatMode: sql.NullInt64{Int64: int64(s.Repeat), Valid: true},
		Shuffle:    s.Shuffle,
		Playing:    s.Playing,
		UpdatedAt:  nowStr,
	}); err != nil {
		e.log.Error("persist session", "device", deviceID, "err", err)
		return err
	}

	return nil
}

// trackByID is a helper to get a track by ID, returning nil if not found or on error.
func (e *Engine) trackByID(ctx context.Context, trackID string) *models.Track {
	if trackID == "" || e.lib == nil {
		return nil
	}

	track, err := e.lib.TrackByID(ctx, trackID)
	if err != nil || track == nil {
		return nil
	}

	return track
}
