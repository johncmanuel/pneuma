package desktop

import "strings"

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
	pm, err := a.pm()
	if err != nil {
		return nil, err
	}
	return pm.GenerateRandomPlaylist(name, description, durationMinutes, useRemote)
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
