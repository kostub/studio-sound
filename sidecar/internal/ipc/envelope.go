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

func DecodeLine(line []byte) (Envelope, error) {
	if len(line) > maxMessageSize {
		return Envelope{}, ErrMessageTooLarge
	}
	var env Envelope
	if err := json.Unmarshal(line, &env); err != nil {
		return Envelope{}, fmt.Errorf("%w: %v", ErrMalformedEnvelope, err)
	}
	if env.V != ProtocolVersion {
		return Envelope{}, fmt.Errorf("%w: got v=%d, want v=%d", ErrMalformedEnvelope, env.V, ProtocolVersion)
	}
	return env, nil
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
