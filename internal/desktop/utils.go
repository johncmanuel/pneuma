package desktop

import (
	"crypto/sha256"
	"encoding/binary"
	"encoding/hex"
	"encoding/json"
	"fmt"
	"io"
	"os"
	"os/exec"
	"path/filepath"
	"strconv"
	"strings"
)

// durationCacheKey builds a cache key from the file's path, size, and mtime.
func durationCacheKey(path string, fi os.FileInfo) string {
	return path + "|" + strconv.FormatInt(fi.Size(), 10) + "|" + strconv.FormatInt(fi.ModTime().Unix(), 10)
}

// thumbCacheKey returns a short hex key derived from the file path, size, and
// mtime. Used only as an index into artworkHashCache — not as the disk filename.
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

// parseDurationFallbackLocal reads duration using pure Go for supported formats.
func parseDurationFallbackLocal(path string, fi os.FileInfo, lt *LocalTrack) {
	switch strings.ToLower(filepath.Ext(path)) {
	case ".flac":
		parseFLACDurationLocal(path, fi, lt)
	}
}

// parseFLACDurationLocal reads the FLAC STREAMINFO block to compute duration.
func parseFLACDurationLocal(path string, fi os.FileInfo, lt *LocalTrack) {
	// Check cache first.
	key := durationCacheKey(path, fi)
	if v, ok := durationCache.Load(key); ok {
		lt.DurationMs = v.(int64)
		return
	}

	f, err := os.Open(path)
	if err != nil {
		return
	}
	defer f.Close()

	magic := make([]byte, 4)
	if _, err := io.ReadFull(f, magic); err != nil {
		return
	}
	if string(magic) != "fLaC" {
		return
	}

	for {
		hdr := make([]byte, 4)
		if _, err := io.ReadFull(f, hdr); err != nil {
			return
		}
		isLast := hdr[0]&0x80 != 0
		blockType := hdr[0] & 0x7F
		blockLen := int(binary.BigEndian.Uint32([]byte{0, hdr[1], hdr[2], hdr[3]}))

		if blockType == 0 && blockLen >= 18 {
			data := make([]byte, blockLen)
			if _, err := io.ReadFull(f, data); err != nil {
				return
			}
			v := uint64(data[10])<<56 | uint64(data[11])<<48 |
				uint64(data[12])<<40 | uint64(data[13])<<32 |
				uint64(data[14])<<24 | uint64(data[15])<<16 |
				uint64(data[16])<<8 | uint64(data[17])
			sampleRate := int64(v >> 44)
			totalSamples := int64(v & 0x0000000FFFFFFFFF)
			if sampleRate > 0 && totalSamples > 0 {
				lt.DurationMs = totalSamples * 1000 / sampleRate
				durationCache.Store(key, lt.DurationMs)
			}
			return
		}

		if _, err := f.Seek(int64(blockLen), io.SeekCurrent); err != nil {
			return
		}
		if isLast {
			break
		}
	}
}
