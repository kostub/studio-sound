package handlers

import (
	"context"
	"encoding/json"
	"errors"
	"testing"

	"github.com/studio-sound/studio/sidecar/internal/ipc"
)

func TestPingHandler_HappyPath(t *testing.T) {
	// Valid payload: null (ping takes no payload)
	payload := json.RawMessage(`null`)
	result, err := PingHandler(context.Background(), "test-id", payload)
	if err != nil {
		t.Fatalf("expected nil error, got: %v", err)
	}
	if result == nil {
		t.Fatal("expected non-nil result")
	}

	// Check the result shape.
	b, _ := json.Marshal(result)
	var m map[string]any
	if err := json.Unmarshal(b, &m); err != nil {
		t.Fatalf("failed to unmarshal result: %v", err)
	}
	if m["pong"] != true {
		t.Errorf("expected pong=true, got %v", m["pong"])
	}
	if _, ok := m["sidecarVersion"]; !ok {
		t.Error("expected sidecarVersion field in result")
	}
	if _, ok := m["uptimeMs"]; !ok {
		t.Error("expected uptimeMs field in result")
	}
	if _, ok := m["supportedProtocolVersions"]; !ok {
		t.Error("expected supportedProtocolVersions field in result")
	}
}

func TestPingHandler_AbsentPayload(t *testing.T) {
	// Empty/absent payload should also be treated as null.
	result, err := PingHandler(context.Background(), "test-id", nil)
	if err != nil {
		t.Fatalf("expected nil error for absent payload, got: %v", err)
	}
	if result == nil {
		t.Fatal("expected non-nil result")
	}
}

func TestPingHandler_InvalidPayload(t *testing.T) {
	// Non-null payload should be rejected with INVALID_PAYLOAD.
	payload := json.RawMessage(`{"unexpected": "field"}`)
	_, err := PingHandler(context.Background(), "test-id", payload)
	if err == nil {
		t.Fatal("expected error for non-null payload, got nil")
	}

	var rpcErr *ipc.RPCError
	if !errors.As(err, &rpcErr) {
		t.Fatalf("expected *ipc.RPCError, got %T: %v", err, err)
	}
	if rpcErr.Code != ipc.CodeInvalidPayload {
		t.Errorf("expected code %q, got %q", ipc.CodeInvalidPayload, rpcErr.Code)
	}
}
