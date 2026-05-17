package media

import (
	"context"
	"errors"
	"os"
	"strings"

	"github.com/studio-sound/studio/sidecar/internal/ipc"
)

// MapRunError converts a runner-layer error (and the captured stderr tail)
// into a structured RPC error.
func MapRunError(err error, stderrTail string) *ipc.RPCError {
	switch {
	case errors.Is(err, os.ErrNotExist):
		return ipc.NewRPCError(ipc.CodeFileNotFound, "media file not found")
	case errors.Is(err, os.ErrPermission):
		return ipc.NewRPCError(ipc.CodeAccessDenied, "permission denied opening media file")
	case errors.Is(err, context.DeadlineExceeded):
		return ipc.NewRPCError(ipc.CodeFFprobeFailure, "probe exceeded 10s deadline")
	case errors.Is(err, ErrFFprobeMissing):
		return ipc.NewRPCError(ipc.CodeFFprobeFailure, "ffprobe binary not located")
	}

	lower := strings.ToLower(stderrTail)
	switch {
	case strings.Contains(lower, "invalid data found"),
		strings.Contains(lower, "moov atom not found"),
		strings.Contains(lower, "end of file"):
		return ipc.NewRPCError(ipc.CodeCorruptMedia, "ffprobe could not parse the file")
	case strings.Contains(lower, "permission denied"):
		return ipc.NewRPCError(ipc.CodeAccessDenied, "permission denied reading media file")
	}

	return ipc.NewRPCError(ipc.CodeFFprobeFailure, "ffprobe failed: "+truncate(err.Error(), 256))
}

// MapParseError wraps a JSON-decode failure as CORRUPT_MEDIA.
func MapParseError(err error) *ipc.RPCError {
	return ipc.NewRPCError(ipc.CodeCorruptMedia, "ffprobe output parse failed: "+truncate(err.Error(), 256))
}

func truncate(s string, n int) string {
	if len(s) <= n {
		return s
	}
	return s[:n] + "..."
}
