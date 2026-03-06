package scanner

import (
	"crypto/sha256"
	"encoding/hex"
	"fmt"
	"io"
	"os"
	"strconv"
	"sync"
)

// hashCache avoids re-reading files whose path, size, and mtime are unchanged.
// Key format: "path|size|mtime_unix".
var hashCache sync.Map

// contentHash returns the SHA-256 hash of the file at path as "sha256:<hex>".
// Results are cached by (path, size, mtime) so only new or modified files
// pay the full read cost across repeated scan intervals.
func contentHash(path string) (string, error) {
	fi, err := os.Stat(path)
	if err != nil {
		return "", fmt.Errorf("contentHash stat: %w", err)
	}
	cacheKey := path + "|" + strconv.FormatInt(fi.Size(), 10) + "|" + strconv.FormatInt(fi.ModTime().Unix(), 10)

	if v, ok := hashCache.Load(cacheKey); ok {
		return v.(string), nil
	}

	f, err := os.Open(path)
	if err != nil {
		return "", fmt.Errorf("contentHash open: %w", err)
	}
	defer f.Close()

	h := sha256.New()
	if _, err := io.Copy(h, f); err != nil {
		return "", fmt.Errorf("contentHash read: %w", err)
	}
	result := "sha256:" + hex.EncodeToString(h.Sum(nil))
	hashCache.Store(cacheKey, result)
	return result, nil
}

// contentHashFile computes the SHA-256 hash by reading from an already-opened
// file, then seeks it back to the start so the caller can continue reading.
// The result is stored in hashCache under the given key.
func contentHashFile(f *os.File, cacheKey string) (string, error) {
	h := sha256.New()
	if _, err := io.Copy(h, f); err != nil {
		return "", fmt.Errorf("contentHashFile read: %w", err)
	}
	if _, err := f.Seek(0, io.SeekStart); err != nil {
		return "", fmt.Errorf("contentHashFile seek: %w", err)
	}
	result := "sha256:" + hex.EncodeToString(h.Sum(nil))
	hashCache.Store(cacheKey, result)
	return result, nil
}
