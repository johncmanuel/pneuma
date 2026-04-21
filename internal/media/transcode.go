package media

import (
	"context"
	"crypto/sha256"
	"encoding/hex"
	"fmt"
	"log/slog"
	"os"
	"os/exec"
	"path/filepath"
	"sort"
	"strings"
	"sync"
	"time"

	"pneuma/internal/models"
)

// StreamQuality represents the desired quality level for streaming a track.
// The options are: "auto", "low", "medium", "high", and "original".
type StreamQuality string

const (
	StreamQualityAuto     StreamQuality = "auto"
	StreamQualityLow      StreamQuality = "low"
	StreamQualityMedium   StreamQuality = "medium"
	StreamQualityHigh     StreamQuality = "high"
	StreamQualityOriginal StreamQuality = "original"
)

// TranscodeConfig holds configuration options for the StreamTranscoder.
type TranscodeConfig struct {
	FFmpegPath        string
	CacheDir          string
	CacheMaxSizeMB    int
	MaxConcurrentJobs int
}

// StreamTranscoder manages on-the-fly transcoding of audio tracks for streaming at different quality levels.
type StreamTranscoder struct {
	enabled       bool
	ffmpegPath    string
	cacheDir      string
	maxCacheBytes int64
	jobSem        chan struct{}
	log           *slog.Logger

	inFlight sync.Map
	pruneMu  sync.Mutex
}

// ParseStreamQuality converts a raw string input into a StreamQuality value, defaulting to StreamQualityOriginal for unrecognized inputs.
func ParseStreamQuality(raw string) StreamQuality {
	switch strings.ToLower(strings.TrimSpace(raw)) {
	case string(StreamQualityAuto):
		return StreamQualityAuto
	case string(StreamQualityLow):
		return StreamQualityLow
	case string(StreamQualityMedium):
		return StreamQualityMedium
	case string(StreamQualityHigh):
		return StreamQualityHigh
	case string(StreamQualityOriginal):
		return StreamQualityOriginal
	default:
		return StreamQualityOriginal
	}
}

// NormalizeStreamQuality maps StreamQualityAuto to StreamQualityMedium for transcoding purposes, while leaving other values unchanged.
func NormalizeStreamQuality(q StreamQuality) StreamQuality {
	if q == StreamQualityAuto {
		return StreamQualityMedium
	}
	return q
}

// targetBitrateKbps returns the target bitrate in kbps for a given StreamQuality level, or 0 if transcoding is not applicable.
// TODO: Consider making these bitrates configurable in TranscodeConfig.
func (q StreamQuality) targetBitrateKbps() int {
	switch NormalizeStreamQuality(q) {
	case StreamQualityLow:
		return 64
	case StreamQualityMedium:
		return 96
	case StreamQualityHigh:
		return 160
	default:
		return 0
	}
}

// shouldTranscode indicates whether transcoding should be performed for the given StreamQuality level.
func (q StreamQuality) shouldTranscode() bool {
	switch NormalizeStreamQuality(q) {
	case StreamQualityLow, StreamQualityMedium, StreamQualityHigh:
		return true
	default:
		return false
	}
}

// NewStreamTranscoder initializes a StreamTranscoder based on the provided TranscodeConfig.
// It checks for ffmpeg availability, ensures the cache directory is writable, and sets up concurrency limits.
// If any critical setup step fails, it returns a disabled transcoder that will bypass transcoding logic.
func NewStreamTranscoder(cfg TranscodeConfig) *StreamTranscoder {
	transcoderLog := slog.Default().With("component", "stream-transcoder")

	ffmpegPath := strings.TrimSpace(cfg.FFmpegPath)
	if ffmpegPath == "" {
		ffmpegPath = "ffmpeg"
	}

	resolvedFFmpegPath, err := exec.LookPath(ffmpegPath)
	if err != nil {
		transcoderLog.Warn("ffmpeg unavailable; stream transcoding disabled", "path", ffmpegPath, "err", err)
		return &StreamTranscoder{log: transcoderLog}
	}

	cacheDir := strings.TrimSpace(cfg.CacheDir)
	if cacheDir == "" {
		transcoderLog.Warn("transcode cache directory is empty; stream transcoding disabled")
		return &StreamTranscoder{log: transcoderLog}
	}

	if err := os.MkdirAll(cacheDir, 0o755); err != nil {
		transcoderLog.Warn("could not create transcode cache directory; stream transcoding disabled", "dir", cacheDir, "err", err)
		return &StreamTranscoder{log: transcoderLog}
	}

	maxCacheMB := cfg.CacheMaxSizeMB
	if maxCacheMB <= 0 {
		maxCacheMB = 2048
	}

	maxConcurrentJobs := cfg.MaxConcurrentJobs
	if maxConcurrentJobs <= 0 {
		maxConcurrentJobs = 1
	}

	transcoderLog.Info(
		"stream transcoding enabled",
		"ffmpeg", resolvedFFmpegPath,
		"cache_dir", cacheDir,
		"cache_max_size_mb", maxCacheMB,
		"max_concurrent_jobs", maxConcurrentJobs,
	)

	return &StreamTranscoder{
		enabled:       true,
		ffmpegPath:    resolvedFFmpegPath,
		cacheDir:      cacheDir,
		maxCacheBytes: int64(maxCacheMB) << 20,
		jobSem:        make(chan struct{}, maxConcurrentJobs),
		log:           transcoderLog,
	}
}

// ResolveCachedPath checks if a transcoded version of the track exists in the cache for the given quality level.
// If it exists, it returns the path to the cached file and true. If not, it returns an empty string and false.
func (t *StreamTranscoder) ResolveCachedPath(track *models.Track, sourcePath string, sourceInfo os.FileInfo, quality StreamQuality) (string, bool) {
	if !t.canTranscode(track, quality) {
		return "", false
	}

	quality = NormalizeStreamQuality(quality)
	cachePath := t.cachePath(sourcePath, sourceInfo, quality)
	if _, err := os.Stat(cachePath); err != nil {
		return "", false
	}

	now := time.Now()
	_ = os.Chtimes(cachePath, now, now)
	return cachePath, true
}

// QueueTranscode initiates a background transcoding job for the given track and quality level if transcoding is needed and not already in progress.
func (t *StreamTranscoder) QueueTranscode(track *models.Track, sourcePath string, sourceInfo os.FileInfo, quality StreamQuality) {
	if !t.canTranscode(track, quality) {
		return
	}

	quality = NormalizeStreamQuality(quality)
	cachePath := t.cachePath(sourcePath, sourceInfo, quality)
	if _, err := os.Stat(cachePath); err == nil {
		return
	}

	if _, loaded := t.inFlight.LoadOrStore(cachePath, struct{}{}); loaded {
		return
	}

	targetBitrate := quality.targetBitrateKbps()

	go func(trackID, inputPath, outputPath string, bitrate int) {
		defer t.inFlight.Delete(outputPath)

		t.jobSem <- struct{}{}
		defer func() {
			<-t.jobSem
		}()

		if err := t.transcodeToOpus(inputPath, outputPath, bitrate); err != nil {
			t.log.Warn("background transcode failed", "track_id", trackID, "quality", quality, "err", err)
			return
		}

		t.pruneCacheIfNeeded()
	}(track.ID, sourcePath, cachePath, targetBitrate)
}

// canTranscode determines if the given track should be transcoded for the requested quality level
// based on codec, bitrate, and transcoding settings.
func (t *StreamTranscoder) canTranscode(track *models.Track, quality StreamQuality) bool {
	if t == nil || !t.enabled || track == nil {
		return false
	}

	quality = NormalizeStreamQuality(quality)
	if !quality.shouldTranscode() {
		return false
	}

	targetBitrate := quality.targetBitrateKbps()
	if targetBitrate <= 0 {
		return false
	}

	return shouldTranscodeTrack(track, targetBitrate)
}

// shouldTranscodeTrack evaluates whether a specific track should be transcoded based on its codec
// and bitrate relative to the target bitrate.
func shouldTranscodeTrack(track *models.Track, targetBitrate int) bool {
	if IsLosslessCodec(strings.TrimSpace(track.Codec)) {
		return true
	}

	if track.BitrateKbps <= 0 {
		return true
	}

	// Allow some headroom above the target bitrate to avoid unnecessary transcoding for tracks that are close enough in quality.
	return track.BitrateKbps > targetBitrate+32
}

// cachePath generates a unique cache file path for a given source track and
// quality level based on the track's metadata and requested quality.
func (t *StreamTranscoder) cachePath(sourcePath string, sourceInfo os.FileInfo, quality StreamQuality) string {
	basis := fmt.Sprintf("v1|%s|%d|%d|%s", sourcePath, sourceInfo.Size(), sourceInfo.ModTime().UnixNano(), quality)
	sum := sha256.Sum256([]byte(basis))
	key := hex.EncodeToString(sum[:12])
	fileName := fmt.Sprintf("opus-%s-%s.ogg", quality, key)
	return filepath.Join(t.cacheDir, fileName)
}

// transcodeToOpus performs the actual transcoding of the input audio file to an Opus-encoded OGG
// file at the specified bitrate using ffmpeg.
func (t *StreamTranscoder) transcodeToOpus(inputPath, outputPath string, bitrateKbps int) error {
	if err := os.MkdirAll(t.cacheDir, 0o755); err != nil {
		return fmt.Errorf("create cache dir: %w", err)
	}

	tmpPath := outputPath + ".tmp"
	_ = os.Remove(tmpPath)

	ctx, cancel := context.WithTimeout(context.Background(), 15*time.Minute)
	defer cancel()

	// ffmpeg args
	args := []string{
		"-hide_banner",
		"-loglevel", "error",
		"-nostdin",
		"-y",
		"-i", inputPath,
		"-vn",
		"-map_metadata", "-1",
		"-c:a", "libopus",
		"-b:a", fmt.Sprintf("%dk", bitrateKbps),
		"-vbr", "on",
		"-compression_level", "10",
		"-application", "audio",
		"-f", "ogg",
		tmpPath,
	}

	cmd := exec.CommandContext(ctx, t.ffmpegPath, args...)
	if out, err := cmd.CombinedOutput(); err != nil {
		_ = os.Remove(tmpPath)
		return fmt.Errorf("ffmpeg transcode failed: %w (%s)", err, strings.TrimSpace(string(out)))
	}

	if info, err := os.Stat(tmpPath); err != nil || info.Size() <= 0 {
		_ = os.Remove(tmpPath)
		if err != nil {
			return fmt.Errorf("transcode output stat failed: %w", err)
		}
		return fmt.Errorf("transcode output file is empty")
	}

	if err := os.Rename(tmpPath, outputPath); err != nil {
		_ = os.Remove(tmpPath)
		return fmt.Errorf("promote transcode artifact: %w", err)
	}

	return nil
}

// pruneCacheIfNeeded checks the total size of cached transcoded files and removes
// the least recently modified ones if the total exceeds the configured maximum cache size.
func (t *StreamTranscoder) pruneCacheIfNeeded() {
	if t.maxCacheBytes <= 0 {
		return
	}

	t.pruneMu.Lock()
	defer t.pruneMu.Unlock()

	entries, err := os.ReadDir(t.cacheDir)
	if err != nil {
		return
	}

	type cacheEntry struct {
		path    string
		size    int64
		modTime time.Time
	}

	var (
		cacheEntries []cacheEntry
		totalBytes   int64
	)

	for _, entry := range entries {
		if entry.IsDir() {
			continue
		}

		name := entry.Name()
		// Only consider files that match the expected cache file naming pattern to avoid
		// accidentally deleting unrelated files in the cache directory.
		if !strings.HasPrefix(name, "opus-") || !strings.HasSuffix(name, ".ogg") {
			continue
		}

		info, err := entry.Info()
		if err != nil {
			continue
		}

		totalBytes += info.Size()
		cacheEntries = append(cacheEntries, cacheEntry{
			path:    filepath.Join(t.cacheDir, name),
			size:    info.Size(),
			modTime: info.ModTime(),
		})
	}

	if totalBytes <= t.maxCacheBytes {
		return
	}

	sort.Slice(cacheEntries, func(i, j int) bool {
		return cacheEntries[i].modTime.Before(cacheEntries[j].modTime)
	})

	for _, entry := range cacheEntries {
		if totalBytes <= t.maxCacheBytes {
			break
		}

		if err := os.Remove(entry.path); err != nil {
			continue
		}

		totalBytes -= entry.size
	}
}
