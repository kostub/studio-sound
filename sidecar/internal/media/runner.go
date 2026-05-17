package media

import (
	"bytes"
	"context"
	"errors"
	"io"
	"os/exec"
)

const (
	stdoutCap     = 1 * 1024 * 1024
	stderrTailCap = 4 * 1024
)

type RunResult struct {
	Stdout     []byte
	ExitCode   int
	StderrTail string
}

// Run invokes ffprobe with the canonical Phase 3 arg set against mediaPath.
// Honours ctx cancellation by killing the ffprobe process group.
func Run(ctx context.Context, ffprobePath, mediaPath string) (*RunResult, error) {
	cmd := exec.CommandContext(ctx, ffprobePath,
		"-v", "error",
		"-hide_banner",
		"-print_format", "json",
		"-show_format",
		"-show_streams",
		maybePrefixLongPath(mediaPath),
	)
	setProcAttrs(cmd)
	cmd.Cancel = func() error { return killGroup(cmd) }

	var stdoutBuf cappedBuffer
	stdoutBuf.cap = stdoutCap
	var stderrTail tailBuffer
	stderrTail.cap = stderrTailCap

	cmd.Stdout = &stdoutBuf
	cmd.Stderr = &stderrTail

	err := cmd.Run()
	res := &RunResult{
		Stdout:     stdoutBuf.Bytes(),
		ExitCode:   exitCodeFromErr(err, cmd),
		StderrTail: stderrTail.String(),
	}
	if err != nil {
		return res, err
	}
	return res, nil
}

func exitCodeFromErr(err error, cmd *exec.Cmd) int {
	if err == nil {
		if cmd.ProcessState != nil {
			return cmd.ProcessState.ExitCode()
		}
		return 0
	}
	var ee *exec.ExitError
	if errors.As(err, &ee) {
		return ee.ExitCode()
	}
	return -1
}

type cappedBuffer struct {
	buf bytes.Buffer
	cap int
}

func (c *cappedBuffer) Write(p []byte) (int, error) {
	remaining := c.cap - c.buf.Len()
	if remaining <= 0 {
		return len(p), nil
	}
	if len(p) > remaining {
		p = p[:remaining]
	}
	return c.buf.Write(p)
}

func (c *cappedBuffer) Bytes() []byte { return c.buf.Bytes() }

// tailBuffer accumulates bytes but keeps only the last `cap` bytes written.
type tailBuffer struct {
	buf bytes.Buffer
	cap int
}

func (t *tailBuffer) Write(p []byte) (int, error) {
	n, err := t.buf.Write(p)
	if t.buf.Len() > t.cap {
		excess := t.buf.Len() - t.cap
		_, _ = io.CopyN(io.Discard, &t.buf, int64(excess))
	}
	return n, err
}

func (t *tailBuffer) String() string { return t.buf.String() }

// maybePrefixLongPath prepends \\?\ on Windows when the path is long enough
// to risk crossing the legacy MAX_PATH (260) boundary. No-op on non-Windows.
func maybePrefixLongPath(p string) string { return maybeLongPath(p) }
