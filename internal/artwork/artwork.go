package artwork

import (
	"bytes"
	"fmt"
	"image"
	"image/jpeg"
	_ "image/png"
	"os"
	"path/filepath"

	"golang.org/x/image/draw"
)

// ResizeToThumbnail decodes raw image bytes, scales the image down to
// maxDim (preserving aspect ratio), and returns the result encoded as a JPEG.
// If the image is already within maxDim it is only re-encoded.
func ResizeToThumbnail(raw []byte, maxDim int) ([]byte, error) {
	src, _, err := image.Decode(bytes.NewReader(raw))
	if err != nil {
		return nil, fmt.Errorf("decode image: %w", err)
	}

	b := src.Bounds()
	srcW, srcH := b.Dx(), b.Dy()
	dstW, dstH := srcW, srcH

	if srcW > maxDim || srcH > maxDim {
		if srcW >= srcH {
			dstW = maxDim
			dstH = srcH * maxDim / srcW
		} else {
			dstH = maxDim
			dstW = srcW * maxDim / srcH
		}
	}

	if dstW < 1 {
		dstW = 1
	}
	if dstH < 1 {
		dstH = 1
	}

	dst := image.NewNRGBA(image.Rect(0, 0, dstW, dstH))
	draw.BiLinear.Scale(dst, dst.Bounds(), src, b, draw.Over, nil)

	var buf bytes.Buffer
	if err := jpeg.Encode(&buf, dst, &jpeg.Options{Quality: 85}); err != nil {
		return nil, fmt.Errorf("encode thumbnail: %w", err)
	}

	return buf.Bytes(), nil
}

// WriteThumbnail atomically writes data to dir/fileName using a temp file
// and renames it so that a crash mid-write never leaves a corrupt file behind.
func WriteThumbnail(dir, fileName string, data []byte) error {
	thumbPath := filepath.Join(dir, fileName)

	tmp, err := os.CreateTemp(dir, "tmp-thumb-*.jpg")
	if err != nil {
		return fmt.Errorf("temp file: %w", err)
	}
	tmpName := tmp.Name()

	if _, err := tmp.Write(data); err != nil {
		tmp.Close()
		os.Remove(tmpName)
		return fmt.Errorf("write thumbnail: %w", err)
	}
	tmp.Close()

	if err := os.Rename(tmpName, thumbPath); err != nil {
		os.Remove(tmpName)
		return fmt.Errorf("rename thumbnail: %w", err)
	}

	return nil
}
