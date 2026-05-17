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
