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

func TestProbe_HappyPathWithFakeFFprobe(t *testing.T) {
	fake := buildFake(t)
	// Make ffprobe emit a tiny valid output.
	t.Setenv("FAKE_FFPROBE_STDOUT", `{"format":{"format_name":"mov,mp4,m4a,3gp,3g2,mj2","format_long_name":"QuickTime / MOV","duration":"5.0","size":"1024"},"streams":[{"index":0,"codec_type":"video","codec_name":"h264","width":640,"height":480,"r_frame_rate":"30/1"},{"index":1,"codec_type":"audio","codec_name":"aac","channels":2,"sample_rate":"48000","channel_layout":"stereo","disposition":{"default":1}}]}`)
	tmp := t.TempDir()
	mediaPath := filepath.Join(tmp, "x.mp4")
	if err := os.WriteFile(mediaPath, []byte("x"), 0o644); err != nil {
		t.Fatal(err)
	}
	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()
	r, err := Probe(ctx, fake, mediaPath)
	if err != nil {
		t.Fatalf("unexpected err: %v", err)
	}
	if r == nil {
		t.Fatal("nil result")
	}
	if !r.Compatibility.Supported {
		t.Errorf("expected supported, got %+v", r.Compatibility)
	}
	if r.Audio == nil {
		t.Error("audio should be non-nil")
	}
}

func TestProbe_UnsupportedReturnsSuccessWithSupportedFalse(t *testing.T) {
	fake := buildFake(t)
	// unsupported container (asf/wmv)
	t.Setenv("FAKE_FFPROBE_STDOUT", `{"format":{"format_name":"asf","format_long_name":"ASF"},"streams":[{"index":0,"codec_type":"video","codec_name":"wmv2","width":640,"height":480,"r_frame_rate":"30/1"},{"index":1,"codec_type":"audio","codec_name":"wmav2","channels":2,"sample_rate":"44100","disposition":{"default":1}}]}`)
	tmp := t.TempDir()
	mediaPath := filepath.Join(tmp, "x.wmv")
	_ = os.WriteFile(mediaPath, []byte("x"), 0o644)
	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()
	r, err := Probe(ctx, fake, mediaPath)
	if err != nil {
		t.Fatalf("expected success, got err: %v", err)
	}
	if r.Compatibility.Supported {
		t.Errorf("expected supported=false, got %+v", r.Compatibility)
	}
	if len(r.Compatibility.Issues) == 0 {
		t.Error("expected non-empty issues")
	}
}

func TestProbe_FileNotFoundBeforeSpawn(t *testing.T) {
	fake := buildFake(t)
	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()
	_, err := Probe(ctx, fake, "/definitely/not/a/path.mp4")
	if err == nil {
		t.Fatal("expected err")
	}
}

func TestProbe_CorruptStderrMapsToCorruptMedia(t *testing.T) {
	fake := buildFake(t)
	t.Setenv("FAKE_FFPROBE_EXIT", "1")
	t.Setenv("FAKE_FFPROBE_STDERR", "Invalid data found when processing input")
	tmp := t.TempDir()
	mediaPath := filepath.Join(tmp, "x.mp4")
	_ = os.WriteFile(mediaPath, []byte("x"), 0o644)
	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()
	_, err := Probe(ctx, fake, mediaPath)
	if err == nil {
		t.Fatal("expected err")
	}
	var rpcErr *ipc.RPCError
	if !errors.As(err, &rpcErr) {
		t.Fatalf("Probe error not assertable to *ipc.RPCError: %T %v", err, err)
	}
	if rpcErr.Code != ipc.CodeCorruptMedia {
		t.Errorf("got code %q, want %q", rpcErr.Code, ipc.CodeCorruptMedia)
	}
}
