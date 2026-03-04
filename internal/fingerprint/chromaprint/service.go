package chromaprint

import (
	"context"
	"fmt"
	"os/exec"
	"strings"
)

// Service wraps the fpcalc binary for fingerprinting audio files.
type Service struct {
	path string
}

// New creates a chromaprint Service. path is the location of the fpcalc binary.
func New(path string) *Service {
	return &Service{path: path}
}

// Available returns true if the fpcalc binary can be found and executed.
func (s *Service) Available() bool {
	if s.path == "" {
		return false
	}
	_, err := exec.LookPath(s.path)
	return err == nil
}

// Result holds the output of a fingerprint operation.
type Result struct {
	Duration    float64
	Fingerprint string
}

// Fingerprint runs fpcalc on the given audio file and returns the duration
// and raw fingerprint string.
func (s *Service) Fingerprint(ctx context.Context, path string) (*Result, error) {
	if s.path == "" {
		return nil, fmt.Errorf("fpcalc path not configured")
	}

	cmd := exec.CommandContext(ctx, s.path, "-plain", path)
	out, err := cmd.Output()
	if err != nil {
		return nil, fmt.Errorf("fpcalc: %w", err)
	}

	// fpcalc -plain outputs two lines: duration\nfingerprint
	lines := strings.SplitN(strings.TrimSpace(string(out)), "\n", 2)
	if len(lines) < 2 {
		return nil, fmt.Errorf("unexpected fpcalc output: %s", string(out))
	}

	var dur float64
	fmt.Sscanf(lines[0], "%f", &dur)

	return &Result{
		Duration:    dur,
		Fingerprint: lines[1],
	}, nil
}
