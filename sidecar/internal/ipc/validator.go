package ipc

import (
	"bytes"
	"encoding/json"
	"fmt"

	"github.com/santhosh-tekuri/jsonschema/v5"
)

// Validator wraps a compiled JSON Schema and provides payload validation that
// returns typed *RPCError values on failure.
type Validator struct {
	schema *jsonschema.Schema
}

// NewValidator compiles the given JSON Schema bytes and returns a Validator
// that can validate payloads against that schema.
func NewValidator(schemaJSON []byte) (*Validator, error) {
	compiler := jsonschema.NewCompiler()
	// Add the schema bytes as a resource with a synthetic URL.
	if err := compiler.AddResource("schema.json", bytes.NewReader(schemaJSON)); err != nil {
		return nil, fmt.Errorf("add schema resource: %w", err)
	}
	schema, err := compiler.Compile("schema.json")
	if err != nil {
		return nil, fmt.Errorf("compile schema: %w", err)
	}
	return &Validator{schema: schema}, nil
}

// Validate validates the given JSON payload against the compiled schema.
// On success it returns nil. On failure it returns a *RPCError with
// Code == CodeInvalidPayload and a message describing the first validation
// error.
func (v *Validator) Validate(payload json.RawMessage) error {
	// Unmarshal the raw JSON into an interface{} so the schema validator can
	// inspect it.  Use json.Number to preserve numeric precision.
	dec := json.NewDecoder(bytes.NewReader(payload))
	dec.UseNumber()
	var val interface{}
	if err := dec.Decode(&val); err != nil {
		return &RPCError{
			Code:    CodeInvalidPayload,
			Message: "invalid JSON: " + err.Error(),
		}
	}

	if err := v.schema.Validate(val); err != nil {
		return &RPCError{
			Code:    CodeInvalidPayload,
			Message: err.Error(),
		}
	}
	return nil
}
