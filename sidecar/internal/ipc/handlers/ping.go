package handlers

import (
	"context"
	"encoding/json"
	"sync"
	"time"

	"github.com/studio-sound/studio/sidecar/internal/buildinfo"
	"github.com/studio-sound/studio/sidecar/internal/ipc"
)

// pingPayloadSchema is the JSON Schema for the system.ping payload (must be null).
const pingPayloadSchema = `{
	"$schema": "https://json-schema.org/draft/2020-12/schema",
	"type": "null"
}`

var (
	pingValidatorOnce sync.Once
	pingValidator     *ipc.Validator
	pingValidatorErr  error

	startTime = time.Now()
)

func pingValidatorInstance() (*ipc.Validator, error) {
	pingValidatorOnce.Do(func() {
		pingValidator, pingValidatorErr = ipc.NewValidator([]byte(pingPayloadSchema))
	})
	return pingValidator, pingValidatorErr
}

// PingHandler handles the system.ping method. It validates the payload (which
// must be null or absent), then returns liveness information about the sidecar.
func PingHandler(ctx context.Context, id string, payload json.RawMessage) (any, error) {
	v, err := pingValidatorInstance()
	if err != nil {
		return nil, &ipc.RPCError{Code: ipc.CodeInternalError, Message: "failed to load ping schema: " + err.Error()}
	}

	// Treat an absent payload (nil/empty) as JSON null.
	if len(payload) == 0 {
		payload = json.RawMessage(`null`)
	}

	if err := v.Validate(payload); err != nil {
		return nil, err
	}

	result := map[string]any{
		"pong":                      true,
		"sidecarVersion":            buildinfo.Version,
		"uptimeMs":                  time.Since(startTime).Milliseconds(),
		"supportedProtocolVersions": []int{1},
	}
	return result, nil
}
