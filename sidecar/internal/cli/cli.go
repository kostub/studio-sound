package cli

import (
	"fmt"
	"io"

	"github.com/studio-sound/studio/sidecar/internal/health"
)

func Run(args []string, stdout io.Writer, stderr io.Writer) int {
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
	default:
		_, _ = fmt.Fprintf(stderr, "unknown command: %s\n", args[0])
		return 2
	}
}
