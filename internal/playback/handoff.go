package playback

import (
	"context"
	"database/sql"
	"fmt"
	"log/slog"

	"pneuma/internal/models"
	"pneuma/internal/store/sqlite/dbconv"
	"pneuma/internal/store/sqlite/serverdb"
)

// Handoff manages cross-device queue state transfer. When a user wants to
// resume playback on a different device, Handoff reads the source device's
// persisted session and applies it to the target device's engine.
type Handoff struct {
	q      *serverdb.Queries
	engine *Engine
	log    *slog.Logger
}

// NewHandoff creates a Handoff coordinator.
func NewHandoff(q *serverdb.Queries, engine *Engine) *Handoff {
	return &Handoff{
		q:      q,
		engine: engine,
		log:    slog.Default().With("component", "handoff"),
	}
}

// Transfer copies the playback session from sourceDeviceID to targetDeviceID.
// The target device immediately receives the same track, position, and queue,
// which the frontend reads via GetState.
func (h *Handoff) Transfer(ctx context.Context, userID, sourceDeviceID, targetDeviceID string) error {
	// Validate both devices belong to the user.
	devices, err := h.q.DevicesByUser(ctx, userID)
	if err != nil {
		return err
	}
	ownedDevices := make(map[string]bool)
	for _, d := range devices {
		ownedDevices[d.ID] = true
	}
	if !ownedDevices[sourceDeviceID] || !ownedDevices[targetDeviceID] {
		return fmt.Errorf("handoff: both devices must belong to user %q", userID)
	}

	// Load source session.
	srcRow, err := h.q.PlaybackSessionByDevice(ctx, sourceDeviceID)
	if err != nil {
		if err == sql.ErrNoRows {
			return fmt.Errorf("handoff: no session on source device %q", sourceDeviceID)
		}
		return fmt.Errorf("handoff: no session on source device %q", sourceDeviceID)
	}
	src := dbconv.SessionByDeviceToModel(srcRow)

	// Ensure target has an engine session slot.
	if _, err := h.engine.GetState(targetDeviceID); err != nil {
		// Load/create a session for the target device first.
		if _, err := h.engine.LoadSession(ctx, targetDeviceID, userID); err != nil {
			return fmt.Errorf("handoff: load target session: %w", err)
		}
	}

	// Apply source state to target.
	if err := h.engine.SetQueue(ctx, targetDeviceID, userID, src.Queue, queueIndexOf(src.Queue, src.TrackID)); err != nil {
		return err
	}
	if err := h.engine.Seek(ctx, targetDeviceID, userID, src.PositionMS); err != nil {
		return err
	}

	h.log.Info("handoff complete",
		"from", sourceDeviceID, "to", targetDeviceID,
		"track", src.TrackID, "position_ms", src.PositionMS,
	)

	// Broadcast handoff event so the target frontend can act on it.
	h.engine.bus.Publish(string(models.EventQueueChanged), map[string]string{
		"device_id": targetDeviceID,
		"source":    sourceDeviceID,
	})
	return nil
}

// Sessions returns all active playback sessions for a user (for the "resume
// elsewhere" UI in the client).
func (h *Handoff) Sessions(ctx context.Context, userID string) ([]*models.PlaybackSession, error) {
	rows, err := h.q.PlaybackSessionsByUser(ctx, userID)
	if err != nil {
		return nil, err
	}
	return dbconv.SessionsByUserToModels(rows), nil
}

func queueIndexOf(queue []string, trackID string) int {
	for i, id := range queue {
		if id == trackID {
			return i
		}
	}
	return 0
}
