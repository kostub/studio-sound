package ipc

import (
	"encoding/json"
	"fmt"
)

const (
	ProtocolVersion = 1
	maxMessageSize  = 8 * 1024 * 1024 // 8 MiB
)

type Kind string

const (
	KindRequest  Kind = "request"
	KindResponse Kind = "response"
	KindEvent    Kind = "event"
)

type Envelope struct {
	V       int             `json:"v"`
	ID      string          `json:"id"`
	Kind    Kind            `json:"kind"`
	Method  string          `json:"method,omitempty"`
	Payload json.RawMessage `json:"payload,omitempty"`
	Result  json.RawMessage `json:"result,omitempty"`
	Error   *RPCError       `json:"error,omitempty"`
}

// DecodeLine parses one NDJSON envelope from `line`. On error, the returned
// Envelope is *not* empty: we best-effort extract the `id` field so the caller
// can still echo it back in the error response. This is how the Rust client
// correlates malformed-envelope errors to their pending request — if `id` is
// missing the request will only complete via timeout.
func DecodeLine(line []byte) (Envelope, error) {
	if len(line) > maxMessageSize {
		return bestEffortEnvelope(line), ErrMessageTooLarge
	}
	var env Envelope
	if err := json.Unmarshal(line, &env); err != nil {
		return bestEffortEnvelope(line), fmt.Errorf("%w: %v", ErrMalformedEnvelope, err)
	}
	if env.V != ProtocolVersion {
		return env, fmt.Errorf("%w: got v=%d, want v=%d", ErrProtocolVersionMismatch, env.V, ProtocolVersion)
	}
	return env, nil
}

// bestEffortEnvelope tries to extract just the `id` field from a malformed
// or oversized line so that the error response can be correlated back to the
// caller's pending request. Any parse failure here is silently ignored — the
// resulting envelope simply carries `id=""`, which matches the prior behaviour.
func bestEffortEnvelope(line []byte) Envelope {
	if len(line) > maxMessageSize {
		// Don't try to parse 8+ MiB of garbage just to fish out an id.
		return Envelope{}
	}
	var partial struct {
		ID string `json:"id"`
	}
	_ = json.Unmarshal(line, &partial)
	return Envelope{ID: partial.ID}
}

func EncodeResponse(id string, result any) ([]byte, error) {
	r, err := json.Marshal(result)
	if err != nil {
		return nil, err
	}
	env := Envelope{V: ProtocolVersion, ID: id, Kind: KindResponse, Result: r}
	return json.Marshal(env)
}

func EncodeError(id string, rpcErr *RPCError) ([]byte, error) {
	env := Envelope{V: ProtocolVersion, ID: id, Kind: KindResponse, Error: rpcErr}
	return json.Marshal(env)
}
