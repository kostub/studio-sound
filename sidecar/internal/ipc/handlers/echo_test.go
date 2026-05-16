package handlers

import (
	"context"
	"encoding/json"
	"errors"
	"os"
	"path/filepath"
	"runtime"
	"strings"
	"testing"

	"github.com/studio-sound/studio/sidecar/internal/ipc"
)

// TestEchoMaxChars_MatchesCanonicalSchema guards against drift between the
// Go-side echoMaxChars constant and the canonical JSON Schema's maxLength.
// If this fails, update either the constant or schemas/system.echo.schema.json
// so they agree, and rerun `npm run gen:schemas`.
func TestEchoMaxChars_MatchesCanonicalSchema(t *testing.T) {
	_, thisFile, _, ok := runtime.Caller(0)
	if !ok {
		t.Fatal("runtime.Caller(0) failed")
	}
	// internal/ipc/handlers/echo_test.go → repo root → schemas/...
	repoRoot := filepath.Join(filepath.Dir(thisFile), "..", "..", "..", "..")
	schemaPath := filepath.Join(repoRoot, "schemas", "system.echo.schema.json")
	raw, err := os.ReadFile(schemaPath)
	if err != nil {
		t.Skipf("canonical schema not readable at %s: %v", schemaPath, err)
	}
	var schema map[string]any
	if err := json.Unmarshal(raw, &schema); err != nil {
		t.Fatalf("parse %s: %v", schemaPath, err)
	}
	defs, _ := schema["$defs"].(map[string]any)
	payload, _ := defs["EchoPayload"].(map[string]any)
	props, _ := payload["properties"].(map[string]any)
	text, _ := props["text"].(map[string]any)
	got, ok := text["maxLength"].(float64)
	if !ok {
		t.Fatalf("EchoPayload.text.maxLength missing or not a number in %s", schemaPath)
	}
	if int(got) != echoMaxChars {
		t.Errorf("canonical schema maxLength=%d but echoMaxChars=%d (must match)", int(got), echoMaxChars)
	}
}

func TestEchoHandler_HappyPath(t *testing.T) {
	payload := json.RawMessage(`{"text":"hello world"}`)
	result, err := EchoHandler(context.Background(), "test-id", payload)
	if err != nil {
		t.Fatalf("expected nil error, got: %v", err)
	}
	if result == nil {
		t.Fatal("expected non-nil result")
	}

	b, _ := json.Marshal(result)
	var m map[string]string
	if err := json.Unmarshal(b, &m); err != nil {
		t.Fatalf("failed to unmarshal result: %v", err)
	}
	if m["text"] != "hello world" {
		t.Errorf("expected text=%q, got %q", "hello world", m["text"])
	}
}

func TestEchoHandler_EmptyText(t *testing.T) {
	payload := json.RawMessage(`{"text":""}`)
	result, err := EchoHandler(context.Background(), "test-id", payload)
	if err != nil {
		t.Fatalf("expected nil error for empty text, got: %v", err)
	}

	b, _ := json.Marshal(result)
	var m map[string]string
	if err := json.Unmarshal(b, &m); err != nil {
		t.Fatalf("failed to unmarshal result: %v", err)
	}
	if m["text"] != "" {
		t.Errorf("expected empty text, got %q", m["text"])
	}
}

func TestEchoHandler_TooLongText(t *testing.T) {
	// Build a string > 4096 characters.
	longText := strings.Repeat("a", 4097)
	payload, _ := json.Marshal(map[string]string{"text": longText})

	_, err := EchoHandler(context.Background(), "test-id", json.RawMessage(payload))
	if err == nil {
		t.Fatal("expected error for too-long text, got nil")
	}

	var rpcErr *ipc.RPCError
	if !errors.As(err, &rpcErr) {
		t.Fatalf("expected *ipc.RPCError, got %T: %v", err, err)
	}
	if rpcErr.Code != ipc.CodeEchoTooLong {
		t.Errorf("expected code %q, got %q", ipc.CodeEchoTooLong, rpcErr.Code)
	}
}

func TestEchoHandler_ExactlyAtLimit(t *testing.T) {
	// Exactly 4096 characters should be allowed.
	text := strings.Repeat("a", 4096)
	payload, _ := json.Marshal(map[string]string{"text": text})

	result, err := EchoHandler(context.Background(), "test-id", json.RawMessage(payload))
	if err != nil {
		t.Fatalf("expected nil error for exactly-4096-character text, got: %v", err)
	}
	if result == nil {
		t.Fatal("expected non-nil result")
	}
}

func TestEchoHandler_MultiByteCharacters(t *testing.T) {
	// Emojis are multi-byte but count as 1 character (rune) each.
	// 4096 🚀 emojis.
	text := strings.Repeat("🚀", 4096)
	payload, _ := json.Marshal(map[string]string{"text": text})

	result, err := EchoHandler(context.Background(), "test-id", json.RawMessage(payload))
	if err != nil {
		t.Fatalf("expected nil error for 4096 emojis, got: %v", err)
	}
	if result == nil {
		t.Fatal("expected non-nil result")
	}

	// 4097 🚀 emojis should fail.
	textTooLong := strings.Repeat("🚀", 4097)
	payloadTooLong, _ := json.Marshal(map[string]string{"text": textTooLong})
	_, err = EchoHandler(context.Background(), "test-id", json.RawMessage(payloadTooLong))
	if err == nil {
		t.Fatal("expected error for 4097 emojis, got nil")
	}
}

func TestEchoHandler_InvalidPayload_MissingText(t *testing.T) {
	// Missing required "text" field.
	payload := json.RawMessage(`{}`)
	_, err := EchoHandler(context.Background(), "test-id", payload)
	if err == nil {
		t.Fatal("expected error for missing text field, got nil")
	}

	var rpcErr *ipc.RPCError
	if !errors.As(err, &rpcErr) {
		t.Fatalf("expected *ipc.RPCError, got %T: %v", err, err)
	}
	if rpcErr.Code != ipc.CodeInvalidPayload {
		t.Errorf("expected code %q, got %q", ipc.CodeInvalidPayload, rpcErr.Code)
	}
}

func TestEchoHandler_InvalidPayload_WrongType(t *testing.T) {
	// text must be a string, not a number.
	payload := json.RawMessage(`{"text": 42}`)
	_, err := EchoHandler(context.Background(), "test-id", payload)
	if err == nil {
		t.Fatal("expected error for wrong type, got nil")
	}

	var rpcErr *ipc.RPCError
	if !errors.As(err, &rpcErr) {
		t.Fatalf("expected *ipc.RPCError, got %T: %v", err, err)
	}
	if rpcErr.Code != ipc.CodeInvalidPayload {
		t.Errorf("expected code %q, got %q", ipc.CodeInvalidPayload, rpcErr.Code)
	}
}

func TestEchoHandler_InvalidPayload_NonObject(t *testing.T) {
	// Payload must be an object.
	payload := json.RawMessage(`"not an object"`)
	_, err := EchoHandler(context.Background(), "test-id", payload)
	if err == nil {
		t.Fatal("expected error for non-object payload, got nil")
	}

	var rpcErr *ipc.RPCError
	if !errors.As(err, &rpcErr) {
		t.Fatalf("expected *ipc.RPCError, got %T: %v", err, err)
	}
	if rpcErr.Code != ipc.CodeInvalidPayload {
		t.Errorf("expected code %q, got %q", ipc.CodeInvalidPayload, rpcErr.Code)
	}
}
