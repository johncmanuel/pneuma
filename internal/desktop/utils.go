package desktop

import (
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"fmt"
	"log/slog"
	"os"
	"os/exec"
	"path/filepath"
	"strconv"
	"strings"

	"github.com/mewkiz/flac"
)

// durationCacheKey builds a cache key from the file's path, size, and mtime.
func durationCacheKey(path string, fi os.FileInfo) string {
	return path + "|" + strconv.FormatInt(fi.Size(), 10) + "|" + strconv.FormatInt(fi.ModTime().Unix(), 10)
}

// thumbCacheKey returns a short hex key derived from the file path, size, and mtime.
// Used only as an index into artworkHashCache, not as the disk filename.
func thumbCacheKey(path string, info os.FileInfo) string {
	h := sha256.New()
	fmt.Fprintf(h, "%s|%d|%d", path, info.Size(), info.ModTime().UnixNano())
	return hex.EncodeToString(h.Sum(nil))[:16]
}

// probeLocalDuration shells out to ffprobe to read duration for a local track.
// Results are cached by path+size+mtime so subsequent scans skip the exec.
func probeLocalDuration(path string, fi os.FileInfo, lt *LocalTrack) {
	key := durationCacheKey(path, fi)
	if v, ok := durationCache.Load(key); ok {
		lt.DurationMs = v.(int64)
		return
	}

	if ffprobePath == "" {
		return
	}

	cmd := exec.Command(ffprobePath,
		"-v", "quiet",
		"-print_format", "json",
		"-show_format",
		path,
	)

	out, err := cmd.Output()
	if err != nil {
		return
	}

	var result struct {
		Format struct {
			Duration string `json:"duration"`
		} `json:"format"`
	}

	if err := json.Unmarshal(out, &result); err != nil {
		return
	}

	if dur, err := strconv.ParseFloat(result.Format.Duration, 64); err == nil {
		lt.DurationMs = int64(dur * 1000)
		durationCache.Store(key, lt.DurationMs)
	}
}

// parseDurationFallbackLocal reads duration if ffprobe is unavailable.
func parseDurationFallbackLocal(path string, fi os.FileInfo, lt *LocalTrack) {
	switch strings.ToLower(filepath.Ext(path)) {
	case ".flac":
		parseFLACDurationLocal(path, fi, lt)
	}
}

// parseFLACDurationLocal reads the FLAC STREAMINFO block to compute duration.
func parseFLACDurationLocal(path string, fi os.FileInfo, lt *LocalTrack) {
	key := durationCacheKey(path, fi)
	if v, ok := durationCache.Load(key); ok {
		lt.DurationMs = v.(int64)
		return
	}

	stream, err := flac.Open(path)
	if err != nil {
		slog.Error("Unable to obtain FLAC stream info", "path", path, "err", err)
		return
	}
	defer stream.Close()

	if stream.Info != nil && stream.Info.SampleRate > 0 && stream.Info.NSamples > 0 {
		// convert to MS
		lt.DurationMs = int64(stream.Info.NSamples) * 1000 / int64(stream.Info.SampleRate)
		durationCache.Store(key, lt.DurationMs)
	}
}
