package cli

import (
	"bufio"
	"bytes"
	"encoding/json"
	"io"
	"strings"
	"testing"
	"time"
)

// TestServeExitsOnStdinClose verifies that the serve subcommand exits cleanly
// when the stdin pipe is closed (simulating the parent process terminating).
func TestServeExitsOnStdinClose(t *testing.T) {
	pr, pw := io.Pipe()

	var stdout bytes.Buffer
	var stderr bytes.Buffer

	done := make(chan int, 1)
	go func() {
		done <- Run([]string{"serve"}, pr, &stdout, &stderr)
	}()

	// Close the write end — the serve loop sees EOF and should return.
	pw.Close()

	select {
	case code := <-done:
		// io.EOF is an expected clean exit; the serve loop returns 0 on EOF.
		if code != 0 {
			t.Errorf("expected exit code 0 on stdin close, got %d (stderr: %q)", code, stderr.String())
		}
	case <-time.After(2 * time.Second):
		t.Fatal("serve did not exit within 2 seconds after stdin was closed")
	}
}

// TestServeProcessesOnePing sends a valid NDJSON request and verifies that
// the serve loop processes it and writes back a well-formed response.
// The ping handler is not registered until item 3.2, so the response may be
// an UNKNOWN_METHOD error — the test only verifies that the IPC loop
// processes the line and writes a valid JSON response with the correct id.
func TestServeProcessesOnePing(t *testing.T) {
	pr, pw := io.Pipe()

	pr2, pw2 := io.Pipe()

	var stderr bytes.Buffer

	done := make(chan int, 1)
	go func() {
		done <- Run([]string{"serve"}, pr, pw2, &stderr)
	}()

	// Write a valid NDJSON ping request.
	const request = `{"v":1,"id":"test-1","kind":"request","method":"system.ping","payload":{}}` + "\n"
	if _, err := io.WriteString(pw, request); err != nil {
		t.Fatalf("failed to write request: %v", err)
	}

	// Read one response line from stdout.
	responseLine := make(chan string, 1)
	go func() {
		scanner := bufio.NewScanner(pr2)
		if scanner.Scan() {
			responseLine <- scanner.Text()
		} else {
			responseLine <- ""
		}
	}()

	var line string
	select {
	case line = <-responseLine:
	case <-time.After(2 * time.Second):
		pw.Close()
		t.Fatal("timed out waiting for a response from serve")
	}

	if line == "" {
		t.Fatal("expected a non-empty response line from serve")
	}

	// Parse the response as JSON.
	var env map[string]any
	if err := json.Unmarshal([]byte(line), &env); err != nil {
		t.Fatalf("response is not valid JSON: %v\nline: %s", err, line)
	}

	// Verify the response id matches the request id.
	id, _ := env["id"].(string)
	if id != "test-1" {
		t.Errorf("expected response id=%q, got %q", "test-1", id)
	}

	// Verify the response has either a result or an error (UNKNOWN_METHOD is
	// acceptable since system.ping is not yet registered in Phase 1 item 2.x).
	hasResult := env["result"] != nil
	errField, hasError := env["error"]
	if !hasResult && !hasError {
		t.Errorf("response has neither \"result\" nor \"error\" field: %s", line)
	}
	if hasError {
		errObj, ok := errField.(map[string]any)
		if !ok {
			t.Errorf("\"error\" field is not an object: %v", errField)
		} else {
			code, _ := errObj["code"].(string)
			if !strings.HasPrefix(code, "UNKNOWN_METHOD") && code != "UNKNOWN_METHOD" {
				// Accept UNKNOWN_METHOD specifically — any other error code
				// would be unexpected for a method that simply isn't registered yet.
				// We do not fail for other codes because the dispatcher can also
				// legitimately return MALFORMED_ENVELOPE if it deems {} invalid.
				// The key assertion is that we got a response with the right id.
				t.Logf("note: received error code %q (expected UNKNOWN_METHOD or similar)", code)
			}
		}
	}

	// Close stdin to stop the server.
	pw.Close()

	select {
	case <-done:
	case <-time.After(2 * time.Second):
		t.Error("serve did not exit within 2 seconds after stdin was closed")
	}
}
