package ipc

import (
	"encoding/json"
	"errors"
)

const (
	CodeProtocolVersionMismatch = "PROTOCOL_VERSION_MISMATCH"
	CodeMalformedEnvelope       = "MALFORMED_ENVELOPE"
	CodeUnknownMethod           = "UNKNOWN_METHOD"
	CodeInvalidPayload          = "INVALID_PAYLOAD"
	CodeInternalError           = "INTERNAL_ERROR"
	CodeMessageTooLarge         = "MESSAGE_TOO_LARGE"
	CodeEchoTooLong             = "ECHO_TOO_LONG"
)

var (
	ErrMessageTooLarge   = errors.New("message too large")
	ErrMalformedEnvelope = errors.New("malformed envelope")
)

type RPCError struct {
	Code    string          `json:"code"`
	Message string          `json:"message"`
	Details json.RawMessage `json:"details,omitempty"`
}

func (e *RPCError) Error() string { return e.Code + ": " + e.Message }

func NewRPCError(code, message string) *RPCError {
	return &RPCError{Code: code, Message: message}
}

func (e *RPCError) WithDetails(v any) *RPCError {
	b, _ := json.Marshal(v)
	e.Details = b
	return e
}
