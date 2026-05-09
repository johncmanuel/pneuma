package scanner

import (
	"context"
	"crypto/sha256"
	"encoding/hex"
	"fmt"
	"io"
	"os"
	"path/filepath"
	"strings"

	"pneuma/internal/library"
	"pneuma/internal/metadata/parser"
	"pneuma/internal/models"
)

// skipDuplicateFingerprint stores the message used when a track is skipped due to a duplicate fingerprint.
// Used by the frontend to show a more user-friendly message.
const skipDuplicateFingerprint = "duplicate_fingerprint"

// IngestResult represents the result of an ingest operation,
// including the track, whether it was new, and any skip reasons.
type IngestResult struct {
	// Track is the track that was ingested or updated. Is nil if the track was skipped.
	Track *models.Track
	// IsNew indicates whether the track was newly added (true) or updated (false).
	// Is false if the track was skipped.
	IsNew bool
	// Skipped indicates whether the track was skipped due to a duplicate
	// fingerprint or other reason.
	Skipped bool
	// SkipReason provides the reason for skipping the track, usually provided by skipDuplicateFingerprint.
	// Empty if the track was not skipped.
	SkipReason string
	// DuplicatePath is the path of the existing track that caused the skip due to a duplicate fingerprint.
	// Empty if the track was not skipped or if the duplicate track has no path.
	DuplicatePath string
}

// Ingestor is responsible for loading tracks into the library, including operations such as
// parsing metadata, generating fingerprints, and handling duplicates.
type Ingestor struct {
	// lib is the library service used to interact with the track database
	// and perform operations like lookups and upserts.
	lib *library.Service
	// parser is the metadata parser
	// used to extract track information from audio files.
	parser *parser.Parser
	// bus is the event bus used to publish events when tracks are added or updated.
	bus EventBus
}

// NewIngestor creates a new Ingestor instance with the provided library service, metadata parser, and event bus.
func NewIngestor(lib *library.Service, p *parser.Parser, bus EventBus) *Ingestor {
	return &Ingestor{
		lib:    lib,
		parser: p,
		bus:    bus,
	}
}

// Ingest processes the given file path, extracting metadata, generating a fingerprint, and adding or updating the track in the library.
func (i *Ingestor) Ingest(ctx context.Context, path string, existing *models.Track) (*IngestResult, error) {
	track, err := i.parser.ParseFile(ctx, path)
	if err != nil {
		return nil, fmt.Errorf("parse file: %w", err)
	}

	f, err := os.Open(path)
	if err != nil {
		return nil, fmt.Errorf("open file: %w", err)
	}
	defer f.Close()

	h := sha256.New()
	if _, err := io.Copy(h, f); err != nil {
		return nil, fmt.Errorf("fingerprint file: %w", err)
	}

	fingerprint := hex.EncodeToString(h.Sum(nil))

	// Check for existing track with the same fingerprint to prevent duplicates,
	// but only if the existing track (if any) doesn't already have the same fingerprint.
	dup, err := i.lib.TrackByFingerprint(ctx, fingerprint)
	if err != nil {
		return nil, fmt.Errorf("fingerprint lookup: %w", err)
	}
	if dup != nil && dup.DeletedAt == nil && dup.Path != path {
		return &IngestResult{
			Skipped:       true,
			SkipReason:    skipDuplicateFingerprint,
			DuplicatePath: dup.Path,
		}, nil
	}

	if existing == nil {
		existing, err = i.lib.TrackByPath(ctx, path)
		if err != nil {
			return nil, fmt.Errorf("lookup by path: %w", err)
		}
	}

	isNew := existing == nil

	// If the track already exists, preserve certain fields that shouldn't be overwritten by metadata parsing.
	if existing != nil {
		track.ID = existing.ID
		track.CreatedAt = existing.CreatedAt
		track.UploadedByUserID = existing.UploadedByUserID

		baseName := strings.TrimSuffix(filepath.Base(path), filepath.Ext(path))
		if track.Title == baseName && existing.Title != "" {
			track.Title = existing.Title
		}
	}

	track.Fingerprint = fingerprint

	if err := i.lib.UpsertTrack(ctx, track); err != nil {
		return nil, fmt.Errorf("upsert track: %w", err)
	}

	if isNew {
		i.bus.Publish("track.added", compactTrackEventPayload(track))
	} else {
		i.bus.Publish("track.updated", compactTrackEventPayload(track))
	}

	return &IngestResult{Track: track, IsNew: isNew}, nil
}
