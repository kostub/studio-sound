package handlers

import (
	"context"
	"encoding/json"
	"log/slog"
	"sync"
	"time"

	"github.com/studio-sound/studio/sidecar/internal/ipc"
	"github.com/studio-sound/studio/sidecar/internal/media"
)

// probePayloadSchema is the JSON Schema for the media.probe payload. It
// mirrors $defs.ProbePayload in schemas/media.probe.schema.json.
const probePayloadSchema = `{
	"$schema": "https://json-schema.org/draft/2020-12/schema",
	"type": "object",
	"additionalProperties": false,
	"required": ["path"],
	"properties": {
		"path": { "type": "string", "minLength": 1, "maxLength": 4096 }
	}
}`

var (
	probeValidatorOnce sync.Once
	probeValidator     *ipc.Validator
	probeValidatorErr  error
)

func probeValidatorInstance() (*ipc.Validator, error) {
	probeValidatorOnce.Do(func() {
		probeValidator, probeValidatorErr = ipc.NewValidator([]byte(probePayloadSchema))
	})
	return probeValidator, probeValidatorErr
}

// probePayload mirrors the ProbePayload generated type for local decoding.
type probePayload struct {
	Path string `json:"path"`
}

// ProbeHandler handles the media.probe method. It validates the payload,
// resolves the bundled ffprobe binary, applies a 10-second per-probe
// deadline, and delegates to media.Probe.
func ProbeHandler(ctx context.Context, id string, payload json.RawMessage) (any, error) {
	v, err := probeValidatorInstance()
	if err != nil {
		return nil, &ipc.RPCError{Code: ipc.CodeInternalError, Message: "failed to load probe schema: " + err.Error()}
	}

	// Treat an absent payload as an invalid payload (path is required).
	if len(payload) == 0 {
		payload = json.RawMessage(`{}`)
	}

	if err := v.Validate(payload); err != nil {
		return nil, err
	}

	var req probePayload
	if err := json.Unmarshal(payload, &req); err != nil {
		return nil, &ipc.RPCError{Code: ipc.CodeInvalidPayload, Message: "failed to decode payload: " + err.Error()}
	}

	// Resolve ffprobe on each call so env-var changes are picked up (and tests
	// can override via t.Setenv without fighting a cached sync.Once value).
	ffprobePath, resolveErr := media.ResolveFFprobe()
	if resolveErr != nil {
		return nil, &ipc.RPCError{Code: ipc.CodeFFprobeMissing, Message: resolveErr.Error()}
	}

	slog.Info("probe_started", "id", id, "path", req.Path)
	start := time.Now()

	probeCtx, cancel := context.WithTimeout(ctx, 10*time.Second)
	defer cancel()

	result, probeErr := media.Probe(probeCtx, ffprobePath, req.Path)
	elapsed := time.Since(start).Milliseconds()
	if probeErr != nil {
		code := ""
		var rpc *ipc.RPCError
		if ok := isRPCError(probeErr, &rpc); ok {
			code = rpc.Code
		}
		slog.Warn("probe_failed", "id", id, "duration_ms", elapsed, "code", code)
		return nil, probeErr
	}

	slog.Info("probe_completed", "id", id, "duration_ms", elapsed,
		"supported", result.Compatibility.Supported)
	return result, nil
}

// isRPCError checks whether err is a *ipc.RPCError and, if so, sets out.
func isRPCError(err error, out **ipc.RPCError) bool {
	if rpc, ok := err.(*ipc.RPCError); ok {
		*out = rpc
		return true
	}
	return false
}
