package scanner

import (
	"crypto/sha256"
	"encoding/hex"
	"fmt"
	"io"
	"os"
)

// contentHash computes the SHA-256 hash of the file at path and returns it
// as the string "sha256:<hex>". This is used as a reliable dedup key for
// exact file copies regardless of tags or encoding metadata.
func contentHash(path string) (string, error) {
	f, err := os.Open(path)
	if err != nil {
		return "", fmt.Errorf("contentHash open: %w", err)
	}
	defer f.Close()

	h := sha256.New()
	if _, err := io.Copy(h, f); err != nil {
		return "", fmt.Errorf("contentHash read: %w", err)
	}
	return "sha256:" + hex.EncodeToString(h.Sum(nil)), nil
}
