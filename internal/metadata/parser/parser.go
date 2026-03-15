package parser

import (
	"context"
	"encoding/json"
	"fmt"
	"log/slog"
	"os"
	"os/exec"
	"path/filepath"
	"strconv"
	"strings"
	"time"

	"github.com/dhowden/tag"
	"github.com/google/uuid"
	"github.com/mewkiz/flac"

	"pneuma/internal/models"
)

// Parser reads audio file metadata.
type Parser struct {
	ffprobePath string
}

// New creates a Parser. ffmpegOrProbePath may point to ffmpeg or ffprobe;
// if it looks like ffmpeg, we derive the ffprobe path automatically.
func New(ffmpegOrProbePath string) *Parser {
	p := ffmpegOrProbePath

	if strings.HasSuffix(p, "ffmpeg") {
		p = p[:len(p)-len("ffmpeg")] + "ffprobe"
	}
	// Verify ffprobe is available; fall back to bare "ffprobe" in PATH
	if p != "" {
		if _, err := exec.LookPath(p); err != nil {
			if alt, err2 := exec.LookPath("ffprobe"); err2 == nil {
				p = alt
			} else {
				p = "" // disable probing
			}
		}
	}
	return &Parser{ffprobePath: p}
}

// ParseFile extracts basic tag metadata from path.
func (p *Parser) ParseFile(_ context.Context, path string) (*models.Track, error) {
	f, err := os.Open(path)
	if err != nil {
		return nil, fmt.Errorf("open %s: %w", path, err)
	}
	defer f.Close()

	info, err := f.Stat()
	if err != nil {
		return nil, err
	}

	now := time.Now()
	t := &models.Track{
		ID:            uuid.NewString(),
		Path:          path,
		Title:         titleFromPath(path),
		Codec:         codecFromPath(path),
		FileSizeBytes: info.Size(),
		LastModified:  info.ModTime(),
		CreatedAt:     now,
		UpdatedAt:     now,
	}

	m, err := tag.ReadFrom(f)
	if err != nil {
		// No tags — return with filename-derived title.
		return t, nil
	}

	if m.Title() != "" {
		t.Title = m.Title()
	}
	t.AlbumArtist = m.AlbumArtist()
	t.AlbumName = m.Album()
	t.Genre = m.Genre()
	t.Year = m.Year()
	tn, _ := m.Track()
	dn, _ := m.Disc()
	t.TrackNumber = tn
	t.DiscNumber = dn

	// Store raw artist/album names; the library service resolves IDs.
	if m.AlbumArtist() == "" && m.Artist() != "" {
		t.AlbumArtist = m.Artist()
	}

	// Enrich with ffprobe (duration, bitrate, sample rate).
	if p.ffprobePath != "" {
		_ = p.probe(context.Background(), path, t) // best-effort
	}
	// Pure-Go fallback when ffprobe is unavailable or returned no duration.
	if t.DurationMS == 0 {
		_ = parseDurationFallback(path, t)
	}

	return t, nil
}

// ParseMeta holds raw metadata strings that don't map directly to Track fields.
type ParseMeta struct {
	ArtistName string
	AlbumName  string
}

// ParseFileWithMeta returns both the Track and the raw artist/album names.
func (p *Parser) ParseFileWithMeta(ctx context.Context, path string) (*models.Track, *ParseMeta, error) {
	f, err := os.Open(path)
	if err != nil {
		return nil, nil, fmt.Errorf("open %s: %w", path, err)
	}
	defer f.Close()

	info, err := f.Stat()
	if err != nil {
		return nil, nil, err
	}

	now := time.Now()
	t := &models.Track{
		ID:            uuid.NewString(),
		Path:          path,
		Title:         titleFromPath(path),
		Codec:         codecFromPath(path),
		FileSizeBytes: info.Size(),
		LastModified:  info.ModTime(),
		CreatedAt:     now,
		UpdatedAt:     now,
	}

	meta := &ParseMeta{}

	m, err := tag.ReadFrom(f)
	if err == nil {
		if m.Title() != "" {
			t.Title = m.Title()
		}
		meta.ArtistName = m.Artist()
		meta.AlbumName = m.Album()
		t.AlbumArtist = m.AlbumArtist()
		t.Genre = m.Genre()
		t.Year = m.Year()
		tn, _ := m.Track()
		dn, _ := m.Disc()
		t.TrackNumber = tn
		t.DiscNumber = dn
	}

	// use ffprobe to fill in duration, bitrate, sample rate, and channels if available.
	if p.ffprobePath != "" {
		_ = p.probe(ctx, path, t)
	}

	if t.DurationMS == 0 {
		_ = parseDurationFallback(path, t)
	}

	return t, meta, nil
}

// probe runs ffprobe and fills duration, bitrate, sample_rate, channels.
func (p *Parser) probe(ctx context.Context, path string, t *models.Track) error {
	cmd := exec.CommandContext(ctx, p.ffprobePath,
		"-v", "quiet",
		"-print_format", "json",
		"-show_format",
		"-show_streams",
		path,
	)
	out, err := cmd.Output()
	if err != nil {
		return fmt.Errorf("ffprobe: %w", err)
	}

	var result struct {
		Format struct {
			Duration string `json:"duration"`
			BitRate  string `json:"bit_rate"`
		} `json:"format"`
		Streams []struct {
			SampleRate string `json:"sample_rate"`
			Channels   int    `json:"channels"`
		} `json:"streams"`
	}
	if err := json.Unmarshal(out, &result); err != nil {
		return fmt.Errorf("parse ffprobe json: %w", err)
	}

	if dur, err := strconv.ParseFloat(result.Format.Duration, 64); err == nil {
		t.DurationMS = int64(dur * 1000)
	}
	if br, err := strconv.Atoi(result.Format.BitRate); err == nil {
		t.BitrateKbps = br / 1000
	}
	if len(result.Streams) > 0 {
		s := result.Streams[0]
		if sr, err := strconv.Atoi(s.SampleRate); err == nil {
			t.SampleRateHz = sr
		}
	}
	return nil
}

func codecFromPath(path string) string {
	ext := strings.ToLower(filepath.Ext(path))
	switch ext {
	case ".mp3":
		return "mp3"
	case ".flac":
		return "flac"
	case ".ogg":
		return "vorbis"
	case ".opus":
		return "opus"
	case ".m4a", ".aac":
		return "aac"
	case ".wav", ".aiff":
		return "pcm"
	case ".wv":
		return "wavpack"
	case ".ape":
		return "ape"
	default:
		return "unknown"
	}
}

// parseDurationFallback reads duration from the audio file if ffprobe
// is unavailable.
func parseDurationFallback(path string, t *models.Track) error {
	switch strings.ToLower(filepath.Ext(path)) {
	case ".flac":
		return parseFLACDuration(path, t)
	}
	return nil
}

// parseFLACDuration reads the FLAC STREAMINFO block and computes duration.
func parseFLACDuration(path string, t *models.Track) error {
	stream, err := flac.Open(path)
	if err != nil {
		slog.Error("Unable to obtain FLAC stream info", "path", path, "err", err)
		return err
	}
	defer stream.Close()

	if stream.Info != nil && stream.Info.SampleRate > 0 && stream.Info.NSamples > 0 {
		// convert to MS
		t.DurationMS = int64(stream.Info.NSamples) * 1000 / int64(stream.Info.SampleRate)
		return nil
	}
	return fmt.Errorf("FLAC STREAMINFO not found or incomplete")
}

func titleFromPath(path string) string {
	name := filepath.Base(path)
	return strings.TrimSuffix(name, filepath.Ext(name))
}
