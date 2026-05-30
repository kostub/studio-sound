//go:build integration

package ipc_test

import (
	"bufio"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"os"
	"os/exec"
	"path/filepath"
	"runtime"
	"testing"
	"time"
)

// responseEnvelope is a minimal struct used to decode IPC response lines in
// the integration test without importing internal packages.
type responseEnvelope struct {
	V      int              `json:"v"`
	ID     string           `json:"id"`
	Kind   string           `json:"kind"`
	Result *json.RawMessage `json:"result,omitempty"`
	Error  *struct {
		Code    string `json:"code"`
		Message string `json:"message"`
	} `json:"error,omitempty"`
}

// buildSidecar compiles the sidecar binary into a temporary directory and
// returns the path to the resulting executable.
func buildSidecar(t *testing.T) string {
	t.Helper()

	// Locate the repository root relative to this file's source directory.
	// runtime.Caller(0) returns the path of *this* source file at compile time.
	_, filename, _, ok := runtime.Caller(0)
	if !ok {
		t.Fatal("runtime.Caller(0) failed")
	}
	// Walk up: internal/ipc → internal → sidecar (module root)
	sidecarDir := filepath.Join(filepath.Dir(filename), "..", "..")

	tmpDir := t.TempDir()
	binName := "sidecar-e2e-test"
	if runtime.GOOS == "windows" {
		binName += ".exe"
	}
	binPath := filepath.Join(tmpDir, binName)

	ctx, cancel := context.WithTimeout(context.Background(), 60*time.Second)
	defer cancel()

	cmd := exec.CommandContext(ctx, "go", "build", "-o", binPath, "./cmd/sidecar")
	cmd.Dir = sidecarDir
	if out, err := cmd.CombinedOutput(); err != nil {
		t.Fatalf("failed to build sidecar: %v\n%s", err, out)
	}
	return binPath
}

// readResponse reads one newline-terminated JSON line from r within the given
// deadline and decodes it into a responseEnvelope. The lineCh is an optional
// pre-started goroutine channel; if nil a fresh one is created.
func readResponse(t *testing.T, r *bufio.Reader, deadline time.Duration) responseEnvelope {
	t.Helper()
	return readResponseFromCh(t, startLineReader(r), deadline)
}

type lineResult struct {
	line []byte
	err  error
}

// startLineReader launches a background goroutine that reads one line from r
// and sends the result on the returned buffered channel. The goroutine is not
// cancelable — the caller must drain or abandon the channel.
func startLineReader(r *bufio.Reader) chan lineResult {
	ch := make(chan lineResult, 1)
	go func() {
		line, err := r.ReadBytes('\n')
		ch <- lineResult{line: line, err: err}
	}()
	return ch
}

func readResponseFromCh(t *testing.T, ch chan lineResult, deadline time.Duration) responseEnvelope {
	t.Helper()
	select {
	case res := <-ch:
		if res.err != nil && res.err != io.EOF {
			t.Fatalf("error reading response: %v", res.err)
		}
		if len(res.line) == 0 {
			t.Fatal("got empty response line")
		}
		var env responseEnvelope
		if err := json.Unmarshal(res.line, &env); err != nil {
			t.Fatalf("failed to decode response %q: %v", string(res.line), err)
		}
		return env
	case <-time.After(deadline):
		t.Fatalf("timed out waiting for response after %v", deadline)
		return responseEnvelope{} // unreachable
	}
}

// writeRequest sends a JSON line terminated with '\n' to w.
func writeRequest(t *testing.T, w io.Writer, v any) {
	t.Helper()
	b, err := json.Marshal(v)
	if err != nil {
		t.Fatalf("failed to marshal request: %v", err)
	}
	if _, err := fmt.Fprintf(w, "%s\n", b); err != nil {
		t.Fatalf("failed to write request: %v", err)
	}
}

// TestMediaProbeRoundTrip exercises the media.probe IPC method end-to-end:
// it builds the sidecar binary, sends a media.probe envelope with a real
// test-asset path, and asserts a successful response with supported=true.
//
// Run with: go test -tags=integration ./internal/ipc/... -v -run TestMediaProbeRoundTrip
func TestMediaProbeRoundTrip(t *testing.T) {
	ffprobePath := os.Getenv("STUDIO_FFPROBE_PATH")
	if ffprobePath == "" {
		t.Skip("STUDIO_FFPROBE_PATH not set; skipping media.probe round-trip test")
	}

	binPath := buildSidecar(t)

	// Locate test-assets relative to this source file.
	_, thisFile, _, ok := runtime.Caller(0)
	if !ok {
		t.Fatal("runtime.Caller(0) failed")
	}
	// internal/ipc → internal → sidecar → repo root → test-assets
	assetDir := filepath.Join(filepath.Dir(thisFile), "..", "..", "..", "test-assets")
	mediaFile := filepath.Join(assetDir, "tiny-h264-aac-stereo.mp4")
	if _, err := os.Stat(mediaFile); err != nil {
		t.Skipf("test asset not found at %s: %v", mediaFile, err)
	}

	ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
	defer cancel()

	cmd := exec.CommandContext(ctx, binPath, "serve")
	cmd.Env = append(os.Environ(),
		"STUDIO_LOG_FILE=",
		"STUDIO_FFPROBE_PATH="+ffprobePath,
	)

	stdin, err := cmd.StdinPipe()
	if err != nil {
		t.Fatalf("failed to get stdin pipe: %v", err)
	}
	stdoutPipe, err := cmd.StdoutPipe()
	if err != nil {
		t.Fatalf("failed to get stdout pipe: %v", err)
	}

	if err := cmd.Start(); err != nil {
		t.Fatalf("failed to start sidecar: %v", err)
	}
	defer func() {
		_ = stdin.Close()
		_, _ = cmd.Process.Wait()
	}()

	reader := bufio.NewReader(stdoutPipe)

	// Send media.probe request.
	payload, err := json.Marshal(map[string]string{"path": mediaFile})
	if err != nil {
		t.Fatalf("failed to marshal probe payload: %v", err)
	}
	req := map[string]any{
		"v":       1,
		"id":      "test-probe-roundtrip",
		"kind":    "request",
		"method":  "media.probe",
		"payload": json.RawMessage(payload),
	}
	writeRequest(t, stdin, req)

	// Read the response — allow up to 15s for ffprobe to complete.
	resp := readResponse(t, reader, 15*time.Second)

	if resp.ID != "test-probe-roundtrip" {
		t.Errorf("got id=%q, want test-probe-roundtrip", resp.ID)
	}
	if resp.Kind != "response" {
		t.Errorf("got kind=%q, want response", resp.Kind)
	}
	if resp.Error != nil {
		t.Fatalf("unexpected error: code=%q message=%q", resp.Error.Code, resp.Error.Message)
	}
	if resp.Result == nil {
		t.Fatal("expected non-nil result in probe response")
	}

	// Decode the result and assert compatibility.supported=true.
	var result struct {
		Compatibility struct {
			Supported bool `json:"supported"`
		} `json:"compatibility"`
	}
	if err := json.Unmarshal(*resp.Result, &result); err != nil {
		t.Fatalf("failed to decode probe result: %v", err)
	}
	if !result.Compatibility.Supported {
		t.Errorf("expected compatibility.supported=true, got false; raw result: %s", string(*resp.Result))
	}

	// Shut down the sidecar cleanly.
	shutdownReq := map[string]any{
		"v":       1,
		"id":      "test-probe-shutdown",
		"kind":    "request",
		"method":  "system.shutdown",
		"payload": nil,
	}
	writeRequest(t, stdin, shutdownReq)
	_ = stdin.Close()
	readResponse(t, reader, 3*time.Second)
}

// TestE2EIntegration exercises the real sidecar binary end-to-end:
//   - system.ping round-trip
//   - system.echo round-trip
//   - system.shutdown causing a clean process exit
//
// Run with: go test -tags=integration ./internal/ipc/... -v -run TestE2EIntegration
func TestE2EIntegration(t *testing.T) {
	binPath := buildSidecar(t)

	ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
	defer cancel()

	cmd := exec.CommandContext(ctx, binPath, "serve")
	cmd.Env = append(os.Environ(), "STUDIO_LOG_FILE=") // suppress log file creation

	stdin, err := cmd.StdinPipe()
	if err != nil {
		t.Fatalf("failed to get stdin pipe: %v", err)
	}
	stdoutPipe, err := cmd.StdoutPipe()
	if err != nil {
		t.Fatalf("failed to get stdout pipe: %v", err)
	}

	if err := cmd.Start(); err != nil {
		t.Fatalf("failed to start sidecar: %v", err)
	}

	reader := bufio.NewReader(stdoutPipe)
	const responseTimeout = 3 * time.Second

	// ── Step 1: system.ping ─────────────────────────────────────────────────
	t.Run("ping", func(t *testing.T) {
		req := map[string]any{
			"v":       1,
			"id":      "test-e2e-ping",
			"kind":    "request",
			"method":  "system.ping",
			"payload": nil,
		}
		writeRequest(t, stdin, req)

		resp := readResponse(t, reader, responseTimeout)

		if resp.ID != "test-e2e-ping" {
			t.Errorf("got id=%q, want test-e2e-ping", resp.ID)
		}
		if resp.Kind != "response" {
			t.Errorf("got kind=%q, want response", resp.Kind)
		}
		if resp.Error != nil {
			t.Errorf("unexpected error: code=%q message=%q", resp.Error.Code, resp.Error.Message)
		}
		if resp.Result == nil {
			t.Error("expected result field to be present")
		}

		// Verify the result contains pong:true
		var result map[string]any
		if err := json.Unmarshal(*resp.Result, &result); err != nil {
			t.Fatalf("failed to decode ping result: %v", err)
		}
		if pong, ok := result["pong"]; !ok || pong != true {
			t.Errorf("expected pong:true in result, got %v", result)
		}
	})

	// ── Step 2: system.echo ─────────────────────────────────────────────────
	t.Run("echo", func(t *testing.T) {
		req := map[string]any{
			"v":      1,
			"id":     "test-e2e-echo",
			"kind":   "request",
			"method": "system.echo",
			"payload": map[string]string{
				"text": "hello",
			},
		}
		writeRequest(t, stdin, req)

		resp := readResponse(t, reader, responseTimeout)

		if resp.ID != "test-e2e-echo" {
			t.Errorf("got id=%q, want test-e2e-echo", resp.ID)
		}
		if resp.Kind != "response" {
			t.Errorf("got kind=%q, want response", resp.Kind)
		}
		if resp.Error != nil {
			t.Errorf("unexpected error: code=%q message=%q", resp.Error.Code, resp.Error.Message)
		}
		if resp.Result == nil {
			t.Error("expected result field to be present")
		}

		// Verify the result has text:"hello"
		var result map[string]string
		if err := json.Unmarshal(*resp.Result, &result); err != nil {
			t.Fatalf("failed to decode echo result: %v", err)
		}
		if result["text"] != "hello" {
			t.Errorf("expected result.text=hello, got %q", result["text"])
		}
	})

	// ── Step 3: system.shutdown — process must exit cleanly ─────────────────
	t.Run("shutdown", func(t *testing.T) {
		req := map[string]any{
			"v":       1,
			"id":      "test-e2e-shutdown",
			"kind":    "request",
			"method":  "system.shutdown",
			"payload": nil,
		}
		writeRequest(t, stdin, req)

		// Close stdin immediately so the sidecar's blocking readLine unblocks
		// after it sends the shutdown response (cancelling the context alone
		// is not enough because readLine is blocked on a syscall).
		stdin.Close()

		// Wait for the shutdown response.
		resp := readResponse(t, reader, responseTimeout)
		if resp.Error != nil {
			t.Errorf("shutdown returned error: code=%q message=%q", resp.Error.Code, resp.Error.Message)
		}
		if resp.Result == nil {
			t.Error("expected result field in shutdown response")
		} else {
			var result map[string]any
			if err := json.Unmarshal(*resp.Result, &result); err != nil {
				t.Fatalf("failed to decode shutdown result: %v", err)
			}
			if accepted, ok := result["accepted"]; !ok || accepted != true {
				t.Errorf("expected accepted:true in shutdown result, got %v", result)
			}
		}

		// Wait for the process to exit within 3 seconds.
		exitCh := make(chan error, 1)
		go func() { exitCh <- cmd.Wait() }()

		select {
		case err := <-exitCh:
			if err != nil {
				t.Errorf("sidecar exited with error: %v", err)
			}
		case <-time.After(3 * time.Second):
			// Force-kill to avoid leaking the process during test cleanup.
			_ = cmd.Process.Kill()
			t.Error("sidecar did not exit within 3 seconds after shutdown")
		}
	})
}
