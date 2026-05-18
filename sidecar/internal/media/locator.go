package media

import (
	"errors"
	"os"
)

// ErrFFprobeMissing is returned by ResolveFFprobe when the bundled ffprobe
// binary cannot be located via the STUDIO_FFPROBE_PATH env var.
var ErrFFprobeMissing = errors.New("ffprobe binary not found")

// ResolveFFprobe reads STUDIO_FFPROBE_PATH and verifies the file exists.
// It never consults $PATH — we only ever run the binary that the supervisor
// bundled and explicitly passed to us.
func ResolveFFprobe() (string, error) {
	path := os.Getenv("STUDIO_FFPROBE_PATH")
	if path == "" {
		return "", ErrFFprobeMissing
	}
	info, err := os.Stat(path)
	if err != nil || info.IsDir() {
		return "", ErrFFprobeMissing
	}
	return path, nil
}
