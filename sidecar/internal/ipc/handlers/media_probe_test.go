package handlers

import (
	"context"
	"encoding/json"
	"errors"
	"os"
	"os/exec"
	"path/filepath"
	"reflect"
	"runtime"
	"testing"

	"github.com/studio-sound/studio/sidecar/internal/ipc"
)

func TestProbePayloadSchema_MatchesCanonicalSchema(t *testing.T) {
	_, thisFile, _, ok := runtime.Caller(0)
	if !ok {
		t.Fatal("runtime.Caller(0) failed")
	}
	// internal/ipc/handlers/media_probe_test.go -> repo root -> schemas/...
	repoRoot := filepath.Join(filepath.Dir(thisFile), "..", "..", "..", "..")
	schemaPath := filepath.Join(repoRoot, "schemas", "media.probe.schema.json")
	raw, err := os.ReadFile(schemaPath)
	if err != nil {
		t.Skipf("canonical schema not readable at %s: %v", schemaPath, err)
	}
	var schema map[string]any
	if err := json.Unmarshal(raw, &schema); err != nil {
		t.Fatalf("parse %s: %v", schemaPath, err)
	}
	defs, _ := schema["$defs"].(map[string]any)
	canonical, ok := defs["ProbePayload"].(map[string]any)
	if !ok {
		t.Fatalf("ProbePayload definition missing in %s", schemaPath)
	}

	var inline map[string]any
	if err := json.Unmarshal([]byte(probePayloadSchema), &inline); err != nil {
		t.Fatalf("parse inline probe schema: %v", err)
	}
	delete(inline, "$schema")

	if !reflect.DeepEqual(inline, canonical) {
		t.Fatalf("inline probe payload schema differs from %s $defs.ProbePayload", schemaPath)
	}
}

// buildFakeForHandlerTest builds the fakeffprobe helper into a temp dir and
// returns its path. Mirrors buildFake from internal/media/runner_test.go.
func buildFakeForHandlerTest(t *testing.T) string {
	t.Helper()
	tmp := t.TempDir()
	out := filepath.Join(tmp, "fakeffprobe")
	if runtime.GOOS == "windows" {
		out += ".exe"
	}
	_, thisFile, _, ok := runtime.Caller(0)
	if !ok {
		t.Fatal("runtime.Caller(0) failed")
	}
	// handlers/ → ipc/ → internal/ → sidecar/ → cmd/fakeffprobe
	fakePath := filepath.Join(filepath.Dir(thisFile), "..", "..", "..", "cmd", "fakeffprobe")
	cmd := exec.Command("go", "build", "-o", out, fakePath)
	cmd.Stderr = os.Stderr
	if err := cmd.Run(); err != nil {
		t.Fatalf("failed to build fakeffprobe: %v", err)
	}
	return out
}

func TestProbeHandler_RejectsMissingPath(t *testing.T) {
	_, err := ProbeHandler(context.Background(), "id-1", json.RawMessage(`{}`))
	if err == nil {
		t.Fatal("expected err for missing path")
	}
	var rpcErr *ipc.RPCError
	if !errors.As(err, &rpcErr) {
		t.Fatalf("expected *ipc.RPCError, got %T: %v", err, err)
	}
	if rpcErr.Code != ipc.CodeInvalidPayload {
		t.Errorf("expected code %q, got %q", ipc.CodeInvalidPayload, rpcErr.Code)
	}
}

func TestProbeHandler_RejectsEmptyPath(t *testing.T) {
	_, err := ProbeHandler(context.Background(), "id-1", json.RawMessage(`{"path":""}`))
	if err == nil {
		t.Fatal("expected err for empty path")
	}
	var rpcErr *ipc.RPCError
	if !errors.As(err, &rpcErr) {
		t.Fatalf("expected *ipc.RPCError, got %T: %v", err, err)
	}
	if rpcErr.Code != ipc.CodeInvalidPayload {
		t.Errorf("expected code %q, got %q", ipc.CodeInvalidPayload, rpcErr.Code)
	}
}

func TestProbeHandler_HappyPathReturnsResult(t *testing.T) {
	fake := buildFakeForHandlerTest(t)
	t.Setenv("STUDIO_FFPROBE_PATH", fake)
	t.Setenv("FAKE_FFPROBE_STDOUT", `{"format":{"format_name":"mov,mp4,m4a,3gp,3g2,mj2","format_long_name":"QuickTime / MOV","duration":"5.0","size":"1024"},"streams":[{"index":0,"codec_type":"video","codec_name":"h264","width":640,"height":480,"r_frame_rate":"30/1"},{"index":1,"codec_type":"audio","codec_name":"aac","channels":2,"sample_rate":"48000","channel_layout":"stereo","disposition":{"default":1}}]}`)
	tmp := t.TempDir()
	media := filepath.Join(tmp, "x.mp4")
	if err := os.WriteFile(media, []byte("x"), 0o644); err != nil {
		t.Fatal(err)
	}
	payload, _ := json.Marshal(map[string]string{"path": media})
	r, err := ProbeHandler(context.Background(), "id-1", payload)
	if err != nil {
		t.Fatalf("unexpected err: %v", err)
	}
	if r == nil {
		t.Fatal("nil result")
	}
}

func TestProbeHandler_FileNotFoundReturnsRPCError(t *testing.T) {
	fake := buildFakeForHandlerTest(t)
	t.Setenv("STUDIO_FFPROBE_PATH", fake)
	payload, _ := json.Marshal(map[string]string{"path": "/does/not/exist.mp4"})
	_, err := ProbeHandler(context.Background(), "id-1", payload)
	if err == nil {
		t.Fatal("expected err for missing file")
	}
	var rpcErr *ipc.RPCError
	if !errors.As(err, &rpcErr) {
		t.Fatalf("expected *ipc.RPCError, got %T: %v", err, err)
	}
	if rpcErr.Code != ipc.CodeFileNotFound {
		t.Errorf("expected code %q, got %q", ipc.CodeFileNotFound, rpcErr.Code)
	}
}
