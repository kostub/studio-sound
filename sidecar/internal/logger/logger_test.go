package logger

import (
	"bytes"
	"encoding/json"
	"strings"
	"testing"
)

func TestNewEmptyPathReturnsNonNilLogger(t *testing.T) {
	l := New("")
	if l == nil {
		t.Fatal("expected non-nil logger from New(\"\"), got nil")
	}
	// Smoke test: logging at Info level should not panic.
	l.Info("smoke test message")
}

func TestNewWithWriterWritesValidJSONLines(t *testing.T) {
	var buf bytes.Buffer
	l := NewWithWriter(&buf)
	if l == nil {
		t.Fatal("expected non-nil logger, got nil")
	}

	const wantMsg = "hello from test"
	l.Info(wantMsg, "key", "value")

	output := strings.TrimSpace(buf.String())
	if output == "" {
		t.Fatal("expected non-empty output, got empty string")
	}

	// The output should be valid JSON.
	var record map[string]any
	if err := json.Unmarshal([]byte(output), &record); err != nil {
		t.Fatalf("output is not valid JSON: %v\noutput: %s", err, output)
	}

	// Verify the message field.
	msg, ok := record["msg"].(string)
	if !ok {
		t.Fatalf("expected \"msg\" field in JSON output, got %v", record)
	}
	if msg != wantMsg {
		t.Fatalf("expected msg=%q, got msg=%q", wantMsg, msg)
	}

	// Verify the extra key is present.
	if _, ok := record["key"]; !ok {
		t.Fatal("expected \"key\" field in JSON output")
	}
}
