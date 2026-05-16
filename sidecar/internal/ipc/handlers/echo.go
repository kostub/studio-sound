package handlers

import (
	"context"
	"encoding/json"
	"sync"
	"unicode/utf8"

	"github.com/studio-sound/studio/sidecar/internal/ipc"
)

// echoMaxChars is the maximum allowed length of the echo `text` field in
// Unicode code points. Single source of truth for the Go side; the canonical
// JSON Schema (schemas/system.echo.schema.json) carries the same constant for
// codegen/contract purposes — a unit test asserts the two stay in sync.
//
// The handler enforces this limit explicitly (via utf8.RuneCountInString
// below) rather than via the inline schema so over-long inputs surface as the
// dedicated CodeEchoTooLong error rather than a generic CodeInvalidPayload.
const echoMaxChars = 4096

// echoPayloadSchema is the JSON Schema for the system.echo payload, used
// only to validate structural shape (object with required string `text`).
// Length is enforced separately — see echoMaxChars docstring above.
const echoPayloadSchema = `{
	"$schema": "https://json-schema.org/draft/2020-12/schema",
	"type": "object",
	"additionalProperties": false,
	"required": ["text"],
	"properties": {
		"text": { "type": "string" }
	}
}`

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
// the echo schema, enforces a 4096-character maximum on the text field, and returns
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

	// Enforce the 4096-character limit on the text field.
	if utf8.RuneCountInString(p.Text) > echoMaxChars {
		return nil, &ipc.RPCError{
			Code:    ipc.CodeEchoTooLong,
			Message: "echo text exceeds maximum length of 4096 characters",
		}
	}

	return map[string]string{"text": p.Text}, nil
}
