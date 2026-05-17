package media

import (
	"context"
	"os"
	"os/exec"
	"path/filepath"
	"runtime"
	"strconv"
	"strings"
	"testing"
	"time"
)

// buildFake builds the fakeffprobe helper into a temp dir and returns its path.
func buildFake(t *testing.T) string {
	t.Helper()
	tmp := t.TempDir()
	out := filepath.Join(tmp, "fakeffprobe")
	if runtime.GOOS == "windows" {
		out += ".exe"
	}
	cmd := exec.Command("go", "build", "-o", out, "../../cmd/fakeffprobe")
	cmd.Stderr = os.Stderr
	if err := cmd.Run(); err != nil {
		t.Fatalf("failed to build fakeffprobe: %v", err)
	}
	return out
}

func TestRun_SuccessReturnsStdout(t *testing.T) {
	fake := buildFake(t)
	t.Setenv("FAKE_FFPROBE_STDOUT", `{"ok":true}`)
	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()
	r, err := Run(ctx, fake, "ignored.mp4")
	if err != nil {
		t.Fatalf("unexpected err: %v", err)
	}
	if string(r.Stdout) != `{"ok":true}` {
		t.Errorf("got stdout %q", string(r.Stdout))
	}
	if r.ExitCode != 0 {
		t.Errorf("got exit %d, want 0", r.ExitCode)
	}
}

func TestRun_NonZeroExitPreservesStderrTail(t *testing.T) {
	fake := buildFake(t)
	t.Setenv("FAKE_FFPROBE_STDERR", "Invalid data found when processing input")
	t.Setenv("FAKE_FFPROBE_EXIT", "1")
	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()
	r, err := Run(ctx, fake, "ignored.mp4")
	if err == nil {
		t.Fatal("expected non-nil err on nonzero exit")
	}
	if r == nil || r.ExitCode != 1 {
		t.Errorf("got exit %v, want 1", r)
	}
	if !strings.Contains(r.StderrTail, "Invalid data") {
		t.Errorf("got stderr tail %q", r.StderrTail)
	}
}

func TestRun_OversizeStdoutTruncatedToCap(t *testing.T) {
	fake := buildFake(t)
	t.Setenv("FAKE_FFPROBE_STDOUT_BYTES", strconv.Itoa(2*1024*1024)) // 2 MiB
	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()
	r, err := Run(ctx, fake, "ignored.mp4")
	if err != nil {
		t.Fatalf("unexpected err: %v", err)
	}
	if len(r.Stdout) != stdoutCap {
		t.Errorf("got stdout len %d, want %d", len(r.Stdout), stdoutCap)
	}
}

func TestRun_DeadlineKills(t *testing.T) {
	fake := buildFake(t)
	t.Setenv("FAKE_FFPROBE_SLEEP_MS", "10000") // 10s
	ctx, cancel := context.WithTimeout(context.Background(), 200*time.Millisecond)
	defer cancel()
	start := time.Now()
	_, err := Run(ctx, fake, "ignored.mp4")
	elapsed := time.Since(start)
	if err == nil {
		t.Fatal("expected err on deadline")
	}
	if elapsed > 2*time.Second {
		t.Errorf("did not kill quickly enough: %v", elapsed)
	}
}

func TestRun_StderrTailClippedTo4KiB(t *testing.T) {
	fake := buildFake(t)
	big := strings.Repeat("x", 8*1024)
	t.Setenv("FAKE_FFPROBE_STDERR", big)
	t.Setenv("FAKE_FFPROBE_EXIT", "1")
	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()
	r, _ := Run(ctx, fake, "ignored.mp4")
	if r == nil || len(r.StderrTail) > stderrTailCap {
		t.Errorf("got stderr tail len %d, cap %d", len(r.StderrTail), stderrTailCap)
	}
}
