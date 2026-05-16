package ipc

import (
	"encoding/json"
	"errors"
	"testing"
)

const testSchema = `{
	"type": "object",
	"properties": {
		"name": { "type": "string" }
	},
	"required": ["name"]
}`

func TestNewValidator_InvalidSchema(t *testing.T) {
	_, err := NewValidator([]byte(`not json`))
	if err == nil {
		t.Fatal("expected error for invalid schema, got nil")
	}
}

func TestValidator_ValidPayload(t *testing.T) {
	v, err := NewValidator([]byte(testSchema))
	if err != nil {
		t.Fatalf("NewValidator failed: %v", err)
	}

	payload := json.RawMessage(`{"name":"test"}`)
	if err := v.Validate(payload); err != nil {
		t.Errorf("expected nil error for valid payload, got: %v", err)
	}
}

func TestValidator_MissingRequiredField(t *testing.T) {
	v, err := NewValidator([]byte(testSchema))
	if err != nil {
		t.Fatalf("NewValidator failed: %v", err)
	}

	payload := json.RawMessage(`{}`)
	err = v.Validate(payload)
	if err == nil {
		t.Fatal("expected error for missing required field, got nil")
	}

	var rpcErr *RPCError
	if !errors.As(err, &rpcErr) {
		t.Fatalf("expected *RPCError, got %T: %v", err, err)
	}
	if rpcErr.Code != CodeInvalidPayload {
		t.Errorf("expected code %q, got %q", CodeInvalidPayload, rpcErr.Code)
	}
}

func TestValidator_WrongType(t *testing.T) {
	v, err := NewValidator([]byte(testSchema))
	if err != nil {
		t.Fatalf("NewValidator failed: %v", err)
	}

	// name should be string, not number
	payload := json.RawMessage(`{"name":42}`)
	err = v.Validate(payload)
	if err == nil {
		t.Fatal("expected error for wrong type, got nil")
	}

	var rpcErr *RPCError
	if !errors.As(err, &rpcErr) {
		t.Fatalf("expected *RPCError, got %T: %v", err, err)
	}
	if rpcErr.Code != CodeInvalidPayload {
		t.Errorf("expected code %q, got %q", CodeInvalidPayload, rpcErr.Code)
	}
}

func TestValidator_InvalidJSON(t *testing.T) {
	v, err := NewValidator([]byte(testSchema))
	if err != nil {
		t.Fatalf("NewValidator failed: %v", err)
	}

	payload := json.RawMessage(`not json`)
	err = v.Validate(payload)
	if err == nil {
		t.Fatal("expected error for invalid JSON, got nil")
	}

	var rpcErr *RPCError
	if !errors.As(err, &rpcErr) {
		t.Fatalf("expected *RPCError, got %T: %v", err, err)
	}
	if rpcErr.Code != CodeInvalidPayload {
		t.Errorf("expected code %q, got %q", CodeInvalidPayload, rpcErr.Code)
	}
}
