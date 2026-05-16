package handlers

import (
	"context"
	"encoding/json"
	"sync"

	"github.com/studio-sound/studio/sidecar/internal/ipc"
)

// echoPayloadSchema is the JSON Schema for the system.echo payload.
const echoPayloadSchema = `{
	"$schema": "https://json-schema.org/draft/2020-12/schema",
	"type": "object",
	"additionalProperties": false,
	"required": ["text"],
	"properties": {
		"text": { "type": "string", "maxLength": 4096 }
	}
}`

const echoMaxBytes = 1024

var (
	echoValidatorOnce sync.Once
	echoValidator     *ipc.Validator
	echoValidatorErr  error
)

func echoValidatorInstance() (*ipc.Validator, error) {
	echoValidatorOnce.Do(func() {
		echoValidator, echoValidatorErr = ipc.NewValidator([]byte(echoPayloadSchema))
	})
	return echoValidator, echoValidatorErr
}

// echoPayload mirrors the EchoPayload generated type for local decoding.
type echoPayload struct {
	Text string `json:"text"`
}

// EchoHandler handles the system.echo method. It validates the payload against
// the echo schema, enforces a 1024-byte maximum on the text field, and returns
// the same text echoed back in the result.
func EchoHandler(ctx context.Context, id string, payload json.RawMessage) (any, error) {
	v, err := echoValidatorInstance()
	if err != nil {
		return nil, &ipc.RPCError{Code: ipc.CodeInternalError, Message: "failed to load echo schema: " + err.Error()}
	}

	// Treat an absent payload as an invalid payload (text is required).
	if len(payload) == 0 {
		payload = json.RawMessage(`{}`)
	}

	if err := v.Validate(payload); err != nil {
		return nil, err
	}

	var p echoPayload
	if err := json.Unmarshal(payload, &p); err != nil {
		return nil, &ipc.RPCError{Code: ipc.CodeInvalidPayload, Message: "failed to decode payload: " + err.Error()}
	}

	// Enforce the 1024-byte limit on the text field.
	if len(p.Text) > echoMaxBytes {
		return nil, &ipc.RPCError{
			Code:    ipc.CodeEchoTooLong,
			Message: "echo text exceeds maximum length of 1024 bytes",
		}
	}

	return map[string]string{"text": p.Text}, nil
}
