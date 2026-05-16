package ipc

import (
	"context"
	"io"
	"log/slog"
)

// Serve creates a Dispatcher and runs the IPC serve loop, reading JSON
// envelopes from stdin and writing responses to stdout. The log parameter is
// used for structured logging within the dispatcher; pass a logger from the
// logger package or a test logger.
func Serve(ctx context.Context, stdin io.Reader, stdout, stderr io.Writer, log *slog.Logger) error {
	d := NewDispatcher(log)
	return d.Serve(ctx, stdin, stdout)
}
