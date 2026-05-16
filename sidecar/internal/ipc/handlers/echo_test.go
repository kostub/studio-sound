package handlers

import (
	"context"
	"encoding/json"
	"errors"
	"strings"
	"testing"

	"github.com/studio-sound/studio/sidecar/internal/ipc"
)

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
	// Build a string > 1024 bytes.
	longText := strings.Repeat("a", 1025)
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
	// Exactly 1024 bytes should be allowed.
	text := strings.Repeat("a", 1024)
	payload, _ := json.Marshal(map[string]string{"text": text})

	result, err := EchoHandler(context.Background(), "test-id", json.RawMessage(payload))
	if err != nil {
		t.Fatalf("expected nil error for exactly-1024-byte text, got: %v", err)
	}
	if result == nil {
		t.Fatal("expected non-nil result")
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
