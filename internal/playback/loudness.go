package playback

import (
	"context"
	"encoding/json"
	"fmt"
	"os/exec"
	"strconv"
	"strings"
)

// Analyzer computes ReplayGain / EBU R128 loudness values using ffmpeg's
// loudnorm filter. Values are stored on the track and used by the frontend
// player to set gain before audio output, keeping perceived volume consistent
// across tracks and albums.
type Analyzer struct {
	ffmpegPath string
}

// NewAnalyzer returns an Analyzer that invokes ffmpegPath.
func NewAnalyzer(ffmpegPath string) *Analyzer {
	if ffmpegPath == "" {
		ffmpegPath = "ffmpeg"
	}
	return &Analyzer{ffmpegPath: ffmpegPath}
}

// LoudnessResult holds the integrated loudness and gain offset for a track.
type LoudnessResult struct {
	// IntegratedLUFS is the program loudness in LUFS (EBU R128).
	IntegratedLUFS float64
	// TruePeakDBFS is the measured true peak level.
	TruePeakDBFS float64
	// GainDB is the recommended gain adjustment to reach -14 LUFS
	// (the streaming de facto standard, close to Spotify's -14 LUFS target).
	GainDB float64
}

const targetLUFS = -14.0

// Analyse runs a two-pass ffmpeg loudnorm measurement on the file at path and
// returns the loudness result. This is CPU-intensive; run in a goroutine.
func (a *Analyzer) Analyse(ctx context.Context, path string) (*LoudnessResult, error) {
	if _, err := exec.LookPath(a.ffmpegPath); err != nil {
		return nil, fmt.Errorf("loudness: ffmpeg not found: %w", err)
	}

	// Pass 1: measure loudness (no audio output).
	args := []string{
		"-hide_banner",
		"-i", path,
		"-af", "loudnorm=print_format=json",
		"-f", "null", "-",
	}
	out, err := exec.CommandContext(ctx, a.ffmpegPath, args...).CombinedOutput()
	if err != nil {
		return nil, fmt.Errorf("loudness ffmpeg: %w\n%s", err, out)
	}

	return parseFFmpegLoudnorm(string(out))
}

// ─── Parser ──────────────────────────────────────────────────────────────────

type loudnormJSON struct {
	InputI       string `json:"input_i"`
	InputTP      string `json:"input_tp"`
	InputLRA     string `json:"input_lra"`
	InputThresh  string `json:"input_thresh"`
	OutputI      string `json:"output_i"`
	OutputTP     string `json:"output_tp"`
	NormType     string `json:"normalization_type"`
	TargetOffset string `json:"target_offset"`
}

func parseFFmpegLoudnorm(output string) (*LoudnessResult, error) {
	// ffmpeg writes the JSON block to stderr after the last line of analysis.
	start := strings.LastIndex(output, "{")
	end := strings.LastIndex(output, "}")
	if start < 0 || end < 0 || end <= start {
		return nil, fmt.Errorf("loudness: cannot find JSON in ffmpeg output")
	}

	var data loudnormJSON
	if err := json.Unmarshal([]byte(output[start:end+1]), &data); err != nil {
		return nil, fmt.Errorf("loudness: parse JSON: %w", err)
	}

	lufs, err := strconv.ParseFloat(strings.TrimSpace(data.InputI), 64)
	if err != nil {
		return nil, fmt.Errorf("loudness: parse LUFS %q: %w", data.InputI, err)
	}
	tp, _ := strconv.ParseFloat(strings.TrimSpace(data.InputTP), 64)

	return &LoudnessResult{
		IntegratedLUFS: lufs,
		TruePeakDBFS:   tp,
		GainDB:         targetLUFS - lufs,
	}, nil
}
