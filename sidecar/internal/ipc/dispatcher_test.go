package ipc

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"strings"
	"testing"
)

// readFirstLine reads the first newline-terminated response from r.
func readFirstLine(t *testing.T, r io.Reader) []byte {
	t.Helper()
	var buf bytes.Buffer
	b := make([]byte, 1)
	for {
		n, err := r.Read(b)
		if n > 0 {
			buf.WriteByte(b[0])
			if b[0] == '\n' {
				return bytes.TrimRight(buf.Bytes(), "\n")
			}
		}
		if err != nil {
			t.Fatalf("read error: %v", err)
		}
	}
}

func runDispatch(t *testing.T, d *Dispatcher, input string) Envelope {
	t.Helper()
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	pr, pw := io.Pipe()
	var out bytes.Buffer

	done := make(chan error, 1)
	go func() {
		done <- d.Serve(ctx, pr, &out)
	}()

	// Write the request line then close the pipe to signal EOF.
	fmt.Fprintf(pw, "%s\n", input)
	pw.Close()

	<-done

	line := bytes.TrimRight(out.Bytes(), "\n")
	var env Envelope
	if err := json.Unmarshal(line, &env); err != nil {
		t.Fatalf("failed to parse response %q: %v", line, err)
	}
	return env
}

func TestDispatcher_HappyPath(t *testing.T) {
	d := NewDispatcher(nil)
	d.Register("test.ok", func(ctx context.Context, id string, payload json.RawMessage) (any, error) {
		return map[string]any{"ok": true}, nil
	})

	req := `{"v":1,"id":"abc123","kind":"request","method":"test.ok","payload":null}`
	env := runDispatch(t, d, req)

	if env.Error != nil {
		t.Fatalf("unexpected error: %+v", env.Error)
	}
	if env.Result == nil {
		t.Fatal("expected result field to be present")
	}
	if env.Kind != KindResponse {
		t.Errorf("got Kind=%q, want response", env.Kind)
	}

	var result map[string]any
	if err := json.Unmarshal(env.Result, &result); err != nil {
		t.Fatalf("failed to parse result: %v", err)
	}
	if result["ok"] != true {
		t.Errorf("expected ok=true, got %v", result["ok"])
	}
}

func TestDispatcher_UnknownMethod(t *testing.T) {
	d := NewDispatcher(nil)

	req := `{"v":1,"id":"abc123","kind":"request","method":"does.not.exist","payload":null}`
	env := runDispatch(t, d, req)

	if env.Error == nil {
		t.Fatal("expected error field to be present")
	}
	if env.Error.Code != CodeUnknownMethod {
		t.Errorf("got error.code=%q, want %q", env.Error.Code, CodeUnknownMethod)
	}
}

func TestDispatcher_MalformedJSON(t *testing.T) {
	d := NewDispatcher(nil)

	req := `this is not json`
	env := runDispatch(t, d, req)

	if env.Error == nil {
		t.Fatal("expected error field to be present")
	}
	if env.Error.Code != CodeMalformedEnvelope {
		t.Errorf("got error.code=%q, want %q", env.Error.Code, CodeMalformedEnvelope)
	}
}

func TestDispatcher_HandlerPanic(t *testing.T) {
	d := NewDispatcher(nil)
	d.Register("panic.method", func(ctx context.Context, id string, payload json.RawMessage) (any, error) {
		panic("intentional panic for test")
	})

	req := `{"v":1,"id":"panic-id","kind":"request","method":"panic.method","payload":null}`
	env := runDispatch(t, d, req)

	if env.Error == nil {
		t.Fatal("expected error field to be present")
	}
	if env.Error.Code != CodeInternalError {
		t.Errorf("got error.code=%q, want %q", env.Error.Code, CodeInternalError)
	}
}

func TestDispatcher_OversizedLine(t *testing.T) {
	d := NewDispatcher(nil)

	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	// Build a line > 8 MiB
	big := strings.Repeat("x", 9*1024*1024)

	pr, pw := io.Pipe()
	var out bytes.Buffer

	done := make(chan error, 1)
	go func() {
		done <- d.Serve(ctx, pr, &out)
	}()

	// Write in a goroutine to avoid deadlocking the test — writing 9 MiB may
	// block until the dispatcher has drained enough of the pipe.
	go func() {
		fmt.Fprintf(pw, "%s\n", big)
		pw.Close()
	}()

	<-done

	line := bytes.TrimRight(out.Bytes(), "\n")
	var env Envelope
	if err := json.Unmarshal(line, &env); err != nil {
		t.Fatalf("failed to parse response %q: %v", string(line[:min(len(line), 200)]), err)
	}

	if env.Error == nil {
		t.Fatal("expected error field to be present")
	}
	if env.Error.Code != CodeMessageTooLarge {
		t.Errorf("got error.code=%q, want %q", env.Error.Code, CodeMessageTooLarge)
	}
}

func min(a, b int) int {
	if a < b {
		return a
	}
	return b
}
