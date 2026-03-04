package writeback

import (
	"context"
	"fmt"
	"os/exec"
	"path/filepath"
	"strconv"
	"strings"

	"pneuma/internal/models"
)

// Writer writes metadata tags to audio files using external tools.
type Writer struct{}

// New creates a Writer.
func New() *Writer {
	return &Writer{}
}

// Write dispatches to the correct CLI tool based on the file extension.
func (w *Writer) Write(ctx context.Context, track *models.Track) error {
	ext := strings.ToLower(filepath.Ext(track.Path))
	switch ext {
	case ".mp3":
		return w.writeMp3(ctx, track)
	case ".flac":
		return w.writeFlac(ctx, track)
	case ".m4a", ".aac":
		return w.writeM4a(ctx, track)
	default:
		return w.writeFfmpeg(ctx, track)
	}
}

func (w *Writer) writeMp3(ctx context.Context, t *models.Track) error {
	args := []string{}
	if t.Title != "" {
		args = append(args, "--song", t.Title)
	}
	if t.AlbumArtist != "" {
		args = append(args, "--artist", t.AlbumArtist)
	}
	if t.Genre != "" {
		args = append(args, "--genre", t.Genre)
	}
	if t.Year != 0 {
		args = append(args, "--year", strconv.Itoa(t.Year))
	}
	if t.TrackNumber != 0 {
		args = append(args, "--track", strconv.Itoa(t.TrackNumber))
	}
	if len(args) == 0 {
		return nil
	}
	args = append(args, t.Path)
	cmd := exec.CommandContext(ctx, "mid3v2", args...)
	if out, err := cmd.CombinedOutput(); err != nil {
		return fmt.Errorf("mid3v2: %s: %w", string(out), err)
	}
	return nil
}

func (w *Writer) writeFlac(ctx context.Context, t *models.Track) error {
	args := []string{"--remove-all-tags"}
	if t.Title != "" {
		args = append(args, fmt.Sprintf("--set-tag=TITLE=%s", t.Title))
	}
	if t.AlbumArtist != "" {
		args = append(args, fmt.Sprintf("--set-tag=ARTIST=%s", t.AlbumArtist))
	}
	if t.Genre != "" {
		args = append(args, fmt.Sprintf("--set-tag=GENRE=%s", t.Genre))
	}
	if t.Year != 0 {
		args = append(args, fmt.Sprintf("--set-tag=DATE=%d", t.Year))
	}
	if t.TrackNumber != 0 {
		args = append(args, fmt.Sprintf("--set-tag=TRACKNUMBER=%d", t.TrackNumber))
	}
	args = append(args, t.Path)
	cmd := exec.CommandContext(ctx, "metaflac", args...)
	if out, err := cmd.CombinedOutput(); err != nil {
		return fmt.Errorf("metaflac: %s: %w", string(out), err)
	}
	return nil
}

func (w *Writer) writeM4a(ctx context.Context, t *models.Track) error {
	args := []string{}
	if t.Title != "" {
		args = append(args, "--title", t.Title)
	}
	if t.AlbumArtist != "" {
		args = append(args, "--artist", t.AlbumArtist)
	}
	if t.Genre != "" {
		args = append(args, "--genre", t.Genre)
	}
	if t.Year != 0 {
		args = append(args, "--year", strconv.Itoa(t.Year))
	}
	if t.TrackNumber != 0 {
		args = append(args, "--tracknum", strconv.Itoa(t.TrackNumber))
	}
	if len(args) == 0 {
		return nil
	}
	args = append(args, "--overWrite", t.Path)
	cmd := exec.CommandContext(ctx, "AtomicParsley", args...)
	if out, err := cmd.CombinedOutput(); err != nil {
		return fmt.Errorf("AtomicParsley: %s: %w", string(out), err)
	}
	return nil
}

func (w *Writer) writeFfmpeg(ctx context.Context, t *models.Track) error {
	args := []string{"-y", "-i", t.Path}
	if t.Title != "" {
		args = append(args, "-metadata", fmt.Sprintf("title=%s", t.Title))
	}
	if t.AlbumArtist != "" {
		args = append(args, "-metadata", fmt.Sprintf("artist=%s", t.AlbumArtist))
	}
	if t.Genre != "" {
		args = append(args, "-metadata", fmt.Sprintf("genre=%s", t.Genre))
	}
	if t.Year != 0 {
		args = append(args, "-metadata", fmt.Sprintf("date=%d", t.Year))
	}
	tmp := t.Path + ".tmp" + filepath.Ext(t.Path)
	args = append(args, "-c", "copy", tmp)
	cmd := exec.CommandContext(ctx, "ffmpeg", args...)
	if out, err := cmd.CombinedOutput(); err != nil {
		return fmt.Errorf("ffmpeg: %s: %w", string(out), err)
	}
	return exec.CommandContext(ctx, "mv", tmp, t.Path).Run()
}
