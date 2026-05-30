//go:build integration

package media

import (
	"context"
	"errors"
	"os"
	"path/filepath"
	"testing"
	"time"

	"github.com/studio-sound/studio/sidecar/internal/ipc"
)

func ffprobePathForTest(t *testing.T) string {
	t.Helper()
	p := os.Getenv("STUDIO_FFPROBE_PATH")
	if p == "" {
		t.Skip("STUDIO_FFPROBE_PATH not set; skipping integration test")
	}
	if _, err := os.Stat(p); err != nil {
		t.Skipf("ffprobe not at %s: %v", p, err)
	}
	return p
}

func assetPath(t *testing.T, name string) string {
	t.Helper()
	// tests live at sidecar/internal/media; assets at <repo>/test-assets
	return filepath.Join("..", "..", "..", "test-assets", name)
}

func TestIntegration_H264AACStereo(t *testing.T) {
	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()
	r, err := Probe(ctx, ffprobePathForTest(t), assetPath(t, "tiny-h264-aac-stereo.mp4"))
	if err != nil {
		t.Fatalf("unexpected err: %v", err)
	}
	if !r.Compatibility.Supported {
		t.Errorf("expected supported=true, got %+v", r.Compatibility)
	}
	if r.Audio == nil || r.Audio.Codec != "aac" {
		t.Errorf("audio = %+v", r.Audio)
	}
}

func TestIntegration_NoAudio(t *testing.T) {
	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()
	r, err := Probe(ctx, ffprobePathForTest(t), assetPath(t, "tiny-no-audio.mp4"))
	if err != nil {
		t.Fatalf("unexpected err: %v", err)
	}
	if r.Compatibility.Supported {
		t.Error("expected supported=false for no-audio file")
	}
	if r.Audio != nil {
		t.Errorf("audio should be nil for no-audio file, got %+v", r.Audio)
	}
}

func TestIntegration_Multitrack(t *testing.T) {
	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()
	r, err := Probe(ctx, ffprobePathForTest(t), assetPath(t, "tiny-h264-aac-multitrack.mov"))
	if err != nil {
		t.Fatal(err)
	}
	if r.Audio == nil || r.Audio.TrackCount != 2 {
		t.Fatalf("expected 2 audio tracks, got audio = %+v", r.Audio)
	}
	// The default-flagged track must be selected.
	var defaultIdx int
	for _, tr := range r.Audio.Tracks {
		if tr.IsDefault {
			defaultIdx = tr.Index
		}
	}
	if r.Audio.TrackIndex != defaultIdx {
		t.Errorf("default track mismatch: trackIndex=%d, default-flagged stream index=%d",
			r.Audio.TrackIndex, defaultIdx)
	}
}

func TestIntegration_VP9Opus(t *testing.T) {
	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()
	r, err := Probe(ctx, ffprobePathForTest(t), assetPath(t, "tiny-vp9-opus.webm"))
	if err != nil {
		t.Fatal(err)
	}
	if !r.Compatibility.Supported {
		t.Errorf("expected supported=true for vp9/opus, got %+v", r.Compatibility)
	}
}

func TestIntegration_Corrupt(t *testing.T) {
	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()
	_, err := Probe(ctx, ffprobePathForTest(t), assetPath(t, "corrupt-truncated.mp4"))
	if err == nil {
		t.Fatal("expected error for corrupt file, got nil")
	}
	var rpcErr *ipc.RPCError
	if !errors.As(err, &rpcErr) {
		t.Fatalf("expected *ipc.RPCError, got %T: %v", err, err)
	}
	if rpcErr.Code != ipc.CodeCorruptMedia {
		t.Errorf("code = %s, want %s", rpcErr.Code, ipc.CodeCorruptMedia)
	}
}

func TestIntegration_UnicodePath(t *testing.T) {
	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()
	r, err := Probe(ctx, ffprobePathForTest(t), assetPath(t, "unicode-name-🎥-интервью.mp4"))
	if err != nil {
		t.Fatalf("unexpected err for unicode path: %v", err)
	}
	if !r.Compatibility.Supported {
		t.Errorf("expected supported=true for unicode path file, got %+v", r.Compatibility)
	}
}

func TestIntegration_FileNotFound(t *testing.T) {
	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()
	_, err := Probe(ctx, ffprobePathForTest(t), assetPath(t, "definitely-does-not-exist.mp4"))
	if err == nil {
		t.Fatal("expected error for missing file, got nil")
	}
}

// TestIntegration_KillMidProbeLeavesNoOrphan cancels the context ~50ms after
// starting a probe so that ffprobe is killed mid-run, then immediately re-probes
// the same file and asserts the second probe succeeds — confirming no file-
// descriptor leak or lingering subprocess blocks subsequent access.
func TestIntegration_KillMidProbeLeavesNoOrphan(t *testing.T) {
	fp := ffprobePathForTest(t)
	asset := assetPath(t, "tiny-h264-aac-stereo.mp4")

	ctx, cancel := context.WithCancel(context.Background())
	// Cancel quickly so ffprobe is killed mid-run.
	go func() {
		time.Sleep(50 * time.Millisecond)
		cancel()
	}()
	_, _ = Probe(ctx, fp, asset)

	// Re-probe immediately; no lingering FD or orphan process should block this.
	ctx2, cancel2 := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel2()
	_, err := Probe(ctx2, fp, asset)
	if err != nil {
		t.Fatalf("second probe failed after kill: %v", err)
	}
}
