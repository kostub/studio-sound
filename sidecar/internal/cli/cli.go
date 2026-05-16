package cli

import (
	"context"
	"flag"
	"fmt"
	"io"
	"os"

	"github.com/studio-sound/studio/sidecar/internal/buildinfo"
	"github.com/studio-sound/studio/sidecar/internal/health"
	"github.com/studio-sound/studio/sidecar/internal/ipc"
	"github.com/studio-sound/studio/sidecar/internal/logger"
)

func Run(args []string, stdin io.Reader, stdout io.Writer, stderr io.Writer) int {
	if len(args) == 0 {
		_, _ = fmt.Fprintln(stderr, "missing command")
		return 2
	}

	switch args[0] {
	case "health":
		if err := health.Write(stdout); err != nil {
			_, _ = fmt.Fprintf(stderr, "failed to encode health response: %v\n", err)
			return 1
		}
		return 0

	case "serve":
		// Parse flags for the serve subcommand.
		fs := flag.NewFlagSet("serve", flag.ContinueOnError)
		fs.SetOutput(stderr)
		logFile := fs.String("log-file", "", "path to log file (default: stderr)")
		if err := fs.Parse(args[1:]); err != nil {
			return 2
		}

		// Allow STUDIO_LOG_FILE env var as a fallback when the flag is not set.
		logPath := *logFile
		if logPath == "" {
			logPath = os.Getenv("STUDIO_LOG_FILE")
		}

		log := logger.New(logPath)
		log.Info("sidecar starting", "version", buildinfo.Version)

		ctx, cancel := context.WithCancel(context.Background())
		defer cancel()

		if err := ipc.Serve(ctx, stdin, stdout, stderr, log); err != nil && err != io.EOF {
			_, _ = fmt.Fprintf(stderr, "serve failed: %v\n", err)
			return 1
		}
		return 0

	default:
		_, _ = fmt.Fprintf(stderr, "unknown command: %s\n", args[0])
		return 2
	}
}
