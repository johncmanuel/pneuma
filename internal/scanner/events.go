package scanner

import "pneuma/internal/models"

// EventBus is any type that can publish library change events.
type EventBus interface {
	Publish(eventType string, payload any)
}

// trackEventPayload is a DTO for track-related events,
// containing only the track ID. No other data is really needed.
type trackEventPayload struct {
	// ID is the unique identifier of the track associated with the event.
	ID string `json:"id"`
}

// compactTrackEventPayload creates a trackEventPayload with only the ID
// field populated, used for events where only the track ID is needed. Returns
// an empty payload if the track is nil.
func compactTrackEventPayload(track *models.Track) trackEventPayload {
	if track == nil {
		return trackEventPayload{}
	}
	return trackEventPayload{ID: track.ID}
}
