package ipc

import (
	"encoding/json"
	"errors"
	"testing"
)

func TestDecodeLine_RoundTrip(t *testing.T) {
	line := []byte(`{"v":1,"id":"01H8X1Y2Z3A4B5C6D7E8F9G0H1","kind":"request","method":"system.ping","payload":null}`)
	env, err := DecodeLine(line)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if env.V != 1 {
		t.Errorf("got V=%d, want 1", env.V)
	}
	if env.ID != "01H8X1Y2Z3A4B5C6D7E8F9G0H1" {
		t.Errorf("got ID=%q", env.ID)
	}
	if env.Kind != KindRequest {
		t.Errorf("got Kind=%q, want request", env.Kind)
	}
	if env.Method != "system.ping" {
		t.Errorf("got Method=%q, want system.ping", env.Method)
	}
}

func TestDecodeLine_MalformedJSON(t *testing.T) {
	line := []byte(`not json at all`)
	_, err := DecodeLine(line)
	if !errors.Is(err, ErrMalformedEnvelope) {
		t.Fatalf("expected ErrMalformedEnvelope, got %v", err)
	}
}

func TestDecodeLine_WrongVersion(t *testing.T) {
	line := []byte(`{"v":2,"id":"x","kind":"request","method":"foo"}`)
	_, err := DecodeLine(line)
	if !errors.Is(err, ErrMalformedEnvelope) {
		t.Fatalf("expected ErrMalformedEnvelope for wrong version, got %v", err)
	}
}

func TestDecodeLine_OversizedLine(t *testing.T) {
	big := make([]byte, 9*1024*1024)
	_, err := DecodeLine(big)
	if !errors.Is(err, ErrMessageTooLarge) {
		t.Fatalf("expected ErrMessageTooLarge, got %v", err)
	}
}

func TestEncodeResponse(t *testing.T) {
	result := map[string]any{"pong": true}
	b, err := EncodeResponse("test-id", result)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	var env Envelope
	if err := json.Unmarshal(b, &env); err != nil {
		t.Fatalf("failed to unmarshal encoded response: %v", err)
	}
	if env.Kind != KindResponse {
		t.Errorf("got Kind=%q, want response", env.Kind)
	}
	if env.ID != "test-id" {
		t.Errorf("got ID=%q, want test-id", env.ID)
	}
	if env.Result == nil {
		t.Error("expected result field to be present")
	}
	if env.Error != nil {
		t.Error("expected no error field in success response")
	}
}

func TestEncodeError(t *testing.T) {
	rpcErr := NewRPCError(CodeUnknownMethod, "unknown method: foo")
	b, err := EncodeError("err-id", rpcErr)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	var env Envelope
	if err := json.Unmarshal(b, &env); err != nil {
		t.Fatalf("failed to unmarshal encoded error: %v", err)
	}
	if env.Kind != KindResponse {
		t.Errorf("got Kind=%q, want response", env.Kind)
	}
	if env.Error == nil {
		t.Fatal("expected error field to be present")
	}
	if env.Error.Code != CodeUnknownMethod {
		t.Errorf("got error.code=%q, want %q", env.Error.Code, CodeUnknownMethod)
	}
}
