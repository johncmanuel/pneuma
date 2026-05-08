package desktop

import (
	"context"
	"fmt"
	"strings"

	"pneuma/internal/playlist"
)

// randomTrack holds the minimum info needed for random playlist generation.
type randomTrack struct {
	source      string // "local_ref" or "remote"
	trackID     string
	localPath   string
	title       string
	album       string
	albumArtist string
	durationMS  int64
}

// GenerateRandomPlaylist creates a new playlist filled with randomly selected
// tracks targeting the given duration in minutes. Local tracks are always
// included. If useRemote is true and the app is connected to a server, remote
// tracks are added to the pool as well, producing a mix of both sources.
func (a *App) GenerateRandomPlaylist(name, description string, durationMinutes int, useRemote bool) (*LocalPlaylistSummary, error) {
	if a.store == nil {
		return nil, fmt.Errorf("db not initialised")
	}
	if strings.TrimSpace(name) == "" {
		return nil, fmt.Errorf("playlist name is required")
	}
	if durationMinutes <= 0 {
		return nil, fmt.Errorf("duration must be at least 1 minute")
	}

	targetMS := int64(durationMinutes) * 60 * 1000

	candidates, err := a.randomPlaylistCandidates(useRemote)
	if err != nil {
		return nil, err
	}
	if len(candidates) == 0 {
		return nil, fmt.Errorf("no track candidates available")
	}

	deduped := dedupeRandomTracks(candidates)
	if len(deduped) == 0 {
		return nil, fmt.Errorf("no track candidates available after deduplication")
	}

	durations := make([]int64, len(deduped))
	for i, t := range deduped {
		durations[i] = t.durationMS
	}
	selected := playlist.SelectRandomByDuration(durations, targetMS)

	if len(selected) == 0 {
		return nil, fmt.Errorf("no selected tracks available")
	}

	pl, err := a.CreateLocalPlaylist(name, description)
	if err != nil {
		return nil, err
	}

	items := make([]LocalPlaylistItem, len(selected))
	for i, idx := range selected {
		t := deduped[idx]
		items[i] = LocalPlaylistItem{
			Position:       i,
			Source:         t.source,
			TrackID:        t.trackID,
			LocalPath:      t.localPath,
			RefTitle:       t.title,
			RefAlbum:       t.album,
			RefAlbumArtist: t.albumArtist,
			RefDurationMS:  t.durationMS,
		}
	}

	if err := a.SetLocalPlaylistItems(pl.ID, items); err != nil {
		_ = a.DeleteLocalPlaylist(pl.ID)
		return nil, fmt.Errorf("set items: %w", err)
	}

	return pl, nil
}

// randomPlaylistCandidates loads the normalized candidates used by the random
// playlist picker. Remote tracks are optional and any remote fetch failure is
// treated as a best-effort miss so local playlists still work.
func (a *App) randomPlaylistCandidates(useRemote bool) ([]randomTrack, error) {
	if a.store == nil {
		return nil, fmt.Errorf("db not initialised")
	}
	localTracks, err := a.store.localRandomPlaylistCandidates(context.Background())
	if err != nil {
		return nil, err
	}

	if !useRemote {
		return localTracks, nil
	}

	a.mu.RLock()
	serverURL := a.serverURL
	token := a.token
	a.mu.RUnlock()

	if serverURL == "" || token == "" {
		return localTracks, nil
	}

	remoteTracks, err := a.remoteRandomPlaylistCandidates(serverURL, token)
	if err != nil {
		return localTracks, nil
	}

	return append(localTracks, remoteTracks...), nil
}

// localRandomPlaylistCandidates normalizes local DB rows into the randomTrack shape.
func (s *AppStore) localRandomPlaylistCandidates(ctx context.Context) ([]randomTrack, error) {
	rows, err := s.dq.ListAllLocalTracks(ctx)
	if err != nil {
		return nil, fmt.Errorf("list local tracks: %w", err)
	}

	candidates := make([]randomTrack, 0, len(rows))
	for _, t := range rows {
		if t.DurationMs <= 0 {
			continue
		}
		candidates = append(candidates, randomTrack{
			source:      "local_ref",
			localPath:   t.Path,
			title:       t.Title,
			album:       t.Album,
			albumArtist: t.AlbumArtist,
			durationMS:  t.DurationMs,
		})
	}

	return candidates, nil
}

// remoteRandomPlaylistCandidates normalizes remote tracks into the same shape as local tracks.
func (a *App) remoteRandomPlaylistCandidates(serverURL, token string) ([]randomTrack, error) {
	const pageSize = 200

	tracks, err := a.fetchAllRemoteTracks(serverURL, token, pageSize)
	if err != nil {
		return nil, err
	}

	candidates := make([]randomTrack, 0, len(tracks))
	for _, t := range tracks {
		if t.DurationMS <= 0 {
			continue
		}
		candidates = append(candidates, randomTrack{
			source:      "remote",
			trackID:     t.ID,
			title:       t.Title,
			album:       t.AlbumName,
			albumArtist: t.AlbumArtist,
			durationMS:  t.DurationMS,
		})
	}

	return candidates, nil
}

// dedupeRandomTracks removes duplicates using the same normalized metadata key
// used by the current random-playlist behavior.
func dedupeRandomTracks(candidates []randomTrack) []randomTrack {
	if len(candidates) == 0 {
		return nil
	}

	deduped := make([]randomTrack, 0, len(candidates))
	seen := make(map[string]struct{}, len(candidates))
	for _, candidate := range candidates {
		key := strings.ToLower(candidate.title) + "|" + strings.ToLower(candidate.album) + "|" + strings.ToLower(candidate.albumArtist)
		if _, exists := seen[key]; exists {
			continue
		}
		seen[key] = struct{}{}
		deduped = append(deduped, candidate)
	}

	return deduped
}
