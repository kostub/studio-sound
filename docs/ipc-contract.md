# IPC Contract — Developer Reference

This document is the developer reference for the Studio Sound sidecar IPC
protocol. The canonical design rationale lives in the requirements document
([docs/reqs/phase1-ipc-contract.md](reqs/phase1-ipc-contract.md)) and the ADR
([docs/adr/2026-05-14-ipc-contract.md](adr/2026-05-14-ipc-contract.md)). This
page focuses on day-to-day use: wire format, available methods, and how to add
a new method.

---

## Wire format

All messages are **Newline-Delimited JSON (NDJSON)**: one JSON object per line,
terminated with `\n`. The sidecar reads from `stdin` and writes to `stdout`.
Maximum message size in either direction is **8 MiB** (messages exceeding this
limit are rejected with a `MESSAGE_TOO_LARGE` error).

### Envelope fields

| Field     | Type                                   | Required          | Notes                                                             |
| --------- | -------------------------------------- | ----------------- | ----------------------------------------------------------------- |
| `v`       | `integer` (must be `1`)                | always            | Protocol version. Mismatches produce `PROTOCOL_VERSION_MISMATCH`. |
| `id`      | `string` (1–64 chars)                  | always            | Correlation ID, echoed in the response.                           |
| `kind`    | `"request"` / `"response"` / `"event"` | always            | Requests come from Rust; responses come from Go.                  |
| `method`  | `string`                               | requests          | Dot-namespaced method name, e.g. `system.ping`.                   |
| `payload` | `object` or `null`                     | requests          | Method-specific input.                                            |
| `result`  | `object` or `null`                     | success responses | Method-specific output.                                           |
| `error`   | error object (see below)               | error responses   | Present only on errors; mutually exclusive with `result`.         |

### Error object

```json
{
  "code":    "SNAKE_CASE_ERROR_CODE",
  "message": "Human-readable description (max 500 chars)",
  "details": { ... }
}
```

### Reserved error codes

| Code                        | Emitter   | Meaning                                                    |
| --------------------------- | --------- | ---------------------------------------------------------- |
| `PROTOCOL_VERSION_MISMATCH` | Go        | `v` field is not `1`.                                      |
| `MALFORMED_ENVELOPE`        | Go        | Envelope is not valid JSON or missing required fields.     |
| `UNKNOWN_METHOD`            | Go        | No handler registered for the requested method.            |
| `INVALID_PAYLOAD`           | Go        | Payload fails JSON Schema validation.                      |
| `INTERNAL_ERROR`            | Go        | Unexpected handler panic or serialisation failure.         |
| `MESSAGE_TOO_LARGE`         | Go / Rust | Message exceeds the 8 MiB limit.                           |
| `ECHO_TOO_LONG`             | Go        | Echo `text` exceeds 4 096 characters.                      |
| `SIDECAR_UNAVAILABLE`       | Rust      | Sidecar is not in the `Up` state.                          |
| `SIDECAR_BUSY`              | Rust      | More than 64 requests are in flight.                       |
| `TIMEOUT`                   | Rust      | The per-method deadline expired before a response arrived. |

---

## Available methods

### `system.ping`

Liveness probe. Returns runtime information about the sidecar.

**Request**

```json
{ "v": 1, "id": "<id>", "kind": "request", "method": "system.ping", "payload": null }
```

**Response (success)**

```json
{
  "v": 1,
  "id": "<id>",
  "kind": "response",
  "result": {
    "pong": true,
    "sidecarVersion": "0.1.0",
    "uptimeMs": 1234,
    "supportedProtocolVersions": [1]
  }
}
```

| Result field                | Type      | Notes                                              |
| --------------------------- | --------- | -------------------------------------------------- |
| `pong`                      | `true`    | Always `true`.                                     |
| `sidecarVersion`            | string    | Matches `-ldflags "-X .../buildinfo.Version=<x>"`. |
| `uptimeMs`                  | integer   | Milliseconds since sidecar start.                  |
| `supportedProtocolVersions` | integer[] | Currently `[1]`.                                   |

---

### `system.echo`

Returns the same text it was sent. Useful for round-trip latency measurement.

**Request**

```json
{ "v": 1, "id": "<id>", "kind": "request", "method": "system.echo", "payload": { "text": "hello" } }
```

| Payload field | Type   | Constraint                                            |
| ------------- | ------ | ----------------------------------------------------- |
| `text`        | string | Required. Max 4 096 characters (Unicode code points). |

**Response (success)**

```json
{ "v": 1, "id": "<id>", "kind": "response", "result": { "text": "hello" } }
```

**Error: text too long**

```json
{
  "v": 1,
  "id": "<id>",
  "kind": "response",
  "error": {
    "code": "ECHO_TOO_LONG",
    "message": "echo text exceeds maximum length of 4096 characters"
  }
}
```

---

### `system.shutdown`

Requests graceful sidecar shutdown. The sidecar responds with
`{"accepted":true}`, then cancels its serve loop. Rust closes stdin on the
child after sending the request, so the blocking `readLine` in Go unblocks and
the process exits cleanly with code `0`.

**Request**

```json
{ "v": 1, "id": "<id>", "kind": "request", "method": "system.shutdown", "payload": null }
```

**Response**

```json
{ "v": 1, "id": "<id>", "kind": "response", "result": { "accepted": true } }
```

---

## How to add a new method

Adding a method is mechanical. Follow these steps in order:

### 1. Define the schema

Create `schemas/<namespace>.<name>.schema.json` using JSON Schema Draft 2020-12.
Follow the pattern of the existing schemas — define `$defs` for
`<Name>Payload` and `<Name>Result`. Example:

```json
{
  "$schema": "https://json-schema.org/draft/2020-12/schema",
  "$id": "https://studiosound.app/schemas/system.greet.schema.json",
  "title": "SystemGreet",
  "type": "object",
  "$defs": {
    "GreetPayload": {
      "type": "object",
      "additionalProperties": false,
      "required": ["name"],
      "properties": { "name": { "type": "string", "maxLength": 100 } }
    },
    "GreetResult": {
      "type": "object",
      "additionalProperties": false,
      "required": ["greeting"],
      "properties": { "greeting": { "type": "string" } }
    }
  }
}
```

### 2. Regenerate types

```bash
npm run gen:schemas
```

This updates:

- `app/src/ipc/generated/*.ts` — TypeScript types
- `sidecar/internal/ipc/generated/*.go` — Go types
- `app/src-tauri/src/ipc/generated.rs` — Rust types

Commit the regenerated files. CI will fail the dirty-tree check if you forget.

### 3. Write the Go handler

Create `sidecar/internal/ipc/handlers/<name>.go`. Use `handlers/echo.go` as a
template:

```go
package handlers

import (
    "context"
    "encoding/json"
    "github.com/studio-sound/studio/sidecar/internal/ipc"
)

type greetPayload struct {
    Name string `json:"name"`
}

func GreetHandler(ctx context.Context, id string, payload json.RawMessage) (any, error) {
    var p greetPayload
    if err := json.Unmarshal(payload, &p); err != nil {
        return nil, &ipc.RPCError{Code: ipc.CodeInvalidPayload, Message: err.Error()}
    }
    return map[string]string{"greeting": "Hello, " + p.Name + "!"}, nil
}
```

Add a JSON Schema validator (see `handlers/echo.go`) if the payload needs
schema-level validation beyond `json.Unmarshal`.

### 4. Register the handler

In `sidecar/internal/cli/cli.go` inside the `case "serve":` block:

```go
d.Register("system.greet", handlers.GreetHandler)
```

### 5. Add the Tauri command

In `app/src-tauri/src/ipc/commands.rs`:

```rust
#[tauri::command]
pub async fn greet(
    state: tauri::State<'_, IpcState>,
    name: String,
) -> Result<GreetResult, IpcError> {
    state.client.call("system.greet", GreetPayload { name }, Duration::from_secs(5)).await
}
```

Register it in `lib.rs` inside `.invoke_handler(tauri::generate_handler![..., ipc::commands::greet])`.

### 6. Add the TypeScript client wrapper

In `app/src/ipc/client.ts`:

```ts
export async function greet(name: string): Promise<GreetResult> {
  return invoke<GreetResult>('greet', { name });
}
```

### 7. Write tests

- Go unit test: `sidecar/internal/ipc/handlers/<name>_test.go`
- Rust unit test: inline `#[cfg(test)]` mod in `commands.rs`
- Frontend unit test: `app/src/ipc/client.test.ts`

---

## Log rotation and log file location

### Sidecar log

The sidecar writes structured JSON logs. The log destination is controlled by
the `STUDIO_LOG_FILE` environment variable (or the `--log-file` CLI flag when
running standalone). When running inside Tauri, the Rust supervisor sets
`STUDIO_LOG_FILE` to `<log_dir>/sidecar.log`.

Log rotation (via `lumberjack`): 10 MiB per file, 3 compressed backups.

### Tauri log

The Tauri layer writes to `<log_dir>/tauri.log` via `tracing-appender`.

### Log directory per OS

| OS      | Path                                       |
| ------- | ------------------------------------------ |
| macOS   | `~/Library/Logs/com.studiosound.app/`      |
| Windows | `%LOCALAPPDATA%\com.studiosound.app\Logs\` |

The directory is created automatically on first launch if it does not exist.

**Tip:** Use the "Open Logs Folder" button in the Diagnostics screen
(`Cmd/Ctrl+Shift+D` in development builds, or `?diag=1` in the URL) to open
the directory in Finder/Explorer.

---

## Testing IPC manually

Build the sidecar:

```bash
npm run sidecar:build
```

Pipe NDJSON lines to the `serve` subcommand (macOS Apple Silicon example):

```bash
echo '{"v":1,"id":"t1","kind":"request","method":"system.ping","payload":null}' \
  | app/src-tauri/binaries/studio-sidecar-aarch64-apple-darwin serve
```

Expected response:

```json
{
  "v": 1,
  "id": "t1",
  "kind": "response",
  "result": {
    "pong": true,
    "sidecarVersion": "0.1.0",
    "uptimeMs": 0,
    "supportedProtocolVersions": [1]
  }
}
```

To run the Go-level E2E integration test:

```bash
cd sidecar && go test -tags=integration ./internal/ipc/... -v -run TestE2EIntegration
```
