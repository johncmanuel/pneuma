package playback

import (
	"context"
	"encoding/json"
	"fmt"
	"log/slog"
	"math/rand"
	"sort"
	"sync"
	"time"

	"pneuma/internal/library"
	"pneuma/internal/models"
	"pneuma/internal/store/sqlite"
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

// State represents the current playback state of a device.
type State struct {
	Playing    bool       `json:"playing"`
	TrackID    string     `json:"track_id"`
	PositionMS int64      `json:"position_ms"`
	Queue      []string   `json:"queue"`
	QueueIndex int        `json:"queue_index"`
	Repeat     RepeatMode `json:"repeat"`
	Shuffle    bool       `json:"shuffle"`
}

// Engine tracks live playback state for every active device.
type Engine struct {
	mu       sync.Mutex
	sessions map[string]*State // keyed by device ID
	store    *sqlite.Store
	lib      *library.Service
	bus      EventBus
	log      *slog.Logger
}

// New creates a playback Engine. bus is used by Handoff to publish events.
func New(store *sqlite.Store, bus EventBus, lib *library.Service) *Engine {
	return &Engine{
		sessions: make(map[string]*State),
		store:    store,
		lib:      lib,
		bus:      bus,
		log:      slog.Default().With("component", "engine"),
	}
}

// GetState returns the current playback state for a device.
func (e *Engine) GetState(deviceID string) (State, error) {
	e.mu.Lock()
	defer e.mu.Unlock()
	if s, ok := e.sessions[deviceID]; ok {
		return *s, nil
	}
	return State{}, fmt.Errorf("no active session for device %q", deviceID)
}

// Play starts or resumes playback.
func (e *Engine) Play(ctx context.Context, deviceID, trackID string, positionMS int64) error {
	e.mu.Lock()
	defer e.mu.Unlock()
	s := e.getOrCreate(deviceID)
	s.Playing = true
	if trackID != "" {
		s.TrackID = trackID
	}
	if positionMS > 0 {
		s.PositionMS = positionMS
	}
	return e.persist(ctx, deviceID, s)
}

// Pause sets paused state.
func (e *Engine) Pause(ctx context.Context, deviceID string, paused bool) error {
	e.mu.Lock()
	defer e.mu.Unlock()
	s := e.getOrCreate(deviceID)
	s.Playing = !paused
	return e.persist(ctx, deviceID, s)
}

// Seek sets the playback position (in milliseconds).
func (e *Engine) Seek(ctx context.Context, deviceID string, positionMS int64) error {
	e.mu.Lock()
	defer e.mu.Unlock()
	s := e.getOrCreate(deviceID)
	s.PositionMS = positionMS
	return e.persist(ctx, deviceID, s)
}

// SetQueue replaces the playback queue.
func (e *Engine) SetQueue(ctx context.Context, deviceID string, trackIDs []string, startIndex int) error {
	e.mu.Lock()
	defer e.mu.Unlock()
	s := e.getOrCreate(deviceID)
	s.Queue = trackIDs
	s.QueueIndex = startIndex
	if startIndex >= 0 && startIndex < len(trackIDs) {
		s.TrackID = trackIDs[startIndex]
		s.PositionMS = 0
	}
	return e.persist(ctx, deviceID, s)
}

// Next advances to the next track; returns the new track ID and queue index.
func (e *Engine) Next(ctx context.Context, deviceID string) (string, int, error) {
	e.mu.Lock()
	defer e.mu.Unlock()
	s := e.getOrCreate(deviceID)
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
	err := e.persist(ctx, deviceID, s)
	return s.TrackID, s.QueueIndex, err
}

// Prev goes back to the previous track; returns the new track ID and queue index.
func (e *Engine) Prev(ctx context.Context, deviceID string) (string, int, error) {
	e.mu.Lock()
	defer e.mu.Unlock()
	s := e.getOrCreate(deviceID)
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
	err := e.persist(ctx, deviceID, s)
	return s.TrackID, s.QueueIndex, err
}

// SetRepeat sets the repeat mode for a device.
func (e *Engine) SetRepeat(ctx context.Context, deviceID string, mode RepeatMode) error {
	e.mu.Lock()
	defer e.mu.Unlock()
	s := e.getOrCreate(deviceID)
	s.Repeat = mode
	return e.persist(ctx, deviceID, s)
}

// SetShuffle toggles shuffle for a device. When enabled, the queue is
// randomised with the current track pinned to index 0. When disabled, the
// queue is re-sorted by album name → disc number → track number.
func (e *Engine) SetShuffle(ctx context.Context, deviceID string, enabled bool) error {
	e.mu.Lock()
	defer e.mu.Unlock()
	s := e.getOrCreate(deviceID)
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
	return e.persist(ctx, deviceID, s)
}

// LoadSession restores a session from the database into memory.
func (e *Engine) LoadSession(ctx context.Context, deviceID, userID string) (State, error) {
	sess, err := e.store.PlaybackSessionByDevice(ctx, deviceID)
	if err != nil {
		return State{}, err
	}
	if sess == nil {
		return State{}, nil
	}
	e.mu.Lock()
	defer e.mu.Unlock()
	s := &State{
		TrackID:    sess.TrackID,
		PositionMS: sess.PositionMS,
		Queue:      sess.Queue,
	}
	e.sessions[deviceID] = s
	return *s, nil
}

func (e *Engine) getOrCreate(deviceID string) *State {
	if s, ok := e.sessions[deviceID]; ok {
		return s
	}
	s := &State{}
	e.sessions[deviceID] = s
	return s
}

// persist saves the session to the database.
func (e *Engine) persist(ctx context.Context, deviceID string, s *State) error {
	qj, _ := json.Marshal(s.Queue)
	sess := &models.PlaybackSession{
		ID:         deviceID, // use device ID as session ID for simplicity
		DeviceID:   deviceID,
		TrackID:    s.TrackID,
		PositionMS: s.PositionMS,
		Queue:      s.Queue,
		UpdatedAt:  time.Now(),
	}
	_ = qj // queue is marshalled inside the store layer
	if err := e.store.UpsertPlaybackSession(ctx, sess); err != nil {
		e.log.Error("persist session", "device", deviceID, "err", err)
		return err
	}

	// Notify all connected clients of the state change.
	e.bus.Publish("playback.changed", map[string]any{
		"device_id":   deviceID,
		"track_id":    s.TrackID,
		"playing":     s.Playing,
		"position_ms": s.PositionMS,
		"queue":       s.Queue,
		"queue_index": s.QueueIndex,
		"repeat":      s.Repeat,
		"shuffle":     s.Shuffle,
	})
	return nil
}
