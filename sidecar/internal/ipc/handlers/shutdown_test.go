package handlers

import (
	"context"
	"encoding/json"
	"errors"
	"testing"
	"time"

	"github.com/studio-sound/studio/sidecar/internal/ipc"
)

func TestShutdownHandler_HappyPath(t *testing.T) {
	called := false
	cancelFn := func() { called = true }

	h := ShutdownHandler(cancelFn)
	result, err := h(context.Background(), "test-id", json.RawMessage(`null`))
	if err != nil {
		t.Fatalf("expected nil error, got: %v", err)
	}
	if result == nil {
		t.Fatal("expected non-nil result")
	}

	// Check the result shape: {"accepted": true}
	b, _ := json.Marshal(result)
	var m map[string]any
	if err := json.Unmarshal(b, &m); err != nil {
		t.Fatalf("failed to unmarshal result: %v", err)
	}
	if m["accepted"] != true {
		t.Errorf("expected accepted=true, got %v", m["accepted"])
	}

	// cancelFn should have been called (goroutine may race, give it a moment).
	// We use a channel-based approach to be race-safe.
	cancelCh := make(chan struct{}, 1)
	cancelFn2 := func() { cancelCh <- struct{}{} }
	h2 := ShutdownHandler(cancelFn2)
	_, err = h2(context.Background(), "test-id-2", json.RawMessage(`null`))
	if err != nil {
		t.Fatalf("unexpected error on second call: %v", err)
	}
	select {
	case <-cancelCh:
		// cancelFn was called — expected
	case <-time.After(2 * time.Second):
		t.Fatal("timeout waiting for cancelFn to be called")
	}
	_ = called // suppress unused variable warning
}

func TestShutdownHandler_AbsentPayload(t *testing.T) {
	cancelFn := func() {}
	h := ShutdownHandler(cancelFn)

	result, err := h(context.Background(), "test-id", nil)
	if err != nil {
		t.Fatalf("expected nil error for absent payload, got: %v", err)
	}
	if result == nil {
		t.Fatal("expected non-nil result")
	}
}

func TestShutdownHandler_InvalidPayload(t *testing.T) {
	cancelFn := func() {}
	h := ShutdownHandler(cancelFn)

	// Non-null payload should be rejected with INVALID_PAYLOAD.
	payload := json.RawMessage(`{"unexpected": "field"}`)
	_, err := h(context.Background(), "test-id", payload)
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
