package handlers

import (
	"context"
	"encoding/json"
	"sync"

	"github.com/studio-sound/studio/sidecar/internal/ipc"
)

// shutdownPayloadSchema is the JSON Schema for the system.shutdown payload
// (must be null).
const shutdownPayloadSchema = `{
	"$schema": "https://json-schema.org/draft/2020-12/schema",
	"type": "null"
}`

var (
	shutdownValidatorOnce sync.Once
	shutdownValidator     *ipc.Validator
	shutdownValidatorErr  error
)

func shutdownValidatorInstance() (*ipc.Validator, error) {
	shutdownValidatorOnce.Do(func() {
		shutdownValidator, shutdownValidatorErr = ipc.NewValidator([]byte(shutdownPayloadSchema))
	})
	return shutdownValidator, shutdownValidatorErr
}

// ShutdownHandler returns a Handler that validates the payload (which must be
// null or absent), responds with {"accepted": true}, and then triggers graceful
// shutdown by calling cancelFn on a fresh goroutine so the response can be
// flushed before the serve loop exits.
func ShutdownHandler(cancelFn context.CancelFunc) ipc.Handler {
	return func(ctx context.Context, id string, payload json.RawMessage) (any, error) {
		v, err := shutdownValidatorInstance()
		if err != nil {
			return nil, &ipc.RPCError{Code: ipc.CodeInternalError, Message: "failed to load shutdown schema: " + err.Error()}
		}

		// Treat an absent payload (nil/empty) as JSON null.
		if len(payload) == 0 {
			payload = json.RawMessage(`null`)
		}

		if err := v.Validate(payload); err != nil {
			return nil, err
		}

		// Respond immediately, then cancel the context so the serve loop exits
		// after the response has been flushed.
		go cancelFn()

		return map[string]bool{"accepted": true}, nil
	}
}
