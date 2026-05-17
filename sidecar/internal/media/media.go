// Package media wraps a bundled ffprobe subprocess and produces the
// canonical MediaProbeResult shape consumed by the media.probe IPC method.
//
// The package layers are:
//   - locator.go: resolve the ffprobe binary path
//   - runner.go (+ _unix/_windows): invoke ffprobe with bounded I/O and
//     process-group cancellation
//   - parser.go: decode ffprobe JSON tolerantly
//   - normalize.go: project the parsed output onto the wire-stable
//     MediaProbeResult; audio is object-or-nil (encoded as JSON null when
//     no audio stream is present)
//   - compat.go: classify the file against the allow-list; unsupported
//     produces supported=false + descriptive issues (NOT an RPC error)
//   - errors.go: map runner / parse failures into RPC errors
//   - media.go: the public Probe orchestrator
package media

import (
	"context"
	"os"
)

// Probe runs the full pipeline: stat → run → parse → normalize → evaluate.
// Returns (*MediaProbeResult, nil) on success — even when
// result.Compatibility.Supported is false (unsupported is a successful
// probe, not a runner failure). Returns (nil, *ipc.RPCError) only when the
// file cannot be opened, ffprobe cannot run, or its output cannot be parsed.
func Probe(ctx context.Context, ffprobePath, mediaPath string) (*MediaProbeResult, error) {
	info, err := os.Stat(mediaPath)
	if err != nil {
		return nil, MapRunError(err, "")
	}

	runRes, runErr := Run(ctx, ffprobePath, mediaPath)
	if runErr != nil {
		stderrTail := ""
		if runRes != nil {
			stderrTail = runRes.StderrTail
		}
		return nil, MapRunError(runErr, stderrTail)
	}

	parsed, parseErr := Parse(runRes.Stdout)
	if parseErr != nil {
		return nil, MapParseError(parseErr)
	}

	result, _ := Normalize(parsed, mediaPath, int(info.Size()))
	v := Evaluate(result)
	result.Compatibility = Compatibility{
		Supported: v.Supported,
		Issues:    v.Issues,
		Warnings:  v.Warnings,
	}
	return result, nil
}
