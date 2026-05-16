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
//
// Handlers for the standard methods (system.ping, system.echo, etc.) are
// registered by the caller (cli package) to avoid an import cycle between the
// ipc package and the ipc/handlers sub-package.
func Serve(ctx context.Context, stdin io.Reader, stdout io.Writer, log *slog.Logger, setup func(*Dispatcher)) error {
	d := NewDispatcher(log)
	if setup != nil {
		setup(d)
	}
	return d.Serve(ctx, stdin, stdout)
}
