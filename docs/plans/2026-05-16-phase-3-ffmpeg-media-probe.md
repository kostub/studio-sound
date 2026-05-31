# Phase 3 — FFmpeg Bundling & Media Probe Implementation Plan

> This plan is consumed by the `plan-to-pr` skill. Items are numbered globally across all PRs.
> Commit subjects MUST start with `[item N]` so plan-to-pr can resume after partial failure.

**LLD:** `docs/lld/2026-05-16-phase-3-ffmpeg-media-probe.md`
**PRD:** `docs/reqs/phase_3_ffmpeg_media_probe_prd_and_ui_design.md`
**Goal:** Bundle `ffprobe`, expose `media.probe` over IPC, and ship the full single-file workspace UI (empty state, active card, diagnostics drawer, replace dialog, error/unsupported panels) so creators can ingest one video at a time and see canonical metadata + compatibility verdict.
**Architecture:** A new `internal/media` Go package wraps a bundled `ffprobe` subprocess and produces a canonical `MediaProbeResult` schema (TS/Go/Rust codegen). A new `media.probe` IPC method routes through the existing dispatcher; the Rust supervisor resolves the bundled binary at startup and exports `STUDIO_FFPROBE_PATH` to the child. The Tauri command boundary upgrades from stringified errors to a structured `SerializableIpcError`. The frontend introduces a Zustand workspace store and a complete workspace UI (drop-only, no file picker in Phase 3).
**Tech Stack:** Go 1.24 (sidecar), Rust + Tauri 2 (shell), React + TS + Vitest (UI), Zustand (state), `tauri-plugin-shell` (subprocess), `json-schema-to-typescript` / `go-jsonschema` / `cargo-typify` (codegen), BtbN/FFmpeg-Builds LGPL 7.x (bundled ffprobe).

---

## Decisions that supersede the LLD §4 assumptions

The user confirmed the following during planning. **Where these conflict with the LLD body, this plan wins.**

1. **macOS codesigning is OUT of scope for Phase 3.** No codesign post-build script. Document the `xattr -d com.apple.quarantine` manual workaround in `docs/troubleshooting.md`. Notarisation and codesigning both deferred to a release-hardening phase.
2. **`audio` field is nullable, not optional.** Schema marks `audio` as `oneOf: [Audio, null]` and includes it in `required`. Normalize always assigns either an `Audio` object or explicit `null`.
3. **Unsupported files surface as success-with-`compatibility.supported=false`, NOT as RPC errors.** Only `FILE_NOT_FOUND`, `ACCESS_DENIED`, `CORRUPT_MEDIA`, `FFPROBE_FAILURE`, `FFPROBE_MISSING` are RPC errors. `UNSUPPORTED_CONTAINER`, `UNSUPPORTED_CODEC`, `NO_AUDIO_STREAM` are NOT error codes — they are populated into `compatibility.issues[]` on a successful response with `supported=false`. The UI branches on (ok vs err) × (supported vs unsupported).
4. **Drop-only in Phase 3 — no Browse button.** Drop `tauri-plugin-dialog` Rust dep, `@tauri-apps/plugin-dialog` npm dep, and the `dialog:default` capability from the LLD. `EmptyState` shows the drop zone + supported-formats line + privacy line only (no browse CTA).
5. **`AboutPanel.tsx` deferred.** Ship README + `third_party/LICENSE.ffmpeg-lgpl.txt` only in Phase 3.
6. **Target-triple matching** uses compile-time `cfg!(target_os = "...")` + `cfg!(target_arch = "...")` (matches LLD).
7. **Keep the string-rejection fallback** in `toIpcError` (matches LLD).

---

## PR-stacking overview

| PR | Title | Items | Base |
|---|---|---|---|
| 1 | IPC contract: `media.probe` schema + structured Tauri errors | 1–11 | `master` |
| 2 | Build-time ffprobe bundling + supervisor wire-up | 12–21 | `feature/phase-3-pr1` |
| 3 | Sidecar `internal/media` subprocess layer (locator + runner + fakeffprobe) | 22–28 | `feature/phase-3-pr2` |
| 4 | Sidecar `internal/media` semantic layer (parser + normalize + compat + errors + orchestrator) | 29–38 | `feature/phase-3-pr3` |
| 5 | `media.probe` handler + integration tests + binary fixtures | 39–45 | `feature/phase-3-pr4` |
| 6 | Tauri `media_probe` command + IPC wrapper + workspace state machine | 46–52 | `feature/phase-3-pr5` |
| 7 | Workspace UI core flow (Shell + EmptyState + ActiveFileCard + useDropTarget + tokens) | 53–61 | `feature/phase-3-pr6` |
| 8 | Workspace UI secondary surfaces (DiagnosticsDrawer + ReplaceFileDialog + ErrorPanel + UnsupportedPanel) | 62–69 | `feature/phase-3-pr7` |

Total: 69 items across 8 PRs.

---

## PR 1: IPC contract — `media.probe` schema + structured Tauri errors

**Branch:** `feature/phase-3-pr1`
**Base:** `master`
**Items:** 1–11
**Goal:** Land the canonical `MediaProbeResult` schema (regenerated for TS/Go/Rust), add the new error-code constants, and upgrade all Tauri commands from `Result<Value, String>` to `Result<Value, SerializableIpcError>` so the frontend receives structured `{ code, message, details? }` errors. No new behaviour visible to the user yet; the sidecar still doesn't know `media.probe`.

### Task 1.1: Create the `media.probe` JSON Schema

**Files:**
- Create: `schemas/media.probe.schema.json`

- [ ] **Step 1: Write the schema file**

  ```json
  {
    "$schema": "https://json-schema.org/draft/2020-12/schema",
    "$id": "https://studiosound.app/schemas/media.probe.schema.json",
    "title": "MediaProbe",
    "type": "object",
    "$defs": {
      "ProbePayload": {
        "type": "object",
        "additionalProperties": false,
        "required": ["path"],
        "properties": {
          "path": { "type": "string", "minLength": 1, "maxLength": 4096 }
        }
      },
      "ProbeResult": {
        "type": "object",
        "additionalProperties": false,
        "required": ["id", "path", "filename", "sizeBytes", "container", "audio", "compatibility"],
        "properties": {
          "id":              { "type": "string", "minLength": 1, "maxLength": 64 },
          "path":            { "type": "string", "minLength": 1, "maxLength": 4096 },
          "filename":        { "type": "string", "minLength": 1, "maxLength": 1024 },
          "sizeBytes":       { "type": "integer", "minimum": 0 },
          "durationSeconds": { "type": "number",  "minimum": 0 },
          "container":       { "$ref": "#/$defs/Container" },
          "video":           { "$ref": "#/$defs/Video" },
          "audio": {
            "oneOf": [
              { "$ref": "#/$defs/Audio" },
              { "type": "null" }
            ]
          },
          "compatibility":   { "$ref": "#/$defs/Compatibility" }
        }
      },
      "Container": {
        "type": "object", "additionalProperties": false,
        "required": ["format", "longName"],
        "properties": {
          "format":   { "type": "string", "maxLength": 256 },
          "longName": { "type": "string", "maxLength": 1024 }
        }
      },
      "Video": {
        "type": "object", "additionalProperties": false,
        "required": ["codec", "width", "height", "fps"],
        "properties": {
          "codec":       { "type": "string", "maxLength": 64 },
          "width":       { "type": "integer", "minimum": 0 },
          "height":      { "type": "integer", "minimum": 0 },
          "fps":         { "type": "number",  "minimum": 0 },
          "bitrate":     { "type": "integer", "minimum": 0 },
          "pixelFormat": { "type": "string", "maxLength": 64 }
        }
      },
      "Audio": {
        "type": "object", "additionalProperties": false,
        "required": ["codec", "channels", "sampleRate", "trackIndex", "trackCount", "tracks"],
        "properties": {
          "codec":       { "type": "string", "maxLength": 64 },
          "channels":    { "type": "integer", "minimum": 0 },
          "sampleRate":  { "type": "integer", "minimum": 0 },
          "bitrate":     { "type": "integer", "minimum": 0 },
          "layout":      { "type": "string", "maxLength": 128 },
          "trackIndex":  { "type": "integer", "minimum": 0 },
          "trackCount":  { "type": "integer", "minimum": 1 },
          "tracks": {
            "type": "array", "minItems": 1,
            "items": { "$ref": "#/$defs/AudioTrack" }
          }
        }
      },
      "AudioTrack": {
        "type": "object", "additionalProperties": false,
        "required": ["index", "codec", "channels", "sampleRate", "isDefault"],
        "properties": {
          "index":      { "type": "integer", "minimum": 0 },
          "codec":      { "type": "string", "maxLength": 64 },
          "channels":   { "type": "integer", "minimum": 0 },
          "sampleRate": { "type": "integer", "minimum": 0 },
          "bitrate":    { "type": "integer", "minimum": 0 },
          "layout":     { "type": "string", "maxLength": 128 },
          "title":      { "type": "string", "maxLength": 256 },
          "language":   { "type": "string", "maxLength": 32 },
          "isDefault":  { "type": "boolean" }
        }
      },
      "Compatibility": {
        "type": "object", "additionalProperties": false,
        "required": ["supported", "issues", "warnings"],
        "properties": {
          "supported": { "type": "boolean" },
          "issues":    { "type": "array", "items": { "type": "string", "maxLength": 512 } },
          "warnings":  { "type": "array", "items": { "type": "string", "maxLength": 512 } }
        }
      }
    }
  }
  ```

- [ ] **Step 2: Validate the schema syntactically**

  Run: `npx --no-install ajv compile --spec=draft2020 -s schemas/media.probe.schema.json`
  Expected: `schema schemas/media.probe.schema.json is valid` (or equivalent success).

- [ ] **Step 3: Commit**

  ```bash
  git add schemas/media.probe.schema.json
  git commit -m "[item 1] Add media.probe JSON Schema with nullable audio"
  ```

### Task 1.2: Register the schema in the codegen pipeline and regenerate

**Files:**
- Modify: `scripts/gen-schemas.mjs:12-17`
- Create: `app/src/ipc/generated/media.probe.ts` (regenerated)
- Create: `sidecar/internal/ipc/generated/media_probe.go` (regenerated)

- [ ] **Step 1: Append media.probe to the schemaFiles array**

  In `scripts/gen-schemas.mjs`, change:

  ```js
  const schemaFiles = [
    'envelope.schema.json',
    'system.ping.schema.json',
    'system.echo.schema.json',
    'system.shutdown.schema.json',
  ];
  ```

  to:

  ```js
  const schemaFiles = [
    'envelope.schema.json',
    'system.ping.schema.json',
    'system.echo.schema.json',
    'system.shutdown.schema.json',
    'media.probe.schema.json',
  ];
  ```

- [ ] **Step 2: Run codegen**

  Run: `npm run gen:schemas`
  Expected: `gen-schemas: wrote ...` success; new files appear at `app/src/ipc/generated/media.probe.ts` and `sidecar/internal/ipc/generated/media_probe.go`.

- [ ] **Step 3: Verify generated TS contains ProbePayload and ProbeResult**

  Run: `grep -E 'ProbePayload|ProbeResult' app/src/ipc/generated/media.probe.ts`
  Expected: matches for both `ProbePayload` and `ProbeResult` exports.

- [ ] **Step 4: Verify generated Go contains the types**

  Run: `grep -E 'ProbePayload|ProbeResult' sidecar/internal/ipc/generated/media_probe.go`
  Expected: matches for both types.

- [ ] **Step 5: Verify Go compiles**

  Run: `cd sidecar && go build ./...`
  Expected: exit 0.

- [ ] **Step 6: Verify TS compiles**

  Run: `npm --prefix app run typecheck`
  Expected: exit 0.

- [ ] **Step 7: Commit**

  ```bash
  git add scripts/gen-schemas.mjs app/src/ipc/generated/media.probe.ts sidecar/internal/ipc/generated/media_probe.go
  git commit -m "[item 2] Register media.probe schema and regenerate TS/Go bindings"
  ```

### Task 1.3: Add new RPC error-code constants to the sidecar

**Files:**
- Modify: `sidecar/internal/ipc/errors.go:8-16`

- [ ] **Step 1: Write the failing test**

  Append to `sidecar/internal/ipc/errors_test.go` (create file if missing):

  ```go
  package ipc

  import "testing"

  func TestMediaProbeCodeConstants(t *testing.T) {
      cases := map[string]string{
          CodeFileNotFound:    "FILE_NOT_FOUND",
          CodeAccessDenied:    "ACCESS_DENIED",
          CodeCorruptMedia:    "CORRUPT_MEDIA",
          CodeFFprobeFailure:  "FFPROBE_FAILURE",
          CodeFFprobeMissing:  "FFPROBE_MISSING",
      }
      for got, want := range cases {
          if got != want {
              t.Errorf("constant value = %q, want %q", got, want)
          }
      }
  }
  ```

- [ ] **Step 2: Run the test to verify it fails**

  Run: `cd sidecar && go test ./internal/ipc -run TestMediaProbeCodeConstants`
  Expected: FAIL with `undefined: CodeFileNotFound` (or similar undefined-symbol errors).

- [ ] **Step 3: Add the five constants**

  In `sidecar/internal/ipc/errors.go`, extend the const block:

  ```go
  const (
      CodeProtocolVersionMismatch = "PROTOCOL_VERSION_MISMATCH"
      CodeMalformedEnvelope       = "MALFORMED_ENVELOPE"
      CodeUnknownMethod           = "UNKNOWN_METHOD"
      CodeInvalidPayload          = "INVALID_PAYLOAD"
      CodeInternalError           = "INTERNAL_ERROR"
      CodeMessageTooLarge         = "MESSAGE_TOO_LARGE"
      CodeEchoTooLong             = "ECHO_TOO_LONG"
      CodeFileNotFound            = "FILE_NOT_FOUND"
      CodeAccessDenied            = "ACCESS_DENIED"
      CodeCorruptMedia            = "CORRUPT_MEDIA"
      CodeFFprobeFailure          = "FFPROBE_FAILURE"
      CodeFFprobeMissing          = "FFPROBE_MISSING"
  )
  ```

- [ ] **Step 4: Run the test to verify it passes**

  Run: `cd sidecar && go test ./internal/ipc -run TestMediaProbeCodeConstants`
  Expected: PASS.

- [ ] **Step 5: Commit**

  ```bash
  git add sidecar/internal/ipc/errors.go sidecar/internal/ipc/errors_test.go
  git commit -m "[item 3] Add FILE_NOT_FOUND/ACCESS_DENIED/CORRUPT_MEDIA/FFPROBE_FAILURE/FFPROBE_MISSING constants"
  ```

### Task 1.4: Add `SerializableIpcError` and `From<IpcError>` impl on the Rust side

**Files:**
- Modify: `app/src-tauri/src/ipc/error.rs`

- [ ] **Step 1: Write the failing tests**

  Append to `app/src-tauri/src/ipc/error.rs`:

  ```rust
  #[cfg(test)]
  mod tests {
      use super::*;

      #[test]
      fn serializable_from_protocol_mismatch() {
          let s: SerializableIpcError = IpcError::ProtocolVersionMismatch { got: 2, want: 1 }.into();
          assert_eq!(s.code, "PROTOCOL_VERSION_MISMATCH");
          assert!(s.message.contains("got 2"));
          assert!(s.details.is_none());
      }

      #[test]
      fn serializable_from_timeout() {
          let s: SerializableIpcError = IpcError::Timeout.into();
          assert_eq!(s.code, "TIMEOUT");
      }

      #[test]
      fn serializable_from_unknown_method() {
          let s: SerializableIpcError = IpcError::UnknownMethod("media.bogus".into()).into();
          assert_eq!(s.code, "UNKNOWN_METHOD");
          assert!(s.message.contains("media.bogus"));
      }

      #[test]
      fn serializable_from_other_preserves_code_message_details() {
          let s: SerializableIpcError = IpcError::Other {
              code: "FILE_NOT_FOUND".into(),
              message: "missing".into(),
              details: Some(serde_json::json!({"path": "/x"})),
          }.into();
          assert_eq!(s.code, "FILE_NOT_FOUND");
          assert_eq!(s.message, "missing");
          assert_eq!(s.details.as_ref().unwrap()["path"], "/x");
      }

      #[test]
      fn serializable_round_trips_through_serde_json() {
          let s: SerializableIpcError = IpcError::SidecarUnavailable.into();
          let v = serde_json::to_value(&s).unwrap();
          assert_eq!(v["code"], "SIDECAR_UNAVAILABLE");
          assert!(v["message"].is_string());
          assert!(v.get("details").is_none(), "details omitted when None");
      }
  }
  ```

- [ ] **Step 2: Run the test to verify it fails**

  Run: `cd app/src-tauri && cargo test --lib ipc::error::tests`
  Expected: FAIL — `cannot find type SerializableIpcError`.

- [ ] **Step 3: Add the struct and impl**

  At the end of `app/src-tauri/src/ipc/error.rs`, add:

  ```rust
  #[derive(Debug, serde::Serialize)]
  pub struct SerializableIpcError {
      pub code: String,
      pub message: String,
      #[serde(skip_serializing_if = "Option::is_none")]
      pub details: Option<serde_json::Value>,
  }

  impl From<IpcError> for SerializableIpcError {
      fn from(err: IpcError) -> Self {
          match &err {
              IpcError::ProtocolVersionMismatch { .. } => Self {
                  code: "PROTOCOL_VERSION_MISMATCH".into(),
                  message: err.to_string(),
                  details: None,
              },
              IpcError::MalformedEnvelope(_) => Self {
                  code: "MALFORMED_ENVELOPE".into(),
                  message: err.to_string(),
                  details: None,
              },
              IpcError::Serde(_) => Self {
                  code: "MALFORMED_ENVELOPE".into(),
                  message: err.to_string(),
                  details: None,
              },
              IpcError::SidecarUnavailable => Self {
                  code: "SIDECAR_UNAVAILABLE".into(),
                  message: err.to_string(),
                  details: None,
              },
              IpcError::SidecarBusy => Self {
                  code: "SIDECAR_BUSY".into(),
                  message: err.to_string(),
                  details: None,
              },
              IpcError::Timeout => Self {
                  code: "TIMEOUT".into(),
                  message: err.to_string(),
                  details: None,
              },
              IpcError::UnknownMethod(_) => Self {
                  code: "UNKNOWN_METHOD".into(),
                  message: err.to_string(),
                  details: None,
              },
              IpcError::Other { code, message, details } => Self {
                  code: code.clone(),
                  message: message.clone(),
                  details: details.clone(),
              },
          }
      }
  }
  ```

- [ ] **Step 4: Run tests to verify they pass**

  Run: `cd app/src-tauri && cargo test --lib ipc::error::tests`
  Expected: PASS (5 test cases).

- [ ] **Step 5: Commit**

  ```bash
  git add app/src-tauri/src/ipc/error.rs
  git commit -m "[item 4] Add SerializableIpcError with From<IpcError> for Tauri command boundary"
  ```

### Task 1.5: Flip `ipc_ping` to `Result<Value, SerializableIpcError>`

**Files:**
- Modify: `app/src-tauri/src/commands.rs:38-45`

- [ ] **Step 1: Change the signature and mapping**

  In `app/src-tauri/src/commands.rs`, add the import at the top alongside existing `use`s:

  ```rust
  use crate::ipc::error::SerializableIpcError;
  ```

  Replace the `ipc_ping` body:

  ```rust
  #[tauri::command]
  pub async fn ipc_ping(
      client: State<'_, Arc<IpcClient>>,
  ) -> Result<serde_json::Value, SerializableIpcError> {
      client
          .call("system.ping", serde_json::Value::Null, default_timeout("system.ping"))
          .await
          .map_err(SerializableIpcError::from)
  }
  ```

- [ ] **Step 2: Verify Rust builds**

  Run: `cd app/src-tauri && cargo build`
  Expected: exit 0.

- [ ] **Step 3: Commit**

  ```bash
  git add app/src-tauri/src/commands.rs
  git commit -m "[item 5] ipc_ping returns SerializableIpcError instead of String"
  ```

### Task 1.6: Flip `ipc_echo` and `ipc_shutdown` to `Result<Value, SerializableIpcError>`

**Files:**
- Modify: `app/src-tauri/src/commands.rs:51-77`

- [ ] **Step 1: Update both signatures and mappings**

  Replace the `ipc_echo` and `ipc_shutdown` bodies:

  ```rust
  #[tauri::command]
  pub async fn ipc_echo(
      text: String,
      client: State<'_, Arc<IpcClient>>,
  ) -> Result<serde_json::Value, SerializableIpcError> {
      let payload = serde_json::json!({ "text": text });
      client
          .call("system.echo", payload, default_timeout("system.echo"))
          .await
          .map_err(SerializableIpcError::from)
  }

  #[tauri::command]
  pub async fn ipc_shutdown(
      client: State<'_, Arc<IpcClient>>,
  ) -> Result<serde_json::Value, SerializableIpcError> {
      client
          .call(
              "system.shutdown",
              serde_json::Value::Null,
              default_timeout("system.shutdown"),
          )
          .await
          .map_err(SerializableIpcError::from)
  }
  ```

  Also drop the now-obsolete Phase-6 comment at the top of the file ("Phase 6 will introduce structured error serialisation."). Leave the surrounding `# Error mapping` doc but reword that paragraph to:

  ```text
  //! # Error mapping
  //! Tauri requires that command errors implement `serde::Serialize`. The
  //! `SerializableIpcError` struct maps every `IpcError` variant to a
  //! `{ code, message, details? }` JSON shape consumed by the frontend
  //! `toIpcError` helper.
  ```

- [ ] **Step 2: Verify Rust builds**

  Run: `cd app/src-tauri && cargo build`
  Expected: exit 0.

- [ ] **Step 3: Commit**

  ```bash
  git add app/src-tauri/src/commands.rs
  git commit -m "[item 6] ipc_echo and ipc_shutdown return SerializableIpcError"
  ```

### Task 1.7: Teach `toIpcError` to recognise the structured error shape

**Files:**
- Modify: `app/src/ipc/client.ts:42-51`

- [ ] **Step 1: Write the failing test**

  Append to `app/src/ipc/client.test.ts` (create if missing — mirror existing test patterns in the repo if present):

  ```ts
  import { describe, it, expect } from 'vitest';
  import { toIpcError } from './client';

  describe('toIpcError', () => {
    it('returns structured error as-is when shape matches', () => {
      const e = toIpcError({ code: 'FILE_NOT_FOUND', message: 'missing' });
      expect(e).toEqual({ code: 'FILE_NOT_FOUND', message: 'missing' });
    });

    it('preserves details field when present', () => {
      const e = toIpcError({
        code: 'CORRUPT_MEDIA',
        message: 'invalid data',
        details: { stderrTail: 'moov atom not found' },
      });
      expect(e.code).toBe('CORRUPT_MEDIA');
      expect(e.details).toEqual({ stderrTail: 'moov atom not found' });
    });

    it('falls back to UNKNOWN for non-object string rejection', () => {
      const e = toIpcError('boom');
      expect(e.code).toBe('UNKNOWN');
      expect(e.message).toBe('boom');
    });

    it('falls back to UNKNOWN for null/undefined', () => {
      expect(toIpcError(null).code).toBe('UNKNOWN');
      expect(toIpcError(undefined).code).toBe('UNKNOWN');
    });
  });
  ```

- [ ] **Step 2: Run the test to verify it fails**

  Run: `npm --prefix app run test -- client.test.ts --run`
  Expected: FAIL — at least the "returns structured error as-is" and "preserves details" cases.

- [ ] **Step 3: Update `toIpcError`**

  In `app/src/ipc/client.ts`, replace the body of `toIpcError` with:

  ```ts
  export function toIpcError(err: unknown): IpcError {
    if (
      err !== null &&
      typeof err === 'object' &&
      typeof (err as { code?: unknown }).code === 'string' &&
      typeof (err as { message?: unknown }).message === 'string'
    ) {
      const e = err as { code: string; message: string; details?: unknown };
      return { code: e.code, message: e.message, ...(e.details !== undefined ? { details: e.details } : {}) };
    }
    if (typeof err === 'string') {
      return { code: 'UNKNOWN', message: err };
    }
    return { code: 'UNKNOWN', message: String(err ?? 'unknown error') };
  }
  ```

- [ ] **Step 4: Run the test to verify it passes**

  Run: `npm --prefix app run test -- client.test.ts --run`
  Expected: PASS (4 cases).

- [ ] **Step 5: Commit**

  ```bash
  git add app/src/ipc/client.ts app/src/ipc/client.test.ts
  git commit -m "[item 7] toIpcError accepts structured {code,message,details?} from Tauri commands"
  ```

### Task 1.8: Document `media.probe` and the new error codes in the IPC contract

**Files:**
- Modify: `docs/ipc-contract.md`

- [ ] **Step 1: Append the `media.probe` method section**

  Following the template used by other methods in `docs/ipc-contract.md`, append a new section describing `media.probe` with: request payload shape, response shape, supersedes-LLD note on `audio` being nullable, and per-error-code semantics (FILE_NOT_FOUND, ACCESS_DENIED, CORRUPT_MEDIA, FFPROBE_FAILURE, FFPROBE_MISSING) plus the success-with-`supported=false` contract for unsupported container/codec/no-audio cases.

  The section must explicitly say:

  > Unsupported container, unsupported codec, and no-audio-stream cases are reported as **successful** responses with `compatibility.supported=false` and human-readable strings in `compatibility.issues[]`. Only file-system errors (`FILE_NOT_FOUND`, `ACCESS_DENIED`), parse failures (`CORRUPT_MEDIA`), and runner errors (`FFPROBE_FAILURE`, `FFPROBE_MISSING`) surface as RPC errors.

- [ ] **Step 2: Append the five new error codes to the reserved-codes table** (`docs/ipc-contract.md:43-55`)

  Add rows for: `FILE_NOT_FOUND`, `ACCESS_DENIED`, `CORRUPT_MEDIA`, `FFPROBE_FAILURE`, `FFPROBE_MISSING` with one-line descriptions matching `sidecar/internal/ipc/errors.go`.

- [ ] **Step 3: Verify rendering**

  Run: `grep -E 'FILE_NOT_FOUND|ACCESS_DENIED|CORRUPT_MEDIA|FFPROBE_FAILURE|FFPROBE_MISSING' docs/ipc-contract.md`
  Expected: at least one match per code.

- [ ] **Step 4: Commit**

  ```bash
  git add docs/ipc-contract.md
  git commit -m "[item 8] Document media.probe method and new error codes in IPC contract"
  ```

### Task 1.9: Verify the generated TS file is consumed by `app/src/ipc`

**Files:**
- Modify (if needed): `app/src/ipc/index.ts` or wherever generated types are re-exported

- [ ] **Step 1: Inspect re-export convention**

  Run: `grep -rE "from.*generated" app/src/ipc/ --include='*.ts'`
  Expected: shows the existing pattern (e.g., `export * from './generated/system.echo'`).

- [ ] **Step 2: Add the matching re-export for media.probe**

  Following the same pattern as the existing system.* re-exports, add the media.probe types re-export so `ProbePayload` and `ProbeResult` are importable from `@/ipc` (or whatever the alias is). If there is no central re-export file, skip this step — consumers will import directly from `app/src/ipc/generated/media.probe`.

- [ ] **Step 3: Verify TS compiles**

  Run: `npm --prefix app run typecheck`
  Expected: exit 0.

- [ ] **Step 4: Commit (skip if no change required)**

  ```bash
  git add app/src/ipc/
  git commit -m "[item 9] Re-export MediaProbe types alongside other generated IPC types"
  ```

  If no file changed in step 2, skip the commit entirely. plan-to-pr should treat a no-op item as completed.

### Task 1.10: Verify the codegen-clean assertion in CI

**Files:** (no file change — verify only)

- [ ] **Step 1: Re-run codegen and check for drift**

  Run: `npm run gen:schemas && git status --porcelain`
  Expected: empty output (no drift).

- [ ] **Step 2: If output is non-empty, stage and commit the drift**

  ```bash
  git add app/src/ipc/generated sidecar/internal/ipc/generated app/src-tauri/src/ipc/generated.rs
  git commit -m "[item 10] Stabilise generated bindings after media.probe addition"
  ```

  If output was empty, mark item 10 complete with no commit.

### Task 1.11: Run the full PR 1 test sweep

**Files:** (no file change — verify only)

- [ ] **Step 1: Run all Rust tests**

  Run: `cd app/src-tauri && cargo test`
  Expected: all pass.

- [ ] **Step 2: Run all sidecar tests**

  Run: `cd sidecar && go test ./...`
  Expected: all pass.

- [ ] **Step 3: Run all frontend tests**

  Run: `npm --prefix app run test -- --run`
  Expected: all pass.

- [ ] **Step 4: No commit needed**

  PR 1 is complete and ready for review.

---

## PR 2: Build-time ffprobe bundling + supervisor wire-up

**Branch:** `feature/phase-3-pr2`
**Base:** `feature/phase-3-pr1`
**Depends on:** PR 1 (uses the `FFPROBE_MISSING` error code constant)
**Items:** 12–21
**Goal:** Pin BtbN/FFmpeg-Builds LGPL 7.x ffprobe binaries with SHA-256, download + verify them on `npm run setup` and in CI, place them next to the sidecar in the bundle, and have the Rust supervisor export `STUDIO_FFPROBE_PATH` to the child. Add the LGPL license text, ADR, and troubleshooting doc. After this PR the bundled ffprobe is available to the sidecar (via env), but the sidecar still doesn't invoke it.

### Task 2.1: Add the ffprobe lock file

**Files:**
- Create: `third_party/ffprobe.lock.json`

- [ ] **Step 1: Write the lock file**

  Use BtbN/FFmpeg-Builds LGPL 7.1 release artifacts (pick the latest 7.x available at planning time). The file shape is:

  ```json
  {
    "version": "7.1",
    "source": "https://github.com/BtbN/FFmpeg-Builds/releases",
    "platforms": {
      "windows-amd64": {
        "url": "https://github.com/BtbN/FFmpeg-Builds/releases/download/latest/ffmpeg-n7.1-latest-win64-lgpl-7.1.zip",
        "sha256": "<FILL_AT_IMPLEMENTATION_TIME>",
        "innerPath": "ffmpeg-n7.1-latest-win64-lgpl-7.1/bin/ffprobe.exe"
      },
      "macos-amd64": {
        "url": "https://github.com/BtbN/FFmpeg-Builds/releases/download/latest/ffmpeg-n7.1-latest-macos64-lgpl-7.1.tar.xz",
        "sha256": "<FILL_AT_IMPLEMENTATION_TIME>",
        "innerPath": "ffmpeg-n7.1-latest-macos64-lgpl-7.1/bin/ffprobe"
      },
      "macos-arm64": {
        "url": "https://github.com/BtbN/FFmpeg-Builds/releases/download/latest/ffmpeg-n7.1-latest-macosarm64-lgpl-7.1.tar.xz",
        "sha256": "<FILL_AT_IMPLEMENTATION_TIME>",
        "innerPath": "ffmpeg-n7.1-latest-macosarm64-lgpl-7.1/bin/ffprobe"
      }
    }
  }
  ```

  Implementer step: download each archive once locally with `curl -L -o tmp.bin <url>`, compute `sha256sum tmp.bin` (or `shasum -a 256 tmp.bin` on macOS), and replace each `<FILL_AT_IMPLEMENTATION_TIME>` with the actual hash. **Do not commit the placeholder.**

  BtbN does not provide a stable Apple-Silicon archive in every release. If `macos-arm64` is unavailable for the chosen version, halt and ask the user whether to pin a different version that does have arm64, or to skip the arm64 target for Phase 3 and add a follow-up.

- [ ] **Step 2: Verify all three sha256 fields are 64-hex strings**

  Run: `node -e "const j=require('./third_party/ffprobe.lock.json'); for(const k of Object.keys(j.platforms)){const h=j.platforms[k].sha256; if(!/^[0-9a-f]{64}$/.test(h)) {console.error('bad sha for '+k+': '+h); process.exit(1)}}"`
  Expected: exit 0.

- [ ] **Step 3: Commit**

  ```bash
  git add third_party/ffprobe.lock.json
  git commit -m "[item 12] Pin BtbN/FFmpeg-Builds LGPL 7.x ffprobe with SHA-256 per platform"
  ```

### Task 2.2: Commit the LGPL license text

**Files:**
- Create: `third_party/LICENSE.ffmpeg-lgpl.txt`

- [ ] **Step 1: Fetch the canonical LGPL 2.1 text**

  Run: `curl -fsSL https://www.gnu.org/licenses/lgpl-2.1.txt -o third_party/LICENSE.ffmpeg-lgpl.txt`
  Expected: file exists, ~26 KB, starts with `GNU LESSER GENERAL PUBLIC LICENSE`.

- [ ] **Step 2: Prepend a one-line attribution header**

  Edit `third_party/LICENSE.ffmpeg-lgpl.txt` to prepend (and one blank line):

  ```text
  FFmpeg / ffprobe is bundled under the LGPL v2.1 (or later). Source: https://github.com/BtbN/FFmpeg-Builds (LGPL build).

  ```

- [ ] **Step 3: Commit**

  ```bash
  git add third_party/LICENSE.ffmpeg-lgpl.txt
  git commit -m "[item 13] Bundle FFmpeg LGPL license text"
  ```

### Task 2.3: Write the `scripts/fetch-ffprobe.mjs` downloader

**Files:**
- Create: `scripts/fetch-ffprobe.mjs`

- [ ] **Step 1: Write the script**

  ```js
  #!/usr/bin/env node
  // Downloads, verifies, and extracts the bundled ffprobe per third_party/ffprobe.lock.json.
  // Outputs to app/src-tauri/binaries/ffprobe-<target-triple>{,.exe}.

  import { createHash } from 'node:crypto';
  import { existsSync, mkdirSync, readFileSync, writeFileSync, statSync, chmodSync, createReadStream } from 'node:fs';
  import { dirname, join, resolve } from 'node:path';
  import { fileURLToPath } from 'node:url';
  import { spawnSync } from 'node:child_process';
  import { tmpdir, homedir } from 'node:os';
  import { Readable } from 'node:stream';

  const rootDir  = resolve(dirname(fileURLToPath(import.meta.url)), '..');
  const lockPath = join(rootDir, 'third_party/ffprobe.lock.json');
  const lock     = JSON.parse(readFileSync(lockPath, 'utf8'));
  const outDir   = join(rootDir, 'app/src-tauri/binaries');
  const cacheDir = join(homedir(), '.cache/studio-sound/ffprobe');
  mkdirSync(outDir, { recursive: true });
  mkdirSync(cacheDir, { recursive: true });

  const triples = {
    'windows-amd64': { triple: 'x86_64-pc-windows-msvc',  exe: 'ffprobe-x86_64-pc-windows-msvc.exe' },
    'macos-amd64':   { triple: 'x86_64-apple-darwin',     exe: 'ffprobe-x86_64-apple-darwin'        },
    'macos-arm64':   { triple: 'aarch64-apple-darwin',    exe: 'ffprobe-aarch64-apple-darwin'       },
  };

  async function sha256OfFile(path) {
    const hash = createHash('sha256');
    await new Promise((res, rej) => {
      createReadStream(path).on('data', (d) => hash.update(d)).on('end', res).on('error', rej);
    });
    return hash.digest('hex');
  }

  async function download(url, dest) {
    const r = await fetch(url, { redirect: 'follow' });
    if (!r.ok) throw new Error(`HTTP ${r.status} for ${url}`);
    const ws = (await import('node:fs')).createWriteStream(dest);
    await new Promise((res, rej) => Readable.fromWeb(r.body).pipe(ws).on('finish', res).on('error', rej));
  }

  function extractInner(archivePath, innerPath, destExe) {
    if (archivePath.endsWith('.zip')) {
      // Use Node's built-in zip via `tar`? No — fall back to unzip.
      const r = spawnSync('unzip', ['-p', archivePath, innerPath], { stdio: ['ignore', 'pipe', 'inherit'] });
      if (r.status !== 0) throw new Error(`unzip failed for ${archivePath}`);
      writeFileSync(destExe, r.stdout);
    } else if (archivePath.endsWith('.tar.xz') || archivePath.endsWith('.txz')) {
      const r = spawnSync('tar', ['-xJf', archivePath, '-O', innerPath], { stdio: ['ignore', 'pipe', 'inherit'] });
      if (r.status !== 0) throw new Error(`tar -xJ failed for ${archivePath}`);
      writeFileSync(destExe, r.stdout);
    } else {
      throw new Error(`unsupported archive format: ${archivePath}`);
    }
    if (process.platform !== 'win32') chmodSync(destExe, 0o755);
  }

  for (const [platformKey, spec] of Object.entries(lock.platforms)) {
    const t = triples[platformKey];
    if (!t) throw new Error(`unknown platform in lock: ${platformKey}`);
    const destExe = join(outDir, t.exe);

    // Skip if destination already has the expected sha256 of the extracted ffprobe.
    // We hash the archive (not the extracted binary) for cache-skip; if the
    // extracted file is missing, re-extract.
    const cachedArchive = join(cacheDir, `${spec.sha256.slice(0, 12)}-${platformKey}`);
    if (existsSync(cachedArchive)) {
      const h = await sha256OfFile(cachedArchive);
      if (h !== spec.sha256) throw new Error(`cached archive checksum mismatch for ${platformKey}: ${h} != ${spec.sha256}`);
    } else {
      console.log(`fetch-ffprobe: downloading ${platformKey} from ${spec.url}`);
      await download(spec.url, cachedArchive);
      const h = await sha256OfFile(cachedArchive);
      if (h !== spec.sha256) {
        throw new Error(`downloaded archive checksum mismatch for ${platformKey}: ${h} != ${spec.sha256}`);
      }
    }

    if (!existsSync(destExe) || statSync(destExe).size === 0) {
      console.log(`fetch-ffprobe: extracting ${t.exe} from ${cachedArchive}`);
      extractInner(cachedArchive, spec.innerPath, destExe);
    } else {
      console.log(`fetch-ffprobe: ${t.exe} already present, skipping extract`);
    }
  }

  console.log('fetch-ffprobe: done');
  ```

- [ ] **Step 2: `chmod +x` (Unix only)**

  Run: `chmod +x scripts/fetch-ffprobe.mjs`
  Expected: file is executable.

- [ ] **Step 3: Run the script locally**

  Run: `node scripts/fetch-ffprobe.mjs`
  Expected: exits 0; `app/src-tauri/binaries/ffprobe-<triple>{,.exe}` files exist for the host platform (and others if `unzip`/`tar` cover them).

- [ ] **Step 4: Commit (do NOT commit the downloaded binaries)**

  Confirm `.gitignore` already covers `app/src-tauri/binaries/*` (the sidecar build follows the same pattern). If not, add `app/src-tauri/binaries/ffprobe-*` to `.gitignore`.

  ```bash
  git add scripts/fetch-ffprobe.mjs .gitignore
  git commit -m "[item 14] Add fetch-ffprobe.mjs to download and verify bundled ffprobe per lock file"
  ```

### Task 2.4: Wire the fetch step into `package.json` setup

**Files:**
- Modify: `package.json` (scripts block)

- [ ] **Step 1: Add the script and update setup**

  In `package.json`, modify the `scripts` block:
  - Add: `"ffprobe:fetch": "node scripts/fetch-ffprobe.mjs"`
  - Modify the existing `setup` script to append `&& npm run ffprobe:fetch` immediately before `&& npm run sidecar:build` (or its current equivalent).

- [ ] **Step 2: Verify `npm run setup` does not break**

  Run: `npm run setup`
  Expected: completes successfully; ffprobe binaries appear under `app/src-tauri/binaries/`.

- [ ] **Step 3: Commit**

  ```bash
  git add package.json
  git commit -m "[item 15] npm run setup now fetches the bundled ffprobe"
  ```

### Task 2.5: Add ffprobe to the Tauri bundle's `externalBin`

**Files:**
- Modify: `app/src-tauri/tauri.conf.json:36`

- [ ] **Step 1: Update the externalBin array**

  Change:

  ```json
  "externalBin": ["binaries/studio-sidecar"]
  ```

  to:

  ```json
  "externalBin": ["binaries/studio-sidecar", "binaries/ffprobe"]
  ```

  Tauri resolves each entry as `<entry>-<target-triple>{,.exe}` at bundle time.

- [ ] **Step 2: Verify the dev build doesn't reject the config**

  Run: `cd app && npm run tauri -- info`
  Expected: exit 0; no schema errors.

- [ ] **Step 3: Commit**

  ```bash
  git add app/src-tauri/tauri.conf.json
  git commit -m "[item 16] Bundle ffprobe alongside the sidecar via externalBin"
  ```

### Task 2.6: Resolve `ffprobe_path` in `Supervisor::spawn` and set `STUDIO_FFPROBE_PATH`

**Files:**
- Modify: `app/src-tauri/src/ipc/supervisor.rs:48-138, 220-240`

- [ ] **Step 1: Write the failing test**

  Append to `app/src-tauri/src/ipc/supervisor.rs` (inside the existing `#[cfg(test)] mod tests`):

  ```rust
  #[test]
  fn bundled_ffprobe_basename_uses_compile_time_triple() {
      let name = bundled_ffprobe_basename();
      #[cfg(all(target_os = "macos", target_arch = "x86_64"))]
      assert_eq!(name, "ffprobe-x86_64-apple-darwin");
      #[cfg(all(target_os = "macos", target_arch = "aarch64"))]
      assert_eq!(name, "ffprobe-aarch64-apple-darwin");
      #[cfg(all(target_os = "windows", target_arch = "x86_64"))]
      assert_eq!(name, "ffprobe-x86_64-pc-windows-msvc.exe");
  }
  ```

- [ ] **Step 2: Run the test to verify it fails**

  Run: `cd app/src-tauri && cargo test --lib ipc::supervisor::tests::bundled_ffprobe`
  Expected: FAIL — `cannot find function bundled_ffprobe_basename`.

- [ ] **Step 3: Add the helper, extend `SpawnContext`, update `spawn` and `spawn_child`**

  At module scope in `app/src-tauri/src/ipc/supervisor.rs`, add:

  ```rust
  /// Returns the platform-specific bundled ffprobe filename matching what
  /// `tauri.conf.json`'s `externalBin` produces.
  pub(crate) fn bundled_ffprobe_basename() -> &'static str {
      #[cfg(all(target_os = "macos", target_arch = "x86_64"))]
      { "ffprobe-x86_64-apple-darwin" }
      #[cfg(all(target_os = "macos", target_arch = "aarch64"))]
      { "ffprobe-aarch64-apple-darwin" }
      #[cfg(all(target_os = "windows", target_arch = "x86_64"))]
      { "ffprobe-x86_64-pc-windows-msvc.exe" }
      #[cfg(all(target_os = "linux", target_arch = "x86_64"))]
      { "ffprobe-x86_64-unknown-linux-gnu" }
  }
  ```

  Extend `SpawnContext`:

  ```rust
  struct SpawnContext {
      app: AppHandle,
      log_path: PathBuf,
      ffprobe_path: PathBuf,
  }
  ```

  In `Supervisor::spawn`, after the `log_path` block, resolve `ffprobe_path`:

  ```rust
  let resource_dir = app.path().resource_dir().map_err(|e| IpcError::Other {
      code: "FFPROBE_MISSING".into(),
      message: format!("failed to resolve resource directory: {e}"),
      details: None,
  })?;
  let ffprobe_path = resource_dir.join("binaries").join(bundled_ffprobe_basename());
  if !ffprobe_path.exists() {
      return Err(IpcError::Other {
          code: "FFPROBE_MISSING".into(),
          message: format!("bundled ffprobe not found at {}", ffprobe_path.display()),
          details: None,
      });
  }
  ```

  Pass `ffprobe_path` into `SpawnContext { ... }`.

  In `spawn_child`, extend the command builder with the new env var:

  ```rust
  .env("STUDIO_LOG_FILE", ctx.log_path.to_string_lossy().to_string())
  .env("STUDIO_FFPROBE_PATH", ctx.ffprobe_path.to_string_lossy().to_string());
  ```

- [ ] **Step 4: Run the test to verify it passes**

  Run: `cd app/src-tauri && cargo test --lib ipc::supervisor::tests::bundled_ffprobe`
  Expected: PASS.

- [ ] **Step 5: Verify Rust builds**

  Run: `cd app/src-tauri && cargo build`
  Expected: exit 0.

- [ ] **Step 6: Commit**

  ```bash
  git add app/src-tauri/src/ipc/supervisor.rs
  git commit -m "[item 17] Supervisor resolves bundled ffprobe and exports STUDIO_FFPROBE_PATH"
  ```

### Task 2.7: Update CI to fetch ffprobe and expose `STUDIO_FFPROBE_PATH`

**Files:**
- Modify: `.github/workflows/ci.yml`

- [ ] **Step 1: Add the `Fetch ffprobe` step + cache**

  Between the `Install dependencies` step and the `Generate schemas` step in each job that runs build/test (Windows + macOS Intel + macOS Apple Silicon), add:

  ```yaml
  - name: Cache ffprobe downloads
    uses: actions/cache@v4
    with:
      path: ~/.cache/studio-sound/ffprobe
      key: ffprobe-${{ runner.os }}-${{ runner.arch }}-${{ hashFiles('third_party/ffprobe.lock.json') }}

  - name: Fetch ffprobe
    run: npm run ffprobe:fetch

  - name: Verify ffprobe artifacts
    shell: bash
    run: |
      ls -l app/src-tauri/binaries/ffprobe-* | tee /dev/stderr
      test -s app/src-tauri/binaries/ffprobe-* || { echo "no ffprobe artifact"; exit 1; }
  ```

- [ ] **Step 2: Export `STUDIO_FFPROBE_PATH` for the integration test step**

  In the existing `Integration tests (sidecar E2E)` step, prepend (per-platform):

  ```yaml
  env:
    STUDIO_FFPROBE_PATH: ${{ runner.os == 'Windows' && format('{0}\app\src-tauri\binaries\ffprobe-x86_64-pc-windows-msvc.exe', github.workspace) || (runner.arch == 'ARM64' && format('{0}/app/src-tauri/binaries/ffprobe-aarch64-apple-darwin', github.workspace) || format('{0}/app/src-tauri/binaries/ffprobe-x86_64-apple-darwin', github.workspace)) }}
  ```

  (Or any equivalent per-job env assignment; the goal is that `go test -tags=integration` finds `STUDIO_FFPROBE_PATH` pointing at the host-arch binary.)

- [ ] **Step 3: Validate the workflow syntax locally**

  Run: `npx --no-install action-validator .github/workflows/ci.yml` (or if not available, push to a draft branch and inspect CI's parse error).
  Expected: exit 0 (or successful parse).

- [ ] **Step 4: Commit**

  ```bash
  git add .github/workflows/ci.yml
  git commit -m "[item 18] CI fetches ffprobe per platform and exposes STUDIO_FFPROBE_PATH for integration tests"
  ```

### Task 2.8: Write the bundling ADR

**Files:**
- Create: `docs/adr/2026-05-16-ffprobe-bundling.md`

- [ ] **Step 1: Write the ADR**

  ```markdown
  # ADR: Bundling ffprobe (LGPL build)

  **Status:** Accepted
  **Date:** 2026-05-16

  ## Context

  Phase 3 requires running `ffprobe` locally against creator-supplied media files. We do not want the user to install ffmpeg/ffprobe globally — the app must work offline and out of the box. We also need to keep our license posture defensible.

  ## Decision

  Bundle a single `ffprobe` binary (no `ffmpeg`) from the BtbN/FFmpeg-Builds **LGPL** 7.x release line, per platform (Windows x64, macOS x64, macOS arm64). Pin by SHA-256 in `third_party/ffprobe.lock.json`. Fetch + verify on `npm run setup` and in CI.

  ## Why LGPL (not GPL)

  LGPL allows dynamic linking and bundling without forcing our application source under the GPL. Our usage is "execute as a separate process," which is even more permissive than dynamic linking.

  ## Why BtbN

  BtbN/FFmpeg-Builds is the most actively maintained pre-built FFmpeg distribution and supplies clearly separated LGPL builds. Hashes pinned per platform mean a tampered upstream cannot silently change what we bundle.

  ## Why no codesigning in Phase 3

  Phase 3 ships a development / unsigned Tauri build. macOS Gatekeeper quarantine is documented in `docs/troubleshooting.md` (`xattr -d com.apple.quarantine`). Production codesigning + notarisation belong to a later release-hardening phase.

  ## Consequences

  - First-launch on macOS may flag ffprobe under Gatekeeper. Documented workaround in `docs/troubleshooting.md`.
  - Bundle grows by ~20 MB per platform (~60 MB total across the three platforms).
  - Lock-file bumps require manual SHA recomputation and a CI test pass.
  - We must reproduce the LGPL text inside the bundle (`third_party/LICENSE.ffmpeg-lgpl.txt`) and reference it in README. An in-app license screen is deferred.
  ```

- [ ] **Step 2: Commit**

  ```bash
  git add docs/adr/2026-05-16-ffprobe-bundling.md
  git commit -m "[item 19] ADR: bundle LGPL ffprobe from BtbN, pinned by SHA-256"
  ```

### Task 2.9: Add the troubleshooting doc with the Gatekeeper workaround

**Files:**
- Create: `docs/troubleshooting.md` (or modify if it already exists)

- [ ] **Step 1: Write the section**

  ```markdown
  # Troubleshooting

  ## macOS: "ffprobe" cannot be opened because the developer cannot be verified

  Phase 3 ships an **unsigned** developer build of the bundled ffprobe. macOS's
  Gatekeeper quarantines the binary on first launch and the app surfaces an
  `FFPROBE_MISSING` or `FFPROBE_FAILURE` error.

  Workaround (one-time, per binary):

  ```sh
  xattr -d com.apple.quarantine app/src-tauri/binaries/ffprobe-*-apple-darwin
  ```

  Re-launch the app. Production builds will codesign + notarise the bundled
  binary in a later release-hardening phase; this manual workaround applies
  only to development / sideload installs.

  ## "FFPROBE_MISSING" on a clean checkout

  Run `npm run ffprobe:fetch` (or the full `npm run setup`). The fetcher reads
  `third_party/ffprobe.lock.json`, downloads + verifies the per-platform
  archive, and extracts ffprobe into `app/src-tauri/binaries/`.
  ```

- [ ] **Step 2: Commit**

  ```bash
  git add docs/troubleshooting.md
  git commit -m "[item 20] Troubleshooting doc covers Gatekeeper and FFPROBE_MISSING"
  ```

### Task 2.10: Update README to acknowledge the bundled ffprobe

**Files:**
- Modify: `README.md`

- [ ] **Step 1: Add a "Bundled binaries" section**

  Append (or insert in the appropriate place):

  ```markdown
  ## Bundled binaries

  This application bundles `ffprobe` from FFmpeg under the LGPL v2.1 (or later).
  The full license text is at [`third_party/LICENSE.ffmpeg-lgpl.txt`](third_party/LICENSE.ffmpeg-lgpl.txt).
  Source builds: <https://github.com/BtbN/FFmpeg-Builds> (LGPL releases).
  Version and SHA-256 hashes are pinned in [`third_party/ffprobe.lock.json`](third_party/ffprobe.lock.json).
  ```

- [ ] **Step 2: Commit**

  ```bash
  git add README.md
  git commit -m "[item 21] README references bundled LGPL ffprobe and lock file"
  ```

---

## PR 3: Sidecar `internal/media` subprocess layer

**Branch:** `feature/phase-3-pr3`
**Base:** `feature/phase-3-pr2`
**Items:** 22–28
**Goal:** Land the locator (env-var-based ffprobe path resolution), the cross-platform runner (process-group kill on cancel, bounded stdout, stderr tail), and a `cmd/fakeffprobe` Go test helper that the runner tests use to deterministically simulate ffprobe output / exit codes / timing.

### Task 3.1: Create `sidecar/internal/media/locator.go`

**Files:**
- Create: `sidecar/internal/media/locator.go`

- [ ] **Step 1: Write the failing test**

  Create `sidecar/internal/media/locator_test.go`:

  ```go
  package media

  import (
      "errors"
      "os"
      "path/filepath"
      "testing"
  )

  func TestResolveFFprobe_ReturnsPathWhenEnvSetAndFileExists(t *testing.T) {
      tmp := t.TempDir()
      fake := filepath.Join(tmp, "ffprobe")
      if err := os.WriteFile(fake, []byte("#!/bin/sh\n"), 0o755); err != nil {
          t.Fatal(err)
      }
      t.Setenv("STUDIO_FFPROBE_PATH", fake)
      got, err := ResolveFFprobe()
      if err != nil {
          t.Fatalf("unexpected err: %v", err)
      }
      if got != fake {
          t.Errorf("got %q, want %q", got, fake)
      }
  }

  func TestResolveFFprobe_ReturnsErrWhenEnvUnset(t *testing.T) {
      t.Setenv("STUDIO_FFPROBE_PATH", "")
      _, err := ResolveFFprobe()
      if !errors.Is(err, ErrFFprobeMissing) {
          t.Errorf("got %v, want ErrFFprobeMissing", err)
      }
  }

  func TestResolveFFprobe_ReturnsErrWhenFileMissing(t *testing.T) {
      t.Setenv("STUDIO_FFPROBE_PATH", "/definitely/not/a/real/path/ffprobe")
      _, err := ResolveFFprobe()
      if !errors.Is(err, ErrFFprobeMissing) {
          t.Errorf("got %v, want ErrFFprobeMissing", err)
      }
  }

  func TestResolveFFprobe_DoesNotConsultPATH(t *testing.T) {
      // Even if `ffprobe` is on PATH, we never consult it.
      t.Setenv("STUDIO_FFPROBE_PATH", "")
      _, err := ResolveFFprobe()
      if !errors.Is(err, ErrFFprobeMissing) {
          t.Errorf("got %v, want ErrFFprobeMissing", err)
      }
  }
  ```

- [ ] **Step 2: Run the test to verify it fails**

  Run: `cd sidecar && go test ./internal/media -run TestResolveFFprobe`
  Expected: FAIL — `no Go files` (the package doesn't exist yet).

- [ ] **Step 3: Write the implementation**

  Create `sidecar/internal/media/locator.go`:

  ```go
  package media

  import (
      "errors"
      "os"
  )

  // ErrFFprobeMissing is returned by ResolveFFprobe when the bundled ffprobe
  // binary cannot be located via the STUDIO_FFPROBE_PATH env var.
  var ErrFFprobeMissing = errors.New("ffprobe binary not found")

  // ResolveFFprobe reads STUDIO_FFPROBE_PATH and verifies the file exists.
  // It never consults $PATH — we only ever run the binary that the supervisor
  // bundled and explicitly passed to us.
  func ResolveFFprobe() (string, error) {
      path := os.Getenv("STUDIO_FFPROBE_PATH")
      if path == "" {
          return "", ErrFFprobeMissing
      }
      if _, err := os.Stat(path); err != nil {
          return "", ErrFFprobeMissing
      }
      return path, nil
  }
  ```

- [ ] **Step 4: Run the test to verify it passes**

  Run: `cd sidecar && go test ./internal/media -run TestResolveFFprobe`
  Expected: PASS (4 cases).

- [ ] **Step 5: Commit**

  ```bash
  git add sidecar/internal/media/locator.go sidecar/internal/media/locator_test.go
  git commit -m "[item 22] Locator: ResolveFFprobe via STUDIO_FFPROBE_PATH only"
  ```

### Task 3.2: Create the `cmd/fakeffprobe` test helper

**Files:**
- Create: `sidecar/cmd/fakeffprobe/main.go`

- [ ] **Step 1: Write the helper**

  ```go
  // Package main is a deterministic test-only stand-in for the real ffprobe
  // binary. The runner tests under sidecar/internal/media use it to control
  // stdout / stderr / exit code / runtime via env vars.
  //
  // Env vars:
  //   FAKE_FFPROBE_STDOUT       — written to stdout (raw bytes)
  //   FAKE_FFPROBE_STDOUT_BYTES — number of zero bytes to dump (for oversize tests)
  //   FAKE_FFPROBE_STDERR       — written to stderr
  //   FAKE_FFPROBE_EXIT         — integer exit code (default 0)
  //   FAKE_FFPROBE_SLEEP_MS     — sleep before exit (for deadline tests)
  package main

  import (
      "fmt"
      "os"
      "strconv"
      "time"
  )

  func main() {
      if s := os.Getenv("FAKE_FFPROBE_STDOUT"); s != "" {
          fmt.Fprint(os.Stdout, s)
      }
      if n, _ := strconv.Atoi(os.Getenv("FAKE_FFPROBE_STDOUT_BYTES")); n > 0 {
          buf := make([]byte, 4096)
          for written := 0; written < n; {
              chunk := buf
              if rem := n - written; rem < len(chunk) {
                  chunk = chunk[:rem]
              }
              k, err := os.Stdout.Write(chunk)
              if err != nil {
                  break
              }
              written += k
          }
      }
      if s := os.Getenv("FAKE_FFPROBE_STDERR"); s != "" {
          fmt.Fprint(os.Stderr, s)
      }
      if ms, _ := strconv.Atoi(os.Getenv("FAKE_FFPROBE_SLEEP_MS")); ms > 0 {
          time.Sleep(time.Duration(ms) * time.Millisecond)
      }
      exit, _ := strconv.Atoi(os.Getenv("FAKE_FFPROBE_EXIT"))
      os.Exit(exit)
  }
  ```

- [ ] **Step 2: Verify it builds**

  Run: `cd sidecar && go build ./cmd/fakeffprobe`
  Expected: exit 0.

- [ ] **Step 3: Commit**

  ```bash
  git add sidecar/cmd/fakeffprobe/main.go
  git commit -m "[item 23] Add fakeffprobe Go test helper for runner unit tests"
  ```

### Task 3.3: Add `killGroup` Unix variant

**Files:**
- Create: `sidecar/internal/media/runner_unix.go`

- [ ] **Step 1: Write the file**

  ```go
  //go:build !windows

  package media

  import (
      "os/exec"
      "syscall"
  )

  func setProcAttrs(cmd *exec.Cmd) {
      cmd.SysProcAttr = &syscall.SysProcAttr{Setpgid: true}
  }

  func killGroup(cmd *exec.Cmd) error {
      if cmd.Process == nil {
          return nil
      }
      // Negative pid kills the whole process group.
      return syscall.Kill(-cmd.Process.Pid, syscall.SIGKILL)
  }
  ```

- [ ] **Step 2: Verify it builds on Unix**

  Run: `cd sidecar && go build ./internal/media`
  Expected: exit 0.

- [ ] **Step 3: Commit**

  ```bash
  git add sidecar/internal/media/runner_unix.go
  git commit -m "[item 24] Runner: Unix process-group setup + SIGKILL helper"
  ```

### Task 3.4: Add `killGroup` Windows variant

**Files:**
- Create: `sidecar/internal/media/runner_windows.go`

- [ ] **Step 1: Write the file**

  ```go
  //go:build windows

  package media

  import (
      "fmt"
      "os/exec"

      "golang.org/x/sys/windows"
  )

  func setProcAttrs(cmd *exec.Cmd) {
      cmd.SysProcAttr = &windows.SysProcAttr{CreationFlags: windows.CREATE_NEW_PROCESS_GROUP}
  }

  func killGroup(cmd *exec.Cmd) error {
      if cmd.Process == nil {
          return nil
      }
      // taskkill /T /F /PID <pid> kills the process and its descendants.
      kill := exec.Command("taskkill", "/T", "/F", "/PID", fmt.Sprint(cmd.Process.Pid))
      return kill.Run()
  }
  ```

- [ ] **Step 2: Ensure `golang.org/x/sys` is in `go.mod`**

  Run: `cd sidecar && go get golang.org/x/sys/windows && go mod tidy`
  Expected: exit 0.

- [ ] **Step 3: Verify it builds (cross-compile for windows)**

  Run: `cd sidecar && GOOS=windows GOARCH=amd64 go build ./internal/media`
  Expected: exit 0.

- [ ] **Step 4: Commit**

  ```bash
  git add sidecar/internal/media/runner_windows.go sidecar/go.mod sidecar/go.sum
  git commit -m "[item 25] Runner: Windows CREATE_NEW_PROCESS_GROUP + taskkill helper"
  ```

### Task 3.5: Write the cross-platform `runner.go`

**Files:**
- Create: `sidecar/internal/media/runner.go`

- [ ] **Step 1: Write the failing tests**

  Create `sidecar/internal/media/runner_test.go`:

  ```go
  package media

  import (
      "context"
      "os"
      "os/exec"
      "path/filepath"
      "runtime"
      "strconv"
      "strings"
      "testing"
      "time"
  )

  // buildFake builds the fakeffprobe helper into a temp dir and returns its path.
  func buildFake(t *testing.T) string {
      t.Helper()
      tmp := t.TempDir()
      out := filepath.Join(tmp, "fakeffprobe")
      if runtime.GOOS == "windows" {
          out += ".exe"
      }
      cmd := exec.Command("go", "build", "-o", out, "../../cmd/fakeffprobe")
      cmd.Stderr = os.Stderr
      if err := cmd.Run(); err != nil {
          t.Fatalf("failed to build fakeffprobe: %v", err)
      }
      return out
  }

  func TestRun_SuccessReturnsStdout(t *testing.T) {
      fake := buildFake(t)
      t.Setenv("FAKE_FFPROBE_STDOUT", `{"ok":true}`)
      ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
      defer cancel()
      r, err := Run(ctx, fake, "ignored.mp4")
      if err != nil {
          t.Fatalf("unexpected err: %v", err)
      }
      if string(r.Stdout) != `{"ok":true}` {
          t.Errorf("got stdout %q", string(r.Stdout))
      }
      if r.ExitCode != 0 {
          t.Errorf("got exit %d, want 0", r.ExitCode)
      }
  }

  func TestRun_NonZeroExitPreservesStderrTail(t *testing.T) {
      fake := buildFake(t)
      t.Setenv("FAKE_FFPROBE_STDERR", "Invalid data found when processing input")
      t.Setenv("FAKE_FFPROBE_EXIT", "1")
      ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
      defer cancel()
      r, err := Run(ctx, fake, "ignored.mp4")
      if err == nil {
          t.Fatal("expected non-nil err on nonzero exit")
      }
      if r == nil || r.ExitCode != 1 {
          t.Errorf("got exit %v, want 1", r)
      }
      if !strings.Contains(r.StderrTail, "Invalid data") {
          t.Errorf("got stderr tail %q", r.StderrTail)
      }
  }

  func TestRun_OversizeStdoutTruncatedToCap(t *testing.T) {
      fake := buildFake(t)
      t.Setenv("FAKE_FFPROBE_STDOUT_BYTES", strconv.Itoa(2*1024*1024)) // 2 MiB
      ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
      defer cancel()
      r, err := Run(ctx, fake, "ignored.mp4")
      if err != nil {
          t.Fatalf("unexpected err: %v", err)
      }
      if len(r.Stdout) != stdoutCap {
          t.Errorf("got stdout len %d, want %d", len(r.Stdout), stdoutCap)
      }
  }

  func TestRun_DeadlineKills(t *testing.T) {
      fake := buildFake(t)
      t.Setenv("FAKE_FFPROBE_SLEEP_MS", "10000") // 10s
      ctx, cancel := context.WithTimeout(context.Background(), 200*time.Millisecond)
      defer cancel()
      start := time.Now()
      _, err := Run(ctx, fake, "ignored.mp4")
      elapsed := time.Since(start)
      if err == nil {
          t.Fatal("expected err on deadline")
      }
      if elapsed > 2*time.Second {
          t.Errorf("did not kill quickly enough: %v", elapsed)
      }
  }

  func TestRun_StderrTailClippedTo4KiB(t *testing.T) {
      fake := buildFake(t)
      big := strings.Repeat("x", 8*1024)
      t.Setenv("FAKE_FFPROBE_STDERR", big)
      t.Setenv("FAKE_FFPROBE_EXIT", "1")
      ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
      defer cancel()
      r, _ := Run(ctx, fake, "ignored.mp4")
      if r == nil || len(r.StderrTail) > stderrTailCap {
          t.Errorf("got stderr tail len %d, cap %d", len(r.StderrTail), stderrTailCap)
      }
  }
  ```

- [ ] **Step 2: Run the tests to verify they fail**

  Run: `cd sidecar && go test ./internal/media -run TestRun`
  Expected: FAIL — `undefined: Run` (and `undefined: stdoutCap`, `stderrTailCap`).

- [ ] **Step 3: Write the implementation**

  Create `sidecar/internal/media/runner.go`:

  ```go
  package media

  import (
      "bytes"
      "context"
      "errors"
      "io"
      "os/exec"
  )

  const (
      stdoutCap     = 1 * 1024 * 1024
      stderrTailCap = 4 * 1024
  )

  type RunResult struct {
      Stdout     []byte
      ExitCode   int
      StderrTail string
  }

  // Run invokes ffprobe with the canonical Phase 3 arg set against mediaPath.
  // Honours ctx cancellation by killing the ffprobe process group.
  func Run(ctx context.Context, ffprobePath, mediaPath string) (*RunResult, error) {
      cmd := exec.CommandContext(ctx, ffprobePath,
          "-v", "error",
          "-hide_banner",
          "-print_format", "json",
          "-show_format",
          "-show_streams",
          mediaPath,
      )
      setProcAttrs(cmd)
      // exec.CommandContext defaults to os.Process.Kill on cancel, which on
      // Unix does NOT kill children. Override Cancel to kill the whole group.
      cmd.Cancel = func() error { return killGroup(cmd) }

      var stdoutBuf cappedBuffer
      stdoutBuf.cap = stdoutCap
      var stderrTail tailBuffer
      stderrTail.cap = stderrTailCap

      cmd.Stdout = &stdoutBuf
      cmd.Stderr = &stderrTail

      err := cmd.Run()
      res := &RunResult{
          Stdout:     stdoutBuf.Bytes(),
          ExitCode:   exitCodeFromErr(err, cmd),
          StderrTail: stderrTail.String(),
      }
      if err != nil {
          return res, err
      }
      return res, nil
  }

  func exitCodeFromErr(err error, cmd *exec.Cmd) int {
      if err == nil {
          if cmd.ProcessState != nil {
              return cmd.ProcessState.ExitCode()
          }
          return 0
      }
      var ee *exec.ExitError
      if errors.As(err, &ee) {
          return ee.ExitCode()
      }
      return -1
  }

  type cappedBuffer struct {
      buf bytes.Buffer
      cap int
  }

  func (c *cappedBuffer) Write(p []byte) (int, error) {
      remaining := c.cap - c.buf.Len()
      if remaining <= 0 {
          return len(p), nil
      }
      if len(p) > remaining {
          p = p[:remaining]
      }
      return c.buf.Write(p)
  }

  func (c *cappedBuffer) Bytes() []byte { return c.buf.Bytes() }

  // tailBuffer accumulates bytes but keeps only the last `cap` bytes written.
  type tailBuffer struct {
      buf bytes.Buffer
      cap int
  }

  func (t *tailBuffer) Write(p []byte) (int, error) {
      n, err := t.buf.Write(p)
      if t.buf.Len() > t.cap {
          excess := t.buf.Len() - t.cap
          // Discard the leading `excess` bytes.
          _, _ = io.CopyN(io.Discard, &t.buf, int64(excess))
      }
      return n, err
  }

  func (t *tailBuffer) String() string { return t.buf.String() }
  ```

- [ ] **Step 4: Run tests to verify they pass**

  Run: `cd sidecar && go test ./internal/media -run TestRun`
  Expected: PASS (5 cases).

- [ ] **Step 5: Commit**

  ```bash
  git add sidecar/internal/media/runner.go sidecar/internal/media/runner_test.go
  git commit -m "[item 26] Runner: cross-platform ffprobe invocation with bounded stdout + stderr tail"
  ```

### Task 3.6: Add a "long Windows path prefix" helper

**Files:**
- Modify: `sidecar/internal/media/runner.go`
- Modify: `sidecar/internal/media/runner_test.go`

- [ ] **Step 1: Write the failing test**

  Add to `runner_test.go`:

  ```go
  func TestLongPathPrefix_Windows(t *testing.T) {
      if runtime.GOOS != "windows" {
          t.Skip("windows-only")
      }
      short := `C:\foo\bar.mp4`
      if got := maybePrefixLongPath(short); got != short {
          t.Errorf("short path was prefixed: %q", got)
      }
      long := `C:\` + strings.Repeat("a", 300) + `.mp4`
      got := maybePrefixLongPath(long)
      if !strings.HasPrefix(got, `\\?\`) {
          t.Errorf("long path was not prefixed: %q", got)
      }
  }

  func TestLongPathPrefix_Unix(t *testing.T) {
      if runtime.GOOS == "windows" {
          t.Skip("non-windows-only")
      }
      p := "/tmp/" + strings.Repeat("a", 300) + ".mp4"
      if got := maybePrefixLongPath(p); got != p {
          t.Errorf("unix path was modified: %q", got)
      }
  }
  ```

- [ ] **Step 2: Run the test to verify it fails**

  Run: `cd sidecar && go test ./internal/media -run TestLongPathPrefix`
  Expected: FAIL — `undefined: maybePrefixLongPath`.

- [ ] **Step 3: Implement the helper**

  Add to the bottom of `runner.go`:

  ```go
  // maybePrefixLongPath prepends \\?\ on Windows when the path is long enough
  // to risk crossing the legacy MAX_PATH (260) boundary. No-op on non-Windows.
  func maybePrefixLongPath(p string) string { return maybeLongPath(p) }
  ```

  Add to `runner_unix.go`:

  ```go
  func maybeLongPath(p string) string { return p }
  ```

  Add to `runner_windows.go`:

  ```go
  import "strings"

  func maybeLongPath(p string) string {
      if len(p) <= 240 {
          return p
      }
      if strings.HasPrefix(p, `\\?\`) {
          return p
      }
      return `\\?\` + p
  }
  ```

  In `Run`, replace the `mediaPath` arg in the exec.CommandContext call with `maybePrefixLongPath(mediaPath)`.

- [ ] **Step 4: Run tests to verify they pass**

  Run: `cd sidecar && go test ./internal/media -run TestLongPathPrefix`
  Expected: PASS.

  And: `cd sidecar && GOOS=windows GOARCH=amd64 go build ./internal/media`
  Expected: exit 0.

- [ ] **Step 5: Commit**

  ```bash
  git add sidecar/internal/media/runner.go sidecar/internal/media/runner_unix.go sidecar/internal/media/runner_windows.go sidecar/internal/media/runner_test.go
  git commit -m "[item 27] Runner: prefix long Windows paths with \\\\?\\ for MAX_PATH"
  ```

### Task 3.7: Final PR 3 test sweep

**Files:** (no file change — verify only)

- [ ] **Step 1: Run all sidecar tests**

  Run: `cd sidecar && go test ./...`
  Expected: all pass.

- [ ] **Step 2: Cross-compile sanity**

  Run: `cd sidecar && GOOS=windows GOARCH=amd64 go build ./... && GOOS=darwin GOARCH=arm64 go build ./...`
  Expected: exit 0 for both.

- [ ] **Step 3: No commit needed — item 28 complete**

  PR 3 is ready for review.

---

## PR 4: Sidecar `internal/media` semantic layer

**Branch:** `feature/phase-3-pr4`
**Base:** `feature/phase-3-pr3`
**Items:** 29–38
**Goal:** Parse ffprobe JSON, normalise it into the canonical `MediaProbeResult` shape (always populates `audio` as object-or-null), evaluate compatibility (hybrid model: unsupported = `supported=false` + descriptive issues; only file-system/parse/runner errors become RPCs), and tie it all together in `media.Probe`. All tests use captured JSON fixtures under `sidecar/internal/media/testdata/`.

### Task 4.1: Add fixture ffprobe JSON files under `testdata/`

**Files:**
- Create: `sidecar/internal/media/testdata/h264_aac_stereo.json`
- Create: `sidecar/internal/media/testdata/vp9_opus.json`
- Create: `sidecar/internal/media/testdata/aac_multitrack.json`
- Create: `sidecar/internal/media/testdata/no_audio.json`
- Create: `sidecar/internal/media/testdata/missing_duration.json`
- Create: `sidecar/internal/media/testdata/unsupported_container.json`
- Create: `sidecar/internal/media/testdata/unsupported_codec.json`

- [ ] **Step 1: Write hand-crafted ffprobe JSON for each fixture**

  Each file is the exact JSON that `ffprobe -v error -hide_banner -print_format json -show_format -show_streams <fixture>` would emit. Implementer: generate the real JSON by running ffprobe against the matching binary in `test-assets/` (committed in PR 5) once they exist locally, OR hand-craft the minimum representative JSON now. Required fields:

  - `h264_aac_stereo.json`: `format.format_name="mov,mp4,m4a,3gp,3g2,mj2"`, `format.duration="5.0"`, one `codec_type=video` stream (h264 1280×720 r_frame_rate `30/1`), one `codec_type=audio` stream (aac, 2 channels, sample_rate `48000`, channel_layout `stereo`, `disposition.default=1`).
  - `vp9_opus.json`: `format.format_name="matroska,webm"`, vp9 1920×1080 video, opus stereo audio.
  - `aac_multitrack.json`: `format.format_name="mov,mp4,m4a,3gp,3g2,mj2"`, h264 video, two aac audio streams: index 1 `disposition.default=1` with `tags.title="Microphone"`, index 2 `disposition.default=0` with `tags.title="Game Audio"`.
  - `no_audio.json`: `format.format_name="mov,mp4,m4a,3gp,3g2,mj2"`, only one h264 video stream.
  - `missing_duration.json`: like h264_aac_stereo but with `format.duration` removed.
  - `unsupported_container.json`: `format.format_name="asf"` (WMV), wmv2 video, wmav2 audio.
  - `unsupported_codec.json`: `format.format_name="mov,mp4,m4a,3gp,3g2,mj2"`, h264 video, ac3 audio.

- [ ] **Step 2: Verify each file is valid JSON**

  Run: `for f in sidecar/internal/media/testdata/*.json; do jq . "$f" > /dev/null || { echo "invalid: $f"; exit 1; }; done`
  Expected: exit 0.

- [ ] **Step 3: Commit**

  ```bash
  git add sidecar/internal/media/testdata/
  git commit -m "[item 29] Add hand-crafted ffprobe JSON test fixtures for parser/normalize/compat"
  ```

### Task 4.2: Create `sidecar/internal/media/parser.go`

**Files:**
- Create: `sidecar/internal/media/parser.go`

- [ ] **Step 1: Write the failing test**

  Create `sidecar/internal/media/parser_test.go`:

  ```go
  package media

  import (
      "os"
      "path/filepath"
      "testing"
  )

  func readFixture(t *testing.T, name string) []byte {
      t.Helper()
      b, err := os.ReadFile(filepath.Join("testdata", name))
      if err != nil { t.Fatal(err) }
      return b
  }

  func TestParse_H264AAC(t *testing.T) {
      out, err := Parse(readFixture(t, "h264_aac_stereo.json"))
      if err != nil { t.Fatal(err) }
      if out.Format.FormatName == "" { t.Error("missing format_name") }
      if len(out.Streams) < 2 { t.Errorf("got %d streams, want >=2", len(out.Streams)) }
  }

  func TestParse_AcceptsUnknownFields(t *testing.T) {
      // Tolerate ffprobe version skew.
      _, err := Parse([]byte(`{"format":{"format_name":"x","new_field":"x"},"streams":[]}`))
      if err != nil { t.Errorf("unexpected err on unknown field: %v", err) }
  }

  func TestParse_ReturnsErrOnInvalidJSON(t *testing.T) {
      _, err := Parse([]byte(`not json`))
      if err == nil { t.Error("expected err on invalid JSON") }
  }
  ```

- [ ] **Step 2: Run the test to verify it fails**

  Run: `cd sidecar && go test ./internal/media -run TestParse`
  Expected: FAIL — `undefined: Parse`.

- [ ] **Step 3: Write the implementation**

  Create `sidecar/internal/media/parser.go`:

  ```go
  package media

  import (
      "encoding/json"
      "fmt"
  )

  type ffprobeOutput struct {
      Format  ffprobeFormat   `json:"format"`
      Streams []ffprobeStream `json:"streams"`
  }

  type ffprobeFormat struct {
      FormatName     string `json:"format_name"`
      FormatLongName string `json:"format_long_name"`
      Duration       string `json:"duration"`
      Size           string `json:"size"`
      BitRate        string `json:"bit_rate"`
  }

  type ffprobeStream struct {
      Index         int               `json:"index"`
      CodecType     string            `json:"codec_type"`
      CodecName     string            `json:"codec_name"`
      Width         int               `json:"width,omitempty"`
      Height        int               `json:"height,omitempty"`
      PixFmt        string            `json:"pix_fmt,omitempty"`
      RFrameRate    string            `json:"r_frame_rate,omitempty"`
      Channels      int               `json:"channels,omitempty"`
      SampleRate    string            `json:"sample_rate,omitempty"`
      ChannelLayout string            `json:"channel_layout,omitempty"`
      BitRate       string            `json:"bit_rate,omitempty"`
      Disposition   map[string]int    `json:"disposition,omitempty"`
      Tags          map[string]string `json:"tags,omitempty"`
  }

  func Parse(stdout []byte) (*ffprobeOutput, error) {
      var out ffprobeOutput
      if err := json.Unmarshal(stdout, &out); err != nil {
          return nil, fmt.Errorf("parse ffprobe json: %w", err)
      }
      return &out, nil
  }
  ```

- [ ] **Step 4: Run the test to verify it passes**

  Run: `cd sidecar && go test ./internal/media -run TestParse`
  Expected: PASS (3 cases).

- [ ] **Step 5: Commit**

  ```bash
  git add sidecar/internal/media/parser.go sidecar/internal/media/parser_test.go
  git commit -m "[item 30] Parser: decode ffprobe JSON tolerating unknown fields"
  ```

### Task 4.3: Add an exported `MediaProbeResult` type alias backed by the generated schema

**Files:**
- Create: `sidecar/internal/media/types.go`

- [ ] **Step 1: Write the file**

  ```go
  package media

  import "github.com/example/studio-sound-app/sidecar/internal/ipc/generated"

  // MediaProbeResult is the canonical wire-stable shape, identical to the
  // generated ProbeResult from schemas/media.probe.schema.json. Re-exported so
  // callers within the media package can use a stable name.
  type MediaProbeResult = generated.ProbeResult
  type AudioStream     = generated.Audio
  type AudioTrack      = generated.AudioTrack
  type VideoStream     = generated.Video
  type Container       = generated.Container
  type Compatibility   = generated.Compatibility
  ```

  Replace `github.com/example/studio-sound-app/sidecar/...` with the actual module path from `sidecar/go.mod`. Run `grep '^module' sidecar/go.mod` to find it.

- [ ] **Step 2: Verify it compiles**

  Run: `cd sidecar && go build ./internal/media`
  Expected: exit 0.

- [ ] **Step 3: Commit**

  ```bash
  git add sidecar/internal/media/types.go
  git commit -m "[item 31] Re-export generated MediaProbeResult types under internal/media"
  ```

### Task 4.4: Create `sidecar/internal/media/normalize.go`

**Files:**
- Create: `sidecar/internal/media/normalize.go`

- [ ] **Step 1: Write the failing test**

  Create `sidecar/internal/media/normalize_test.go`:

  ```go
  package media

  import "testing"

  func TestNormalize_PopulatesVideoAndAudio(t *testing.T) {
      out, _ := Parse(readFixture(t, "h264_aac_stereo.json"))
      r, err := Normalize(out, "/tmp/x.mp4", 12345)
      if err != nil { t.Fatal(err) }
      if r.Filename != "x.mp4" { t.Errorf("filename = %q", r.Filename) }
      if r.SizeBytes != 12345 { t.Errorf("sizeBytes = %d", r.SizeBytes) }
      if r.Video == nil || r.Video.Codec != "h264" { t.Errorf("video = %+v", r.Video) }
      if r.Audio == nil || r.Audio.Codec != "aac" || r.Audio.Channels != 2 {
          t.Errorf("audio = %+v", r.Audio)
      }
      if r.DurationSeconds == nil || *r.DurationSeconds <= 0 {
          t.Errorf("durationSeconds = %v", r.DurationSeconds)
      }
  }

  func TestNormalize_AudioIsNilWhenNoAudioStream(t *testing.T) {
      out, _ := Parse(readFixture(t, "no_audio.json"))
      r, err := Normalize(out, "/tmp/x.mp4", 1)
      if err != nil { t.Fatal(err) }
      if r.Audio != nil {
          t.Errorf("audio should be nil for no-audio file, got %+v", r.Audio)
      }
  }

  func TestNormalize_DefaultTrackSelectionByDispositionFlag(t *testing.T) {
      out, _ := Parse(readFixture(t, "aac_multitrack.json"))
      r, _ := Normalize(out, "/tmp/x.mov", 1)
      if r.Audio == nil { t.Fatal("audio nil") }
      // index 1 is default-flagged in the fixture
      if r.Audio.TrackIndex != 1 { t.Errorf("trackIndex = %d, want 1", r.Audio.TrackIndex) }
      if r.Audio.TrackCount != 2 { t.Errorf("trackCount = %d, want 2", r.Audio.TrackCount) }
      if len(r.Audio.Tracks) != 2 { t.Errorf("tracks len = %d", len(r.Audio.Tracks)) }
  }

  func TestNormalize_DurationOmittedWhenMissing(t *testing.T) {
      out, _ := Parse(readFixture(t, "missing_duration.json"))
      r, _ := Normalize(out, "/tmp/x.mp4", 1)
      if r.DurationSeconds != nil {
          t.Errorf("durationSeconds should be nil, got %v", *r.DurationSeconds)
      }
  }

  func TestNormalize_FPSFromRFrameRate(t *testing.T) {
      out := &ffprobeOutput{
          Format:  ffprobeFormat{FormatName: "x"},
          Streams: []ffprobeStream{{CodecType: "video", CodecName: "h264", Width: 1920, Height: 1080, RFrameRate: "30000/1001"}},
      }
      r, _ := Normalize(out, "/tmp/x.mp4", 1)
      if r.Video == nil { t.Fatal("video nil") }
      if r.Video.Fps < 29.96 || r.Video.Fps > 29.98 {
          t.Errorf("fps = %v, want ~29.97", r.Video.Fps)
      }
  }
  ```

- [ ] **Step 2: Run the test to verify it fails**

  Run: `cd sidecar && go test ./internal/media -run TestNormalize`
  Expected: FAIL — `undefined: Normalize`.

- [ ] **Step 3: Write the implementation**

  Create `sidecar/internal/media/normalize.go`:

  ```go
  package media

  import (
      "crypto/rand"
      "encoding/hex"
      "path/filepath"
      "strconv"
      "strings"
  )

  // Normalize converts a parsed ffprobe output into the canonical MediaProbeResult.
  // Always populates Audio as a fully-populated AudioStream or leaves it nil
  // (which Encode marshals as JSON `null`). Compatibility is left zero-valued
  // for Evaluate to fill in.
  func Normalize(in *ffprobeOutput, path string, sizeBytes int64) (*MediaProbeResult, error) {
      r := &MediaProbeResult{
          ID:        newULIDLike(),
          Path:      path,
          Filename:  filepath.Base(path),
          SizeBytes: sizeBytes,
          Container: Container{
              Format:   in.Format.FormatName,
              LongName: in.Format.FormatLongName,
          },
      }

      if dur, err := strconv.ParseFloat(strings.TrimSpace(in.Format.Duration), 64); err == nil && dur > 0 {
          r.DurationSeconds = &dur
      }

      var video *ffprobeStream
      var audioStreams []ffprobeStream
      for i := range in.Streams {
          s := in.Streams[i]
          switch s.CodecType {
          case "video":
              if video == nil { video = &s }
          case "audio":
              audioStreams = append(audioStreams, s)
          }
      }

      if video != nil {
          r.Video = &VideoStream{
              Codec:       video.CodecName,
              Width:       video.Width,
              Height:      video.Height,
              Fps:         parseFps(video.RFrameRate),
              Bitrate:     optInt(video.BitRate),
              PixelFormat: stringOrEmpty(video.PixFmt),
          }
      }

      if len(audioStreams) > 0 {
          def := defaultAudioIndex(audioStreams)
          chosen := audioStreams[def]
          tracks := make([]AudioTrack, len(audioStreams))
          for i, s := range audioStreams {
              tracks[i] = AudioTrack{
                  Index:      s.Index,
                  Codec:      s.CodecName,
                  Channels:   s.Channels,
                  SampleRate: parseInt(s.SampleRate),
                  Bitrate:    optInt(s.BitRate),
                  Layout:     stringOrEmpty(s.ChannelLayout),
                  Title:      stringOrEmpty(s.Tags["title"]),
                  Language:   stringOrEmpty(s.Tags["language"]),
                  IsDefault:  s.Disposition["default"] == 1,
              }
          }
          r.Audio = &AudioStream{
              Codec:       chosen.CodecName,
              Channels:    chosen.Channels,
              SampleRate:  parseInt(chosen.SampleRate),
              Bitrate:     optInt(chosen.BitRate),
              Layout:      stringOrEmpty(chosen.ChannelLayout),
              TrackIndex:  chosen.Index,
              TrackCount:  len(audioStreams),
              Tracks:      tracks,
          }
      }

      return r, nil
  }

  func defaultAudioIndex(streams []ffprobeStream) int {
      // Prefer the first stream flagged disposition.default == 1.
      for i, s := range streams {
          if s.Disposition["default"] == 1 {
              return i
          }
      }
      // Else fall back to the stream with the lowest ffprobe index.
      lowest := 0
      for i, s := range streams {
          if s.Index < streams[lowest].Index {
              lowest = i
          }
      }
      return lowest
  }

  func parseFps(rfr string) float64 {
      parts := strings.SplitN(rfr, "/", 2)
      if len(parts) != 2 { return 0 }
      num, err1 := strconv.ParseFloat(parts[0], 64)
      den, err2 := strconv.ParseFloat(parts[1], 64)
      if err1 != nil || err2 != nil || den == 0 { return 0 }
      return num / den
  }

  func parseInt(s string) int {
      n, _ := strconv.Atoi(strings.TrimSpace(s))
      return n
  }

  func optInt(s string) *int {
      if strings.TrimSpace(s) == "" { return nil }
      if n, err := strconv.Atoi(s); err == nil {
          return &n
      }
      return nil
  }

  func stringOrEmpty(s string) string { return s }

  // newULIDLike produces a short, monotonic-ish identifier. Phase 3 has no hard
  // ULID dependency; a 26-char hex from a 13-byte random source matches the
  // length and probe-uniqueness guarantee in the LLD.
  func newULIDLike() string {
      var b [13]byte
      _, _ = rand.Read(b[:])
      return strings.ToUpper(hex.EncodeToString(b[:]))[:26]
  }
  ```

  **Note:** if the generated `ProbeResult` types use different Go field types than assumed here (e.g., `int` vs `int64`, or no pointer for `DurationSeconds`), adjust the assignments. The schema's `integer` becomes `int` by default in go-jsonschema; the schema's optional numeric fields without `null` in oneOf become `*int` or omit-when-zero — verify by inspecting `sidecar/internal/ipc/generated/media_probe.go` after PR 1's codegen, and adapt. If `Audio` in the generated type is not a pointer (e.g., it's an embedded value), use a sentinel (`AudioPresent bool`) field added via a wrapper. **If the generated shape forces a different normalize signature, halt and ask before improvising.**

- [ ] **Step 4: Run tests to verify they pass**

  Run: `cd sidecar && go test ./internal/media -run TestNormalize`
  Expected: PASS (5 cases).

- [ ] **Step 5: Commit**

  ```bash
  git add sidecar/internal/media/normalize.go sidecar/internal/media/normalize_test.go
  git commit -m "[item 32] Normalize: ffprobe JSON -> MediaProbeResult; audio is object-or-nil"
  ```

### Task 4.5: Create `sidecar/internal/media/compat.go`

**Files:**
- Create: `sidecar/internal/media/compat.go`

- [ ] **Step 1: Write the failing test**

  Create `sidecar/internal/media/compat_test.go`:

  ```go
  package media

  import "testing"

  func eval(t *testing.T, fixture string) (*MediaProbeResult, Verdict) {
      out, _ := Parse(readFixture(t, fixture))
      r, _ := Normalize(out, "/tmp/x", 1)
      v := Evaluate(r)
      return r, v
  }

  func TestEvaluate_H264AACIsSupported(t *testing.T) {
      _, v := eval(t, "h264_aac_stereo.json")
      if !v.Supported { t.Errorf("expected supported, got %+v", v) }
      if len(v.Issues) != 0 { t.Errorf("unexpected issues: %v", v.Issues) }
  }

  func TestEvaluate_VP9OpusIsSupported(t *testing.T) {
      _, v := eval(t, "vp9_opus.json")
      if !v.Supported { t.Errorf("expected supported, got %+v", v) }
  }

  func TestEvaluate_UnsupportedContainerSetsIssue(t *testing.T) {
      _, v := eval(t, "unsupported_container.json")
      if v.Supported { t.Error("expected unsupported") }
      var found bool
      for _, s := range v.Issues {
          if contains(s, "container") { found = true }
      }
      if !found { t.Errorf("no container issue in: %v", v.Issues) }
  }

  func TestEvaluate_UnsupportedCodecSetsIssue(t *testing.T) {
      _, v := eval(t, "unsupported_codec.json")
      if v.Supported { t.Error("expected unsupported") }
      var found bool
      for _, s := range v.Issues {
          if contains(s, "codec") { found = true }
      }
      if !found { t.Errorf("no codec issue in: %v", v.Issues) }
  }

  func TestEvaluate_NoAudioStreamSetsIssue(t *testing.T) {
      _, v := eval(t, "no_audio.json")
      if v.Supported { t.Error("expected unsupported") }
      var found bool
      for _, s := range v.Issues {
          if contains(s, "audio") { found = true }
      }
      if !found { t.Errorf("no audio issue in: %v", v.Issues) }
  }

  func TestEvaluate_MissingDurationIsWarningNotIssue(t *testing.T) {
      _, v := eval(t, "missing_duration.json")
      if !v.Supported { t.Errorf("missing duration must not block support") }
      var found bool
      for _, s := range v.Warnings {
          if contains(s, "duration") { found = true }
      }
      if !found { t.Errorf("missing duration should produce a warning: %v", v.Warnings) }
  }

  func TestEvaluate_MultitrackAddsInformationalWarning(t *testing.T) {
      _, v := eval(t, "aac_multitrack.json")
      if !v.Supported { t.Error("multitrack must remain supported") }
      var found bool
      for _, s := range v.Warnings {
          if contains(s, "track") { found = true }
      }
      if !found { t.Errorf("multitrack should produce a warning: %v", v.Warnings) }
  }

  func contains(haystack, needle string) bool {
      return len(haystack) >= len(needle) && (haystack == needle || (len(needle) > 0 && (haystack[0:0] == "" && (func() bool {
          for i := 0; i+len(needle) <= len(haystack); i++ {
              if haystack[i:i+len(needle)] == needle { return true }
          }
          return false
      })())))
  }
  ```

  Replace the hand-rolled `contains` with `strings.Contains` after import:

  ```go
  import "strings"
  // ... and use strings.Contains(haystack, needle) directly.
  ```

  (The hand-rolled version above is just to avoid mid-code import drift; simplify to `strings.Contains` in the actual file.)

- [ ] **Step 2: Run the tests to verify they fail**

  Run: `cd sidecar && go test ./internal/media -run TestEvaluate`
  Expected: FAIL — `undefined: Evaluate`, `undefined: Verdict`.

- [ ] **Step 3: Write the implementation**

  Create `sidecar/internal/media/compat.go`:

  ```go
  package media

  import "strings"

  type Verdict struct {
      Supported bool
      Issues    []string
      Warnings  []string
  }

  var supportedContainerTokens = []string{
      "mov", "mp4", "m4a", "3gp", "3g2", "mj2",
      "matroska", "webm",
  }

  var supportedAudioCodecs = map[string]bool{
      "aac": true, "opus": true,
      "pcm_s16le": true, "pcm_s24le": true, "pcm_f32le": true,
      "mp3": true, "vorbis": true, "flac": true,
  }

  func Evaluate(r *MediaProbeResult) Verdict {
      v := Verdict{Supported: true, Issues: []string{}, Warnings: []string{}}

      if !containerSupported(r.Container.Format) {
          v.Supported = false
          v.Issues = append(v.Issues,
              "Unsupported container: "+r.Container.Format)
      }
      if r.Audio == nil {
          v.Supported = false
          v.Issues = append(v.Issues, "No audio stream detected in the file")
      } else if !supportedAudioCodecs[strings.ToLower(r.Audio.Codec)] {
          v.Supported = false
          v.Issues = append(v.Issues,
              "Unsupported audio codec: "+r.Audio.Codec)
      }

      // Warnings (non-blocking)
      if r.DurationSeconds == nil {
          v.Warnings = append(v.Warnings, "File duration could not be determined")
      }
      if r.Audio != nil && r.Audio.TrackCount > 1 {
          v.Warnings = append(v.Warnings,
              "Multiple audio tracks detected (selected track #"+
                  itoa(r.Audio.TrackIndex)+")")
      }
      if r.Audio != nil && r.Audio.Bitrate != nil && *r.Audio.Bitrate == 0 {
          v.Warnings = append(v.Warnings, "Audio bitrate reported as 0")
      }
      return v
  }

  func containerSupported(formatName string) bool {
      // ffprobe reports a comma-separated list of equivalent container short
      // names. Substring-match against any allow-list token.
      lowered := strings.ToLower(formatName)
      for _, tok := range supportedContainerTokens {
          if strings.Contains(lowered, tok) {
              return true
          }
      }
      return false
  }

  func itoa(n int) string { return strings.TrimSpace(formatInt(n)) }

  func formatInt(n int) string {
      // Avoid pulling fmt for one int → string in a hot path.
      // Equivalent to strconv.Itoa(n).
      return strings.TrimSpace(string([]byte(strconv.Itoa(n))))
  }
  ```

  Replace `formatInt` body with simply `return strconv.Itoa(n)` and add `"strconv"` import — the indirection above is just to make the dependency surface explicit. Drop the unused `itoa` wrapper.

- [ ] **Step 4: Run tests to verify they pass**

  Run: `cd sidecar && go test ./internal/media -run TestEvaluate`
  Expected: PASS (7 cases).

- [ ] **Step 5: Commit**

  ```bash
  git add sidecar/internal/media/compat.go sidecar/internal/media/compat_test.go
  git commit -m "[item 33] Compat: hybrid model — unsupported sets supported=false + issues, not RPC error"
  ```

### Task 4.6: Add a "fully supported container × codec matrix" table test

**Files:**
- Modify: `sidecar/internal/media/compat_test.go`

- [ ] **Step 1: Add a table-driven test**

  Append to `compat_test.go`:

  ```go
  func TestEvaluate_AllowListMatrix(t *testing.T) {
      cases := []struct {
          container string
          codec     string
      }{
          {"mov,mp4,m4a,3gp,3g2,mj2", "aac"},
          {"mov,mp4,m4a,3gp,3g2,mj2", "mp3"},
          {"matroska,webm", "opus"},
          {"matroska,webm", "vorbis"},
          {"matroska,webm", "flac"},
          {"mov,mp4,m4a,3gp,3g2,mj2", "pcm_s16le"},
          {"mov,mp4,m4a,3gp,3g2,mj2", "pcm_s24le"},
          {"mov,mp4,m4a,3gp,3g2,mj2", "pcm_f32le"},
      }
      for _, c := range cases {
          r := &MediaProbeResult{
              Container: Container{Format: c.container, LongName: c.container},
              Audio: &AudioStream{
                  Codec: c.codec, Channels: 2, SampleRate: 48000, TrackIndex: 0, TrackCount: 1,
                  Tracks: []AudioTrack{{Index: 0, Codec: c.codec, Channels: 2, SampleRate: 48000, IsDefault: true}},
              },
          }
          v := Evaluate(r)
          if !v.Supported {
              t.Errorf("%s/%s should be supported, got %+v", c.container, c.codec, v)
          }
      }
  }
  ```

- [ ] **Step 2: Run the test**

  Run: `cd sidecar && go test ./internal/media -run TestEvaluate_AllowListMatrix`
  Expected: PASS.

- [ ] **Step 3: Commit**

  ```bash
  git add sidecar/internal/media/compat_test.go
  git commit -m "[item 34] Compat: matrix test asserts all allow-list container x codec combos"
  ```

### Task 4.7: Create `sidecar/internal/media/errors.go`

**Files:**
- Create: `sidecar/internal/media/errors.go`

- [ ] **Step 1: Write the failing test**

  Create `sidecar/internal/media/errors_test.go`:

  ```go
  package media

  import (
      "context"
      "errors"
      "os"
      "testing"

      "github.com/example/studio-sound-app/sidecar/internal/ipc"
  )

  func TestMapRunError_FileNotFound(t *testing.T) {
      e := MapRunError(os.ErrNotExist, "")
      if e.Code != ipc.CodeFileNotFound { t.Errorf("code = %s", e.Code) }
  }

  func TestMapRunError_AccessDenied(t *testing.T) {
      e := MapRunError(os.ErrPermission, "")
      if e.Code != ipc.CodeAccessDenied { t.Errorf("code = %s", e.Code) }
  }

  func TestMapRunError_DeadlineExceeded(t *testing.T) {
      e := MapRunError(context.DeadlineExceeded, "")
      if e.Code != ipc.CodeFFprobeFailure { t.Errorf("code = %s", e.Code) }
      if !errors.Is(asError(e), nil) && e.Message == "" { t.Error("message empty") }
  }

  func TestMapRunError_CorruptMediaFromStderrMoov(t *testing.T) {
      e := MapRunError(errors.New("exit 1"), "moov atom not found")
      if e.Code != ipc.CodeCorruptMedia { t.Errorf("code = %s", e.Code) }
  }

  func TestMapRunError_CorruptMediaFromStderrInvalid(t *testing.T) {
      e := MapRunError(errors.New("exit 1"), "Invalid data found when processing input")
      if e.Code != ipc.CodeCorruptMedia { t.Errorf("code = %s", e.Code) }
  }

  func TestMapRunError_AccessDeniedFromStderr(t *testing.T) {
      e := MapRunError(errors.New("exit 1"), "Permission denied")
      if e.Code != ipc.CodeAccessDenied { t.Errorf("code = %s", e.Code) }
  }

  func TestMapRunError_DefaultIsFFprobeFailure(t *testing.T) {
      e := MapRunError(errors.New("oh no"), "weird unrelated stderr")
      if e.Code != ipc.CodeFFprobeFailure { t.Errorf("code = %s", e.Code) }
  }

  func TestMapParseError_AlwaysCorrupt(t *testing.T) {
      e := MapParseError(errors.New("bad json"))
      if e.Code != ipc.CodeCorruptMedia { t.Errorf("code = %s", e.Code) }
  }

  func asError(e *ipc.RPCError) error { return e }
  ```

  Adjust module path imports to match what `grep '^module' sidecar/go.mod` returns.

- [ ] **Step 2: Run the tests to verify they fail**

  Run: `cd sidecar && go test ./internal/media -run TestMap`
  Expected: FAIL — `undefined: MapRunError`, `undefined: MapParseError`.

- [ ] **Step 3: Write the implementation**

  Create `sidecar/internal/media/errors.go`:

  ```go
  package media

  import (
      "context"
      "errors"
      "os"
      "strings"

      "github.com/example/studio-sound-app/sidecar/internal/ipc"
  )

  // MapRunError converts a runner-layer error (and the captured stderr tail)
  // into a structured RPC error.
  func MapRunError(err error, stderrTail string) *ipc.RPCError {
      switch {
      case errors.Is(err, os.ErrNotExist):
          return ipc.NewRPCError(ipc.CodeFileNotFound, "media file not found")
      case errors.Is(err, os.ErrPermission):
          return ipc.NewRPCError(ipc.CodeAccessDenied, "permission denied opening media file")
      case errors.Is(err, context.DeadlineExceeded):
          return ipc.NewRPCError(ipc.CodeFFprobeFailure, "probe exceeded 10s deadline")
      case errors.Is(err, ErrFFprobeMissing):
          return ipc.NewRPCError(ipc.CodeFFprobeFailure, "ffprobe binary not located")
      }

      lower := strings.ToLower(stderrTail)
      switch {
      case strings.Contains(lower, "invalid data found"),
          strings.Contains(lower, "moov atom not found"),
          strings.Contains(lower, "end of file"):
          return ipc.NewRPCError(ipc.CodeCorruptMedia, "ffprobe could not parse the file")
      case strings.Contains(lower, "permission denied"):
          return ipc.NewRPCError(ipc.CodeAccessDenied, "permission denied reading media file")
      }

      return ipc.NewRPCError(ipc.CodeFFprobeFailure, "ffprobe failed: "+truncate(err.Error(), 256))
  }

  // MapParseError wraps a JSON-decode failure as CORRUPT_MEDIA.
  func MapParseError(err error) *ipc.RPCError {
      return ipc.NewRPCError(ipc.CodeCorruptMedia, "ffprobe output parse failed: "+truncate(err.Error(), 256))
  }

  func truncate(s string, n int) string {
      if len(s) <= n { return s }
      return s[:n] + "..."
  }
  ```

- [ ] **Step 4: Run tests to verify they pass**

  Run: `cd sidecar && go test ./internal/media -run TestMap`
  Expected: PASS.

- [ ] **Step 5: Commit**

  ```bash
  git add sidecar/internal/media/errors.go sidecar/internal/media/errors_test.go
  git commit -m "[item 35] Media errors: classify run/stderr/parse failures into RPC errors"
  ```

### Task 4.8: Create `sidecar/internal/media/media.go` orchestrator

**Files:**
- Create: `sidecar/internal/media/media.go`

- [ ] **Step 1: Write the failing test**

  Create `sidecar/internal/media/media_test.go`:

  ```go
  package media

  import (
      "context"
      "os"
      "path/filepath"
      "runtime"
      "testing"
      "time"
  )

  func TestProbe_HappyPathWithFakeFFprobe(t *testing.T) {
      fake := buildFake(t)
      // Make ffprobe emit a tiny valid output.
      t.Setenv("FAKE_FFPROBE_STDOUT", `{"format":{"format_name":"mov,mp4,m4a,3gp,3g2,mj2","format_long_name":"QuickTime / MOV","duration":"5.0","size":"1024"},"streams":[{"index":0,"codec_type":"video","codec_name":"h264","width":640,"height":480,"r_frame_rate":"30/1"},{"index":1,"codec_type":"audio","codec_name":"aac","channels":2,"sample_rate":"48000","channel_layout":"stereo","disposition":{"default":1}}]}`)
      tmp := t.TempDir()
      mediaPath := filepath.Join(tmp, "x.mp4")
      if err := os.WriteFile(mediaPath, []byte("x"), 0o644); err != nil { t.Fatal(err) }
      ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
      defer cancel()
      r, err := Probe(ctx, fake, mediaPath)
      if err != nil { t.Fatalf("unexpected err: %v", err) }
      if r == nil { t.Fatal("nil result") }
      if !r.Compatibility.Supported { t.Errorf("expected supported, got %+v", r.Compatibility) }
      if r.Audio == nil { t.Error("audio should be non-nil") }
  }

  func TestProbe_UnsupportedReturnsSuccessWithSupportedFalse(t *testing.T) {
      fake := buildFake(t)
      // unsupported container (asf/wmv)
      t.Setenv("FAKE_FFPROBE_STDOUT", `{"format":{"format_name":"asf","format_long_name":"ASF"},"streams":[{"index":0,"codec_type":"video","codec_name":"wmv2","width":640,"height":480,"r_frame_rate":"30/1"},{"index":1,"codec_type":"audio","codec_name":"wmav2","channels":2,"sample_rate":"44100","disposition":{"default":1}}]}`)
      tmp := t.TempDir()
      mediaPath := filepath.Join(tmp, "x.wmv")
      _ = os.WriteFile(mediaPath, []byte("x"), 0o644)
      ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
      defer cancel()
      r, err := Probe(ctx, fake, mediaPath)
      if err != nil { t.Fatalf("expected success, got err: %v", err) }
      if r.Compatibility.Supported { t.Errorf("expected supported=false, got %+v", r.Compatibility) }
      if len(r.Compatibility.Issues) == 0 { t.Error("expected non-empty issues") }
  }

  func TestProbe_FileNotFoundBeforeSpawn(t *testing.T) {
      fake := buildFake(t)
      ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
      defer cancel()
      _, err := Probe(ctx, fake, "/definitely/not/a/path.mp4")
      if err == nil { t.Fatal("expected err") }
  }

  func TestProbe_CorruptStderrMapsToCorruptMedia(t *testing.T) {
      fake := buildFake(t)
      t.Setenv("FAKE_FFPROBE_EXIT", "1")
      t.Setenv("FAKE_FFPROBE_STDERR", "Invalid data found when processing input")
      tmp := t.TempDir()
      mediaPath := filepath.Join(tmp, "x.mp4")
      _ = os.WriteFile(mediaPath, []byte("x"), 0o644)
      ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
      defer cancel()
      _, err := Probe(ctx, fake, mediaPath)
      if err == nil { t.Fatal("expected err") }
      _ = runtime.GOOS
  }
  ```

- [ ] **Step 2: Run the tests to verify they fail**

  Run: `cd sidecar && go test ./internal/media -run TestProbe`
  Expected: FAIL — `undefined: Probe`.

- [ ] **Step 3: Write the implementation**

  Create `sidecar/internal/media/media.go`:

  ```go
  package media

  import (
      "context"
      "os"
  )

  // Probe runs the full pipeline: stat → run → parse → normalize → evaluate.
  // Returns (*MediaProbeResult, nil) on success — even when
  // result.Compatibility.Supported is false (unsupported is a successful
  // probe, not a runner failure). Returns (nil, *ipc.RPCError) only when the
  // file cannot be opened, ffprobe cannot run, or its output cannot be parsed.
  func Probe(ctx context.Context, ffprobePath, mediaPath string) (*MediaProbeResult, error) {
      info, err := os.Stat(mediaPath)
      if err != nil {
          return nil, MapRunError(err, "")
      }
      runRes, runErr := Run(ctx, ffprobePath, mediaPath)
      if runErr != nil {
          stderrTail := ""
          if runRes != nil { stderrTail = runRes.StderrTail }
          return nil, MapRunError(runErr, stderrTail)
      }
      parsed, parseErr := Parse(runRes.Stdout)
      if parseErr != nil {
          return nil, MapParseError(parseErr)
      }
      result, _ := Normalize(parsed, mediaPath, info.Size())
      v := Evaluate(result)
      result.Compatibility = Compatibility{
          Supported: v.Supported,
          Issues:    v.Issues,
          Warnings:  v.Warnings,
      }
      return result, nil
  }
  ```

- [ ] **Step 4: Run tests to verify they pass**

  Run: `cd sidecar && go test ./internal/media -run TestProbe`
  Expected: PASS (4 cases).

- [ ] **Step 5: Commit**

  ```bash
  git add sidecar/internal/media/media.go sidecar/internal/media/media_test.go
  git commit -m "[item 36] Probe orchestrator: stat -> run -> parse -> normalize -> evaluate"
  ```

### Task 4.9: Update the package doc comment

**Files:**
- Modify: `sidecar/internal/media/media.go` (top of file)

- [ ] **Step 1: Add a package doc**

  Prepend at the very top of `media.go`:

  ```go
  // Package media wraps a bundled ffprobe subprocess and produces the
  // canonical MediaProbeResult shape consumed by the media.probe IPC method.
  //
  // The package layers are:
  //   - locator.go: resolve the ffprobe binary path
  //   - runner.go (+ _unix/_windows): invoke ffprobe with bounded I/O and
  //     process-group cancellation
  //   - parser.go: decode ffprobe JSON tolerantly
  //   - normalize.go: project the parsed output onto the wire-stable
  //     MediaProbeResult; audio is object-or-nil (encoded as JSON null when
  //     no audio stream is present)
  //   - compat.go: classify the file against the allow-list; unsupported
  //     produces supported=false + descriptive issues (NOT an RPC error)
  //   - errors.go: map runner / parse failures into RPC errors
  //   - media.go: the public Probe orchestrator
  package media
  ```

- [ ] **Step 2: Commit**

  ```bash
  git add sidecar/internal/media/media.go
  git commit -m "[item 37] Document internal/media package layering"
  ```

### Task 4.10: Final PR 4 test sweep

**Files:** (no file change)

- [ ] **Step 1: Run the full media package suite**

  Run: `cd sidecar && go test ./internal/media -count=1`
  Expected: all pass.

- [ ] **Step 2: Cross-compile**

  Run: `cd sidecar && GOOS=windows GOARCH=amd64 go build ./... && GOOS=darwin GOARCH=arm64 go build ./...`
  Expected: exit 0 for both.

- [ ] **Step 3: Item 38 complete — no commit**

  PR 4 ready for review.

---

## PR 5: `media.probe` IPC handler + integration tests + binary fixtures

**Branch:** `feature/phase-3-pr5`
**Base:** `feature/phase-3-pr4`
**Items:** 39–45
**Goal:** Register `media.probe` as a real IPC method routed through the dispatcher. Add the binary test-asset fixtures and an integration test that spawns the bundled ffprobe end-to-end via NDJSON.

### Task 5.1: Commit binary test-asset fixtures

**Files:**
- Create: `test-assets/tiny-h264-aac-stereo.mp4`
- Create: `test-assets/tiny-h264-aac-multitrack.mov`
- Create: `test-assets/tiny-vp9-opus.webm`
- Create: `test-assets/tiny-no-audio.mp4`
- Create: `test-assets/corrupt-truncated.mp4`
- Create: `test-assets/unicode-name-🎥-интервью.mp4`
- Create: `test-assets/README.md`

- [ ] **Step 1: Generate each fixture using a local ffmpeg**

  Implementer reference commands (run once with a working host ffmpeg + ffprobe to produce the binary fixtures; do NOT include ffmpeg as a dependency anywhere in the repo):

  ```sh
  # Source: Big Buck Bunny clip / Sintel clip / lavfi-generated audio — all CC.
  # tiny-h264-aac-stereo.mp4 (~1 MB)
  ffmpeg -y -f lavfi -i testsrc=duration=5:size=1280x720:rate=30 \
    -f lavfi -i "sine=frequency=440:duration=5" \
    -c:v libx264 -preset veryfast -tune zerolatency -c:a aac -b:a 96k \
    -metadata:s:a:0 title="Main" -disposition:a:0 default \
    test-assets/tiny-h264-aac-stereo.mp4

  # tiny-h264-aac-multitrack.mov with explicit default flag on track 1
  ffmpeg -y -f lavfi -i testsrc=duration=5:size=1280x720:rate=30 \
    -f lavfi -i "sine=frequency=440:duration=5" \
    -f lavfi -i "sine=frequency=880:duration=5" \
    -c:v libx264 -preset veryfast -c:a aac -b:a 96k \
    -map 0:v:0 -map 1:a:0 -map 2:a:0 \
    -disposition:a:0 default -disposition:a:1 0 \
    -metadata:s:a:0 title="Microphone" \
    -metadata:s:a:1 title="Game Audio" \
    test-assets/tiny-h264-aac-multitrack.mov

  # tiny-vp9-opus.webm
  ffmpeg -y -f lavfi -i testsrc=duration=5:size=1280x720:rate=30 \
    -f lavfi -i "sine=frequency=440:duration=5" \
    -c:v libvpx-vp9 -b:v 200k -c:a libopus -b:a 64k \
    test-assets/tiny-vp9-opus.webm

  # tiny-no-audio.mp4
  ffmpeg -y -f lavfi -i testsrc=duration=5:size=1280x720:rate=30 \
    -c:v libx264 -an test-assets/tiny-no-audio.mp4

  # corrupt-truncated.mp4 — write 512 KiB then truncate
  ffmpeg -y -f lavfi -i testsrc=duration=10:size=1280x720:rate=30 \
    -f lavfi -i "sine=frequency=440:duration=10" \
    -c:v libx264 -c:a aac test-assets/tiny-h264-aac-stereo-tmp.mp4
  dd if=test-assets/tiny-h264-aac-stereo-tmp.mp4 of=test-assets/corrupt-truncated.mp4 bs=1024 count=512
  rm test-assets/tiny-h264-aac-stereo-tmp.mp4

  # unicode-name-🎥-интервью.mp4 — same content as the small h264 fixture
  cp test-assets/tiny-h264-aac-stereo.mp4 "test-assets/unicode-name-🎥-интервью.mp4"
  ```

- [ ] **Step 2: Verify total size budget**

  Run: `du -sk test-assets/ | awk '{print $1}'`
  Expected: < 30000 (i.e., under 30 MB).

- [ ] **Step 3: Write `test-assets/README.md`**

  ```markdown
  # Test assets

  Fixture videos used by `sidecar/internal/media/integration_test.go` and manual QA.
  All files are CC0 / synthesised via `ffmpeg -f lavfi` from `testsrc` and `sine`
  — no third-party content.

  | File | Expected verdict |
  |---|---|
  | tiny-h264-aac-stereo.mp4 | supported=true, video=h264, audio=aac stereo, 5s |
  | tiny-h264-aac-multitrack.mov | supported=true, audio.tracks.length=2, default=Microphone |
  | tiny-vp9-opus.webm | supported=true, video=vp9, audio=opus |
  | tiny-no-audio.mp4 | supported=false, issues contains "No audio stream detected" |
  | corrupt-truncated.mp4 | RPC error CORRUPT_MEDIA |
  | unicode-name-🎥-интервью.mp4 | supported=true (Unicode path) |

  Regenerate with the ffmpeg invocations recorded in PR 5 task 5.1.
  ```

- [ ] **Step 4: Commit**

  ```bash
  git add test-assets/
  git commit -m "[item 39] Add binary test-asset fixtures (CC0, generated via lavfi)"
  ```

### Task 5.2: Create the `media.probe` handler

**Files:**
- Create: `sidecar/internal/ipc/handlers/media_probe.go`

- [ ] **Step 1: Write the failing test**

  Create `sidecar/internal/ipc/handlers/media_probe_test.go`:

  ```go
  package handlers

  import (
      "context"
      "encoding/json"
      "os"
      "path/filepath"
      "testing"
  )

  func TestProbeHandler_RejectsMissingPath(t *testing.T) {
      _, err := ProbeHandler(context.Background(), "id-1", json.RawMessage(`{}`))
      if err == nil { t.Fatal("expected err") }
  }

  func TestProbeHandler_RejectsEmptyPath(t *testing.T) {
      _, err := ProbeHandler(context.Background(), "id-1", json.RawMessage(`{"path":""}`))
      if err == nil { t.Fatal("expected err") }
  }

  func TestProbeHandler_HappyPathReturnsResult(t *testing.T) {
      // The handler resolves ffprobe via STUDIO_FFPROBE_PATH. Use a stub via
      // FakeFFprobe so this test runs without the real binary.
      // The stub emits canned ffprobe JSON via FAKE_FFPROBE_STDOUT.
      fake := buildFakeForHandlerTest(t)
      t.Setenv("STUDIO_FFPROBE_PATH", fake)
      t.Setenv("FAKE_FFPROBE_STDOUT", `{"format":{"format_name":"mov,mp4,m4a,3gp,3g2,mj2","duration":"5.0"},"streams":[{"index":0,"codec_type":"video","codec_name":"h264","width":640,"height":480,"r_frame_rate":"30/1"},{"index":1,"codec_type":"audio","codec_name":"aac","channels":2,"sample_rate":"48000","disposition":{"default":1}}]}`)
      tmp := t.TempDir()
      media := filepath.Join(tmp, "x.mp4")
      _ = os.WriteFile(media, []byte("x"), 0o644)
      payload, _ := json.Marshal(map[string]string{"path": media})
      r, err := ProbeHandler(context.Background(), "id-1", payload)
      if err != nil { t.Fatalf("unexpected err: %v", err) }
      if r == nil { t.Fatal("nil result") }
  }
  ```

  Add `buildFakeForHandlerTest`: copies the fakeffprobe helper into a test temp dir. Reuse the build logic from `internal/media/runner_test.go::buildFake` (extract to a tiny `testhelpers_test.go` file in `handlers` if needed, or duplicate — duplicating ~15 lines is fine).

- [ ] **Step 2: Run the tests to verify they fail**

  Run: `cd sidecar && go test ./internal/ipc/handlers -run TestProbeHandler`
  Expected: FAIL — `undefined: ProbeHandler`.

- [ ] **Step 3: Write the implementation**

  Create `sidecar/internal/ipc/handlers/media_probe.go`:

  ```go
  package handlers

  import (
      "context"
      "encoding/json"
      "log/slog"
      "sync"
      "time"

      "github.com/example/studio-sound-app/sidecar/internal/ipc"
      "github.com/example/studio-sound-app/sidecar/internal/media"
  )

  // Inline JSON Schema for payload validation. Must mirror $defs.ProbePayload
  // in schemas/media.probe.schema.json.
  const probePayloadSchema = `{
    "$schema": "https://json-schema.org/draft/2020-12/schema",
    "type": "object",
    "additionalProperties": false,
    "required": ["path"],
    "properties": { "path": { "type": "string", "minLength": 1, "maxLength": 4096 } }
  }`

  var (
      probeValidatorOnce sync.Once
      probeValidator     *ipc.Validator

      ffprobeOnce sync.Once
      ffprobePath string
      ffprobeErr  error
  )

  type probeReq struct{ Path string `json:"path"` }

  // ProbeHandler is the media.probe handler registered with the dispatcher.
  func ProbeHandler(ctx context.Context, id string, payload json.RawMessage) (any, error) {
      probeValidatorOnce.Do(func() {
          probeValidator = ipc.NewValidator(probePayloadSchema)
      })
      if err := probeValidator.Validate(payload); err != nil {
          return nil, ipc.NewRPCError(ipc.CodeInvalidPayload, err.Error())
      }
      var req probeReq
      if err := json.Unmarshal(payload, &req); err != nil {
          return nil, ipc.NewRPCError(ipc.CodeInvalidPayload, err.Error())
      }

      ffprobeOnce.Do(func() {
          ffprobePath, ffprobeErr = media.ResolveFFprobe()
      })
      if ffprobeErr != nil {
          return nil, ipc.NewRPCError(ipc.CodeFFprobeFailure, ffprobeErr.Error())
      }

      slog.Info("probe_started", "id", id, "path", req.Path)
      start := time.Now()

      ctx, cancel := context.WithTimeout(ctx, 10*time.Second)
      defer cancel()
      result, err := media.Probe(ctx, ffprobePath, req.Path)
      elapsed := time.Since(start).Milliseconds()
      if err != nil {
          code := ""
          if rpc, ok := err.(*ipc.RPCError); ok { code = rpc.Code }
          slog.Warn("probe_failed", "id", id, "duration_ms", elapsed, "code", code)
          return nil, err
      }
      slog.Info("probe_completed", "id", id, "duration_ms", elapsed,
          "supported", result.Compatibility.Supported)
      return result, nil
  }
  ```

  Adjust import paths to match `sidecar/go.mod`'s module path. If `ipc.NewValidator` has a different constructor signature (the LLD references `*ipc.Validator` wrapping `santhosh-tekuri/jsonschema/v5`), inspect `sidecar/internal/ipc/validator.go:19-30` and adapt the construction call accordingly.

- [ ] **Step 4: Run tests to verify they pass**

  Run: `cd sidecar && go test ./internal/ipc/handlers -run TestProbeHandler`
  Expected: PASS.

- [ ] **Step 5: Commit**

  ```bash
  git add sidecar/internal/ipc/handlers/media_probe.go sidecar/internal/ipc/handlers/media_probe_test.go
  git commit -m "[item 40] Add media.probe handler: validate, resolve ffprobe, 10s deadline, log lifecycle"
  ```

### Task 5.3: Register the handler in `cli.Run`

**Files:**
- Modify: `sidecar/internal/cli/cli.go:52-56`

- [ ] **Step 1: Add the registration line**

  In the `case "serve":` block inside `cli.Run`, add alongside the existing `d.Register(...)` calls:

  ```go
  d.Register("media.probe", handlers.ProbeHandler)
  ```

- [ ] **Step 2: Verify it builds**

  Run: `cd sidecar && go build ./...`
  Expected: exit 0.

- [ ] **Step 3: Commit**

  ```bash
  git add sidecar/internal/cli/cli.go
  git commit -m "[item 41] Register media.probe handler with the dispatcher"
  ```

### Task 5.4: Add the media-package integration test

**Files:**
- Create: `sidecar/internal/media/integration_test.go`

- [ ] **Step 1: Write the failing test**

  ```go
  //go:build integration

  package media

  import (
      "context"
      "os"
      "path/filepath"
      "testing"
      "time"
  )

  func ffprobePath(t *testing.T) string {
      t.Helper()
      p := os.Getenv("STUDIO_FFPROBE_PATH")
      if p == "" {
          t.Skip("STUDIO_FFPROBE_PATH not set; skipping integration test")
      }
      if _, err := os.Stat(p); err != nil {
          t.Skipf("ffprobe not at %s: %v", p, err)
      }
      return p
  }

  func assetPath(t *testing.T, name string) string {
      t.Helper()
      // tests live at sidecar/internal/media; assets at <repo>/test-assets
      return filepath.Join("..", "..", "..", "test-assets", name)
  }

  func TestIntegration_H264AACStereo(t *testing.T) {
      ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
      defer cancel()
      r, err := Probe(ctx, ffprobePath(t), assetPath(t, "tiny-h264-aac-stereo.mp4"))
      if err != nil { t.Fatalf("unexpected err: %v", err) }
      if !r.Compatibility.Supported { t.Errorf("expected supported, got %+v", r.Compatibility) }
      if r.Audio == nil || r.Audio.Codec != "aac" { t.Errorf("audio = %+v", r.Audio) }
  }

  func TestIntegration_NoAudio(t *testing.T) {
      ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
      defer cancel()
      r, err := Probe(ctx, ffprobePath(t), assetPath(t, "tiny-no-audio.mp4"))
      if err != nil { t.Fatalf("unexpected err: %v", err) }
      if r.Compatibility.Supported { t.Error("expected supported=false") }
      if r.Audio != nil { t.Errorf("audio should be nil, got %+v", r.Audio) }
  }

  func TestIntegration_Multitrack(t *testing.T) {
      ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
      defer cancel()
      r, err := Probe(ctx, ffprobePath(t), assetPath(t, "tiny-h264-aac-multitrack.mov"))
      if err != nil { t.Fatal(err) }
      if r.Audio == nil || r.Audio.TrackCount != 2 { t.Errorf("audio = %+v", r.Audio) }
      // The default-flagged track must be selected.
      var defaultIdx int
      for _, tr := range r.Audio.Tracks { if tr.IsDefault { defaultIdx = tr.Index } }
      if r.Audio.TrackIndex != defaultIdx {
          t.Errorf("default mismatch: trackIndex=%d, default-flagged=%d", r.Audio.TrackIndex, defaultIdx)
      }
  }

  func TestIntegration_VP9Opus(t *testing.T) {
      ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
      defer cancel()
      r, err := Probe(ctx, ffprobePath(t), assetPath(t, "tiny-vp9-opus.webm"))
      if err != nil { t.Fatal(err) }
      if !r.Compatibility.Supported { t.Errorf("got %+v", r.Compatibility) }
  }

  func TestIntegration_Corrupt(t *testing.T) {
      ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
      defer cancel()
      _, err := Probe(ctx, ffprobePath(t), assetPath(t, "corrupt-truncated.mp4"))
      if err == nil { t.Fatal("expected err") }
  }

  func TestIntegration_UnicodePath(t *testing.T) {
      ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
      defer cancel()
      r, err := Probe(ctx, ffprobePath(t), assetPath(t, "unicode-name-🎥-интервью.mp4"))
      if err != nil { t.Fatalf("unexpected err: %v", err) }
      if !r.Compatibility.Supported { t.Errorf("got %+v", r.Compatibility) }
  }

  func TestIntegration_FileNotFound(t *testing.T) {
      ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
      defer cancel()
      _, err := Probe(ctx, ffprobePath(t), assetPath(t, "definitely-does-not-exist.mp4"))
      if err == nil { t.Fatal("expected err") }
  }
  ```

- [ ] **Step 2: Run the test locally**

  Run: `cd sidecar && STUDIO_FFPROBE_PATH=$(pwd)/../app/src-tauri/binaries/ffprobe-$(uname -m | sed 's/arm64/aarch64-apple-darwin/;s/x86_64/x86_64-apple-darwin/') go test -tags=integration -run TestIntegration ./internal/media`
  Expected: all 7 cases pass on the host platform.

- [ ] **Step 3: Commit**

  ```bash
  git add sidecar/internal/media/integration_test.go
  git commit -m "[item 42] Integration test: real ffprobe + binary fixtures end-to-end"
  ```

### Task 5.5: Add an end-to-end `media.probe` envelope round-trip test

**Files:**
- Modify: `sidecar/internal/ipc/integration_test.go`

- [ ] **Step 1: Add a `TestMediaProbeRoundTrip` case**

  Follow the existing test pattern in `sidecar/internal/ipc/integration_test.go`. Sketch:

  ```go
  func TestMediaProbeRoundTrip(t *testing.T) {
      if os.Getenv("STUDIO_FFPROBE_PATH") == "" {
          t.Skip("STUDIO_FFPROBE_PATH not set")
      }
      // Spawn sidecar (using existing helper), then send a media.probe envelope
      // with payload {"path": "<test-assets/tiny-h264-aac-stereo.mp4>"}, read
      // the response envelope, assert kind=response and result.compatibility.supported=true.
  }
  ```

  Inspect the file to use the correct helper name (e.g., `newSidecar`, `sendEnvelope`); follow the existing tests' shape exactly.

- [ ] **Step 2: Run with the integration build tag**

  Run: `cd sidecar && STUDIO_FFPROBE_PATH=... go test -tags=integration -run TestMediaProbeRoundTrip ./internal/ipc`
  Expected: PASS.

- [ ] **Step 3: Commit**

  ```bash
  git add sidecar/internal/ipc/integration_test.go
  git commit -m "[item 43] Add media.probe envelope round-trip integration test"
  ```

### Task 5.6: Add a "kill-mid-probe leaves no orphan" test

**Files:**
- Modify: `sidecar/internal/media/integration_test.go`

- [ ] **Step 1: Add the test**

  Append:

  ```go
  func TestIntegration_KillMidProbeLeavesNoOrphan(t *testing.T) {
      ctx, cancel := context.WithCancel(context.Background())
      // Cancel quickly so ffprobe is killed mid-run.
      go func() { time.Sleep(50 * time.Millisecond); cancel() }()
      _, _ = Probe(ctx, ffprobePath(t), assetPath(t, "tiny-h264-aac-stereo.mp4"))
      // Best-effort: re-run and assert success (no lingering FD on the file).
      ctx2, cancel2 := context.WithTimeout(context.Background(), 10*time.Second)
      defer cancel2()
      _, err := Probe(ctx2, ffprobePath(t), assetPath(t, "tiny-h264-aac-stereo.mp4"))
      if err != nil { t.Fatalf("second probe failed: %v", err) }
  }
  ```

- [ ] **Step 2: Verify**

  Run: `cd sidecar && STUDIO_FFPROBE_PATH=... go test -tags=integration -run TestIntegration_KillMidProbeLeavesNoOrphan ./internal/media`
  Expected: PASS.

- [ ] **Step 3: Commit**

  ```bash
  git add sidecar/internal/media/integration_test.go
  git commit -m "[item 44] Integration test: cancel mid-probe; subsequent probe still succeeds"
  ```

### Task 5.7: Final PR 5 test sweep

**Files:** (no file change)

- [ ] **Step 1: Run unit + integration**

  Run: `cd sidecar && go test ./... && STUDIO_FFPROBE_PATH=... go test -tags=integration ./...`
  Expected: all pass.

- [ ] **Step 2: Item 45 complete — no commit**

  PR 5 ready for review.

---

## PR 6: Tauri `media_probe` command + IPC wrapper + workspace state machine

**Branch:** `feature/phase-3-pr6`
**Base:** `feature/phase-3-pr5`
**Items:** 46–52
**Goal:** Expose `media.probe` to the frontend via a Tauri command (`Result<Value, SerializableIpcError>`, 30s timeout), add a typed `probe(path)` wrapper on the TS IPC client, and land the Zustand workspace store with the state-machine transitions from PRD §12. No UI yet — that's PR 7.

### Task 6.1: Add the `media_probe` Tauri command + 30s timeout arm

**Files:**
- Modify: `app/src-tauri/src/commands.rs`

- [ ] **Step 1: Add the timeout arm**

  In `default_timeout`, add the new arm:

  ```rust
  fn default_timeout(method: &str) -> Duration {
      match method {
          "system.ping" => Duration::from_secs(2),
          "system.echo" => Duration::from_secs(5),
          "system.shutdown" => Duration::from_secs(2),
          "media.probe" => Duration::from_secs(30),
          _ => Duration::from_secs(10),
      }
  }
  ```

- [ ] **Step 2: Add the command**

  Append below `ipc_shutdown`:

  ```rust
  /// Probes a media file and returns the canonical MediaProbeResult.
  ///
  /// `path` is forwarded verbatim — the supervisor's child has the bundled
  /// ffprobe path in env, and the sidecar handler validates / resolves it.
  #[tauri::command]
  pub async fn media_probe(
      path: String,
      client: State<'_, Arc<IpcClient>>,
  ) -> Result<serde_json::Value, SerializableIpcError> {
      let payload = serde_json::json!({ "path": path });
      client
          .call("media.probe", payload, default_timeout("media.probe"))
          .await
          .map_err(SerializableIpcError::from)
  }
  ```

- [ ] **Step 3: Verify Rust builds**

  Run: `cd app/src-tauri && cargo build`
  Expected: exit 0.

- [ ] **Step 4: Commit**

  ```bash
  git add app/src-tauri/src/commands.rs
  git commit -m "[item 46] Add media_probe Tauri command with 30s timeout"
  ```

### Task 6.2: Register `media_probe` in the Tauri handler list

**Files:**
- Modify: `app/src-tauri/src/lib.rs`

- [ ] **Step 1: Locate the `tauri::generate_handler!` invocation**

  Run: `grep -n 'generate_handler' app/src-tauri/src/lib.rs`
  Expected: shows the existing list.

- [ ] **Step 2: Append `commands::media_probe`**

  In `tauri::generate_handler![...]`, add `commands::media_probe` alongside the existing entries.

- [ ] **Step 3: Verify**

  Run: `cd app/src-tauri && cargo build`
  Expected: exit 0.

- [ ] **Step 4: Commit**

  ```bash
  git add app/src-tauri/src/lib.rs
  git commit -m "[item 47] Register media_probe in Tauri handler list"
  ```

### Task 6.3: Add the `probe(path)` wrapper on the TS IPC client

**Files:**
- Modify: `app/src/ipc/client.ts`
- Modify: `app/src/ipc/client.test.ts`

- [ ] **Step 1: Write the failing test**

  Append to `app/src/ipc/client.test.ts`:

  ```ts
  import { vi } from 'vitest';
  vi.mock('@tauri-apps/api/core', () => ({
    invoke: vi.fn(),
  }));
  import { invoke } from '@tauri-apps/api/core';
  import { probe } from './client';

  describe('probe', () => {
    it('invokes media_probe with the given path', async () => {
      (invoke as ReturnType<typeof vi.fn>).mockResolvedValueOnce({
        id: 'A', path: '/x', filename: 'x', sizeBytes: 1,
        container: { format: 'mov,mp4', longName: '' },
        audio: null,
        compatibility: { supported: false, issues: ['no audio'], warnings: [] },
      });
      const r = await probe('/x');
      expect(invoke).toHaveBeenCalledWith('media_probe', { path: '/x' });
      expect(r.compatibility.supported).toBe(false);
    });

    it('throws IpcError on structured rejection', async () => {
      (invoke as ReturnType<typeof vi.fn>).mockRejectedValueOnce({
        code: 'FILE_NOT_FOUND', message: 'missing',
      });
      await expect(probe('/x')).rejects.toMatchObject({ code: 'FILE_NOT_FOUND' });
    });
  });
  ```

- [ ] **Step 2: Run the test to verify it fails**

  Run: `npm --prefix app run test -- client.test.ts --run`
  Expected: FAIL — `probe is not exported`.

- [ ] **Step 3: Implement the wrapper**

  In `app/src/ipc/client.ts`, add (alongside existing wrappers like `ping`, `echo`, `shutdown`):

  ```ts
  import type { ProbeResult } from './generated/media.probe';

  export async function probe(path: string): Promise<ProbeResult> {
    try {
      return (await invoke('media_probe', { path })) as ProbeResult;
    } catch (e) {
      throw toIpcError(e);
    }
  }
  ```

  If `invoke` is imported under a different alias in the file, follow that pattern; if `toIpcError` is defined later, reorder declarations as needed.

- [ ] **Step 4: Run tests to verify they pass**

  Run: `npm --prefix app run test -- client.test.ts --run`
  Expected: PASS.

- [ ] **Step 5: Commit**

  ```bash
  git add app/src/ipc/client.ts app/src/ipc/client.test.ts
  git commit -m "[item 48] Add typed probe(path) wrapper on the TS IPC client"
  ```

### Task 6.4: Add the `zustand` dependency

**Files:**
- Modify: `app/package.json`

- [ ] **Step 1: Install zustand**

  Run: `npm --prefix app install zustand@^4.5.0`
  Expected: exit 0; `package.json` and `package-lock.json` updated.

- [ ] **Step 2: Commit**

  ```bash
  git add app/package.json app/package-lock.json
  git commit -m "[item 49] Add zustand@^4.5.0 dependency for workspace state"
  ```

### Task 6.5: Build the Zustand workspace store

**Files:**
- Create: `app/src/state/workspace.ts`

- [ ] **Step 1: Write the failing test**

  Create `app/src/state/workspace.test.ts`:

  ```ts
  import { describe, it, expect, beforeEach, vi } from 'vitest';

  vi.mock('@/ipc/client', () => ({
    probe: vi.fn(),
  }));
  import { probe } from '@/ipc/client';
  import { useWorkspace, resetWorkspaceForTest } from './workspace';

  describe('workspace store', () => {
    beforeEach(() => {
      resetWorkspaceForTest();
      (probe as ReturnType<typeof vi.fn>).mockReset();
    });

    it('starts in EMPTY state', () => {
      expect(useWorkspace.getState().status).toBe('EMPTY');
    });

    it('loadFile transitions EMPTY -> FILE_LOADED -> PROBING -> READY on success', async () => {
      const result = {
        id: 'A', path: '/x.mp4', filename: 'x.mp4', sizeBytes: 1,
        container: { format: 'mp4', longName: '' }, audio: null,
        compatibility: { supported: false, issues: ['no audio'], warnings: [] },
      };
      (probe as ReturnType<typeof vi.fn>).mockResolvedValue(result);
      const transitions: string[] = [];
      useWorkspace.subscribe((s) => transitions.push(s.status));
      await useWorkspace.getState().loadFile('/x.mp4');
      expect(transitions).toContain('FILE_LOADED');
      expect(transitions).toContain('PROBING');
      expect(useWorkspace.getState().status).toBe('READY');
      expect(useWorkspace.getState().result).toEqual(result);
    });

    it('loadFile transitions to ERROR on probe rejection', async () => {
      (probe as ReturnType<typeof vi.fn>).mockRejectedValue({ code: 'FILE_NOT_FOUND', message: 'missing' });
      await useWorkspace.getState().loadFile('/x.mp4');
      expect(useWorkspace.getState().status).toBe('ERROR');
      expect(useWorkspace.getState().error?.code).toBe('FILE_NOT_FOUND');
    });

    it('retry replays loadFile on the same path', async () => {
      (probe as ReturnType<typeof vi.fn>)
        .mockRejectedValueOnce({ code: 'FFPROBE_FAILURE', message: 'transient' })
        .mockResolvedValueOnce({
          id: 'B', path: '/x.mp4', filename: 'x.mp4', sizeBytes: 1,
          container: { format: 'mp4', longName: '' }, audio: null,
          compatibility: { supported: false, issues: [], warnings: [] },
        });
      await useWorkspace.getState().loadFile('/x.mp4');
      expect(useWorkspace.getState().status).toBe('ERROR');
      await useWorkspace.getState().retry();
      expect(useWorkspace.getState().status).toBe('READY');
    });

    it('clearFile resets to EMPTY', async () => {
      (probe as ReturnType<typeof vi.fn>).mockResolvedValue({
        id: 'A', path: '/x.mp4', filename: 'x.mp4', sizeBytes: 1,
        container: { format: 'mp4', longName: '' }, audio: null,
        compatibility: { supported: true, issues: [], warnings: [] },
      });
      await useWorkspace.getState().loadFile('/x.mp4');
      useWorkspace.getState().clearFile();
      expect(useWorkspace.getState().status).toBe('EMPTY');
      expect(useWorkspace.getState().result).toBeUndefined();
    });

    it('replaceFile clears then loads the new path', async () => {
      (probe as ReturnType<typeof vi.fn>).mockResolvedValue({
        id: 'A', path: '/y.mp4', filename: 'y.mp4', sizeBytes: 1,
        container: { format: 'mp4', longName: '' }, audio: null,
        compatibility: { supported: true, issues: [], warnings: [] },
      });
      await useWorkspace.getState().loadFile('/x.mp4');
      await useWorkspace.getState().replaceFile('/y.mp4');
      expect(useWorkspace.getState().status).toBe('READY');
      expect(useWorkspace.getState().result?.path).toBe('/y.mp4');
    });

    it('retry is a no-op when there is no path', async () => {
      await useWorkspace.getState().retry();
      expect(useWorkspace.getState().status).toBe('EMPTY');
    });
  });
  ```

- [ ] **Step 2: Run the test to verify it fails**

  Run: `npm --prefix app run test -- workspace.test.ts --run`
  Expected: FAIL — `Cannot find module './workspace'`.

- [ ] **Step 3: Implement the store**

  Create `app/src/state/workspace.ts`:

  ```ts
  import { create } from 'zustand';
  import { probe } from '@/ipc/client';
  import type { ProbeResult } from '@/ipc/generated/media.probe';
  import type { IpcError } from '@/ipc/client';

  export type WorkspaceStatus =
    | 'EMPTY'
    | 'FILE_LOADED'
    | 'PROBING'
    | 'READY'
    | 'ERROR'
    | 'RETRYING'
    | 'REMOVED';

  export interface WorkspaceState {
    status: WorkspaceStatus;
    path?: string;
    result?: ProbeResult;
    error?: IpcError;
    loadFile: (path: string) => Promise<void>;
    replaceFile: (path: string) => Promise<void>;
    clearFile: () => void;
    retry: () => Promise<void>;
  }

  const initial: Omit<WorkspaceState, 'loadFile' | 'replaceFile' | 'clearFile' | 'retry'> = {
    status: 'EMPTY',
    path: undefined,
    result: undefined,
    error: undefined,
  };

  export const useWorkspace = create<WorkspaceState>((set, get) => ({
    ...initial,

    async loadFile(path) {
      set({ status: 'FILE_LOADED', path, result: undefined, error: undefined });
      set({ status: 'PROBING' });
      try {
        const result = await probe(path);
        set({ status: 'READY', result, error: undefined });
      } catch (e) {
        set({ status: 'ERROR', error: e as IpcError, result: undefined });
      }
    },

    async replaceFile(path) {
      // Mark REMOVED for one React tick, then delegate to loadFile.
      set({ status: 'REMOVED', result: undefined, error: undefined });
      await get().loadFile(path);
    },

    clearFile() {
      set({ ...initial });
    },

    async retry() {
      const { path } = get();
      if (!path) return;
      set({ status: 'RETRYING', error: undefined });
      try {
        const result = await probe(path);
        set({ status: 'READY', result, error: undefined });
      } catch (e) {
        set({ status: 'ERROR', error: e as IpcError });
      }
    },
  }));

  // For tests only — resets the store between cases.
  export function resetWorkspaceForTest() {
    useWorkspace.setState({ ...initial });
  }

  // Selector helpers used by components.
  export const useWorkspaceStatus = () => useWorkspace((s) => s.status);
  export const useWorkspaceFile = () => useWorkspace((s) => ({ path: s.path, result: s.result, error: s.error }));
  ```

- [ ] **Step 4: Run tests to verify they pass**

  Run: `npm --prefix app run test -- workspace.test.ts --run`
  Expected: PASS (7 cases).

- [ ] **Step 5: Commit**

  ```bash
  git add app/src/state/workspace.ts app/src/state/workspace.test.ts
  git commit -m "[item 50] Add Zustand workspace store with state-machine transitions"
  ```

### Task 6.6: Verify the typecheck and full test sweep

**Files:** (no file change)

- [ ] **Step 1: Typecheck**

  Run: `npm --prefix app run typecheck`
  Expected: exit 0.

- [ ] **Step 2: Frontend test suite**

  Run: `npm --prefix app run test -- --run`
  Expected: all pass.

- [ ] **Step 3: Rust suite**

  Run: `cd app/src-tauri && cargo test`
  Expected: all pass.

- [ ] **Step 4: Item 51 complete — no commit**

### Task 6.7: Smoke-test the round-trip locally (manual / non-blocking)

**Files:** (no file change)

- [ ] **Step 1: Manually probe a file via Tauri's IPC**

  In a `tauri dev` session, open the existing Diagnostics panel (Ctrl+Shift+D) and (if there's a probe button — see PR 7) probe a fixture. If not yet wired, this step is deferred to PR 7.

- [ ] **Step 2: Item 52 complete — PR 6 ready for review.**

---

## PR 7: Workspace UI core flow

**Branch:** `feature/phase-3-pr7`
**Base:** `feature/phase-3-pr6`
**Items:** 53–61
**Goal:** Ship the visible workspace: design tokens, drop-only `EmptyState`, `useDropTarget` hook, `ActiveFileCard` (status/metadata/buttons), `WorkspaceShell` layout, and `App.tsx` replacement. After this PR, the user can drop a video and see "probing → ready/unsupported/error" with a basic metadata summary. No drawer, no replace dialog, no per-error panels yet — those are PR 8.

### Task 7.1: Add CSS design tokens

**Files:**
- Create: `app/src/styles/tokens.css`
- Modify: `app/src/main.tsx`

- [ ] **Step 1: Write the tokens file**

  ```css
  :root {
    /* radius */
    --radius-card: 16px;
    --radius-button: 12px;
    --radius-input: 8px;

    /* spacing */
    --space-1: 4px;
    --space-2: 8px;
    --space-3: 12px;
    --space-4: 16px;
    --space-6: 24px;
    --space-8: 32px;

    /* motion */
    --motion-fast: 120ms;
    --motion-medium: 200ms;
    --motion-slow: 320ms;

    /* status colours (PRD §14.4) */
    --color-status-gray: #9ca3af;
    --color-status-spinner: #3b82f6;
    --color-status-green: #10b981;
    --color-status-yellow: #f59e0b;
    --color-status-red: #ef4444;
  }
  ```

- [ ] **Step 2: Import once from `app/src/main.tsx`**

  Add at the top (alongside any existing CSS imports):

  ```ts
  import './styles/tokens.css';
  ```

- [ ] **Step 3: Verify build**

  Run: `npm --prefix app run build`
  Expected: exit 0.

- [ ] **Step 4: Commit**

  ```bash
  git add app/src/styles/tokens.css app/src/main.tsx
  git commit -m "[item 53] Add CSS design tokens (radius, spacing, motion, status colours)"
  ```

### Task 7.2: Create the `useDropTarget` hook

**Files:**
- Create: `app/src/hooks/useDropTarget.ts`

- [ ] **Step 1: Write the failing test**

  Create `app/src/hooks/useDropTarget.test.tsx`:

  ```tsx
  import { describe, it, expect, vi, beforeEach } from 'vitest';
  import { renderHook, act } from '@testing-library/react';

  // Stub the Tauri webview API.
  const handlers: Array<(e: any) => void> = [];
  vi.mock('@tauri-apps/api/webview', () => ({
    getCurrentWebview: () => ({
      onDragDropEvent: (cb: (e: any) => void) => {
        handlers.push(cb);
        return Promise.resolve(() => {
          const i = handlers.indexOf(cb);
          if (i >= 0) handlers.splice(i, 1);
        });
      },
    }),
  }));

  import { useDropTarget } from './useDropTarget';

  beforeEach(() => { handlers.length = 0; });

  describe('useDropTarget', () => {
    it('emits the dropped path via onDrop', async () => {
      const onDrop = vi.fn();
      const { result } = renderHook(() => useDropTarget({ onDrop }));
      // Wait for the subscription to register.
      await Promise.resolve();
      await act(async () => {
        handlers[0]?.({ payload: { type: 'drop', paths: ['/a/b.mp4'] } });
      });
      expect(onDrop).toHaveBeenCalledWith('/a/b.mp4');
      expect(result.current.isDragOver).toBe(false);
    });

    it('sets isDragOver=true on dragenter', async () => {
      const { result } = renderHook(() => useDropTarget({ onDrop: () => {} }));
      await Promise.resolve();
      await act(async () => {
        handlers[0]?.({ payload: { type: 'enter' } });
      });
      expect(result.current.isDragOver).toBe(true);
    });

    it('ignores multi-file drops beyond the first path', async () => {
      const onDrop = vi.fn();
      const onIgnored = vi.fn();
      renderHook(() => useDropTarget({ onDrop, onMultiFileIgnored: onIgnored }));
      await Promise.resolve();
      await act(async () => {
        handlers[0]?.({ payload: { type: 'drop', paths: ['/a.mp4', '/b.mp4'] } });
      });
      expect(onDrop).toHaveBeenCalledWith('/a.mp4');
      expect(onIgnored).toHaveBeenCalled();
    });
  });
  ```

- [ ] **Step 2: Run the test to verify it fails**

  Run: `npm --prefix app run test -- useDropTarget.test --run`
  Expected: FAIL — `Cannot find module './useDropTarget'`.

- [ ] **Step 3: Write the hook**

  ```ts
  import { useEffect, useState } from 'react';
  import { getCurrentWebview } from '@tauri-apps/api/webview';

  export interface UseDropTargetOptions {
    onDrop: (path: string) => void;
    onMultiFileIgnored?: () => void;
  }

  export function useDropTarget(opts: UseDropTargetOptions) {
    const [isDragOver, setIsDragOver] = useState(false);

    useEffect(() => {
      let unsub: (() => void) | undefined;
      let active = true;
      getCurrentWebview()
        .onDragDropEvent((e: any) => {
          const t = e.payload?.type;
          if (t === 'enter' || t === 'over') {
            setIsDragOver(true);
          } else if (t === 'leave' || t === 'cancel') {
            setIsDragOver(false);
          } else if (t === 'drop') {
            setIsDragOver(false);
            const paths: string[] = e.payload?.paths ?? [];
            if (paths.length > 1) opts.onMultiFileIgnored?.();
            if (paths.length >= 1) opts.onDrop(paths[0]);
          }
        })
        .then((u: () => void) => {
          if (!active) u();
          else unsub = u;
        });
      return () => {
        active = false;
        if (unsub) unsub();
      };
    }, [opts]);

    return { isDragOver };
  }
  ```

- [ ] **Step 4: Run tests to verify they pass**

  Run: `npm --prefix app run test -- useDropTarget.test --run`
  Expected: PASS (3 cases).

- [ ] **Step 5: Commit**

  ```bash
  git add app/src/hooks/useDropTarget.ts app/src/hooks/useDropTarget.test.tsx
  git commit -m "[item 54] useDropTarget hook: Tauri OS drop with multi-file ignore"
  ```

### Task 7.3: Create `EmptyState.tsx` (drop-only, no Browse button)

**Files:**
- Create: `app/src/components/EmptyState.tsx`
- Create: `app/src/components/EmptyState.test.tsx`

- [ ] **Step 1: Write the failing test**

  ```tsx
  import { describe, it, expect, vi } from 'vitest';
  import { render, screen } from '@testing-library/react';
  import { EmptyState } from './EmptyState';

  describe('EmptyState', () => {
    it('renders the drop zone, supported-formats line, and privacy line', () => {
      render(<EmptyState onDrop={vi.fn()} isDragOver={false} />);
      expect(screen.getByText(/drag.*drop|drop a video/i)).toBeInTheDocument();
      expect(screen.getByText(/mp4|mov|webm/i)).toBeInTheDocument();
      expect(screen.getByText(/never leaves|stays on your device|local/i)).toBeInTheDocument();
    });

    it('does NOT render a Browse button (drop-only Phase 3)', () => {
      render(<EmptyState onDrop={vi.fn()} isDragOver={false} />);
      expect(screen.queryByRole('button', { name: /browse/i })).toBeNull();
    });

    it('applies drag-over visual state', () => {
      const { container } = render(<EmptyState onDrop={vi.fn()} isDragOver />);
      const el = container.firstChild as HTMLElement;
      expect(el.className).toMatch(/drag-?over|drop-?active/);
    });
  });
  ```

- [ ] **Step 2: Run the test to verify it fails**

  Run: `npm --prefix app run test -- EmptyState.test --run`
  Expected: FAIL.

- [ ] **Step 3: Implement**

  ```tsx
  import React from 'react';

  export interface EmptyStateProps {
    onDrop: (path: string) => void;
    isDragOver: boolean;
  }

  export function EmptyState({ isDragOver }: EmptyStateProps) {
    return (
      <div
        className={`empty-state${isDragOver ? ' drag-over' : ''}`}
        role="region"
        aria-label="Drop a video file to begin"
      >
        <div className="empty-state__icon" aria-hidden>📼</div>
        <h2 className="empty-state__title">Drop a video here to begin</h2>
        <p className="empty-state__formats">Supports MP4, MOV, WebM, and MKV.</p>
        <p className="empty-state__privacy">
          Your files never leave your device — everything stays local.
        </p>
      </div>
    );
  }
  ```

  Add minimal CSS in the same file (or in a sibling `EmptyState.module.css` if the repo uses CSS modules; inspect a sibling component first).

- [ ] **Step 4: Run tests to verify they pass**

  Run: `npm --prefix app run test -- EmptyState.test --run`
  Expected: PASS.

- [ ] **Step 5: Commit**

  ```bash
  git add app/src/components/EmptyState.tsx app/src/components/EmptyState.test.tsx
  git commit -m "[item 55] Add EmptyState drop zone (drop-only, no Browse button in Phase 3)"
  ```

### Task 7.4: Create `ActiveFileCard.tsx` (status indicator + metadata summary)

**Files:**
- Create: `app/src/components/ActiveFileCard.tsx`
- Create: `app/src/components/ActiveFileCard.test.tsx`

- [ ] **Step 1: Write the failing test**

  ```tsx
  import { describe, it, expect, beforeEach } from 'vitest';
  import { render, screen } from '@testing-library/react';
  import { useWorkspace, resetWorkspaceForTest } from '@/state/workspace';
  import { ActiveFileCard } from './ActiveFileCard';

  describe('ActiveFileCard', () => {
    beforeEach(() => resetWorkspaceForTest());

    it('renders filename and probing dot during PROBING', () => {
      useWorkspace.setState({ status: 'PROBING', path: '/x/hello.mp4' });
      render(<ActiveFileCard />);
      expect(screen.getByText('hello.mp4')).toBeInTheDocument();
      expect(screen.getByTestId('status-dot').className).toMatch(/spinner|blue/);
    });

    it('renders green dot + metadata when READY and supported', () => {
      useWorkspace.setState({
        status: 'READY', path: '/x/hello.mp4',
        result: {
          id: 'A', path: '/x/hello.mp4', filename: 'hello.mp4', sizeBytes: 1024,
          durationSeconds: 5,
          container: { format: 'mp4', longName: 'MP4' },
          video: { codec: 'h264', width: 1280, height: 720, fps: 30 },
          audio: {
            codec: 'aac', channels: 2, sampleRate: 48000, trackIndex: 0, trackCount: 1,
            tracks: [{ index: 0, codec: 'aac', channels: 2, sampleRate: 48000, isDefault: true }],
          },
          compatibility: { supported: true, issues: [], warnings: [] },
        } as any,
      });
      render(<ActiveFileCard />);
      expect(screen.getByText(/h264/)).toBeInTheDocument();
      expect(screen.getByText(/aac/)).toBeInTheDocument();
      expect(screen.getByTestId('status-dot').className).toMatch(/green|ready/);
    });

    it('renders yellow dot when READY but supported=false', () => {
      useWorkspace.setState({
        status: 'READY', path: '/x/hello.wmv',
        result: {
          id: 'A', path: '/x/hello.wmv', filename: 'hello.wmv', sizeBytes: 1,
          container: { format: 'asf', longName: 'ASF' },
          audio: null,
          compatibility: { supported: false, issues: ['Unsupported container: asf'], warnings: [] },
        } as any,
      });
      render(<ActiveFileCard />);
      expect(screen.getByTestId('status-dot').className).toMatch(/yellow|unsupported/);
    });

    it('renders red dot when ERROR', () => {
      useWorkspace.setState({
        status: 'ERROR', path: '/x/missing.mp4',
        error: { code: 'FILE_NOT_FOUND', message: 'missing' },
      });
      render(<ActiveFileCard />);
      expect(screen.getByTestId('status-dot').className).toMatch(/red|error/);
    });

    it('Retry button is shown only in ERROR', () => {
      useWorkspace.setState({ status: 'READY', path: '/x.mp4' });
      const { rerender } = render(<ActiveFileCard />);
      expect(screen.queryByRole('button', { name: /retry/i })).toBeNull();
      useWorkspace.setState({ status: 'ERROR', error: { code: 'X', message: '' } });
      rerender(<ActiveFileCard />);
      expect(screen.getByRole('button', { name: /retry/i })).toBeInTheDocument();
    });

    it('Remove button clears workspace', () => {
      useWorkspace.setState({ status: 'READY', path: '/x.mp4' });
      render(<ActiveFileCard />);
      const btn = screen.getByRole('button', { name: /remove/i });
      btn.click();
      expect(useWorkspace.getState().status).toBe('EMPTY');
    });
  });
  ```

- [ ] **Step 2: Run the test to verify it fails**

  Run: `npm --prefix app run test -- ActiveFileCard.test --run`
  Expected: FAIL.

- [ ] **Step 3: Implement**

  ```tsx
  import React from 'react';
  import { useWorkspace } from '@/state/workspace';

  function dotClassFor(status: string, supported?: boolean): string {
      switch (status) {
        case 'PROBING':
        case 'RETRYING':
        case 'FILE_LOADED':
          return 'status-dot spinner';
        case 'READY':
          return supported ? 'status-dot green' : 'status-dot yellow';
        case 'ERROR':
          return 'status-dot red';
        default:
          return 'status-dot gray';
      }
  }

  function basename(path?: string): string {
    if (!path) return '';
    const i = Math.max(path.lastIndexOf('/'), path.lastIndexOf('\\'));
    return i >= 0 ? path.slice(i + 1) : path;
  }

  function formatBytes(n: number): string {
    if (n < 1024) return `${n} B`;
    if (n < 1024 * 1024) return `${(n/1024).toFixed(1)} KB`;
    return `${(n/1024/1024).toFixed(1)} MB`;
  }

  function formatDuration(s?: number): string {
    if (s == null) return 'Unknown duration';
    const mm = Math.floor(s / 60);
    const ss = Math.floor(s % 60).toString().padStart(2, '0');
    return `${mm}:${ss}`;
  }

  export function ActiveFileCard() {
    const status = useWorkspace((s) => s.status);
    const path = useWorkspace((s) => s.path);
    const result = useWorkspace((s) => s.result);
    const clearFile = useWorkspace((s) => s.clearFile);
    const retry = useWorkspace((s) => s.retry);

    const supported = result?.compatibility?.supported;

    return (
      <section className="active-file-card" aria-label="Active file">
        <div className="active-file-card__header">
          <div className="active-file-card__thumb" aria-hidden>🎬</div>
          <div className="active-file-card__title">
            <div className="active-file-card__filename">{basename(path)}</div>
            <div className="active-file-card__status">
              <span data-testid="status-dot" className={dotClassFor(status, supported)} />
              <span className="active-file-card__status-label">{statusLabel(status, supported)}</span>
            </div>
          </div>
        </div>
        {result && (
          <dl className="active-file-card__meta">
            <div><dt>Duration</dt><dd>{formatDuration(result.durationSeconds ?? undefined)}</dd></div>
            <div><dt>Size</dt><dd>{formatBytes(result.sizeBytes)}</dd></div>
            <div><dt>Container</dt><dd>{result.container.format}</dd></div>
            {result.video && (
              <div><dt>Video</dt><dd>{result.video.codec} {result.video.width}×{result.video.height} @ {result.video.fps.toFixed(2)}fps</dd></div>
            )}
            {result.audio && (
              <div><dt>Audio</dt><dd>{result.audio.codec} {result.audio.channels}ch {result.audio.sampleRate}Hz</dd></div>
            )}
          </dl>
        )}
        <div className="active-file-card__actions">
          {status === 'ERROR' && (
            <button type="button" onClick={() => void retry()}>Retry</button>
          )}
          <button type="button" onClick={clearFile}>Remove</button>
        </div>
      </section>
    );
  }

  function statusLabel(status: string, supported?: boolean): string {
    switch (status) {
      case 'PROBING': return 'Probing…';
      case 'RETRYING': return 'Retrying…';
      case 'FILE_LOADED': return 'Loading…';
      case 'READY': return supported ? 'Ready' : 'Unsupported';
      case 'ERROR': return 'Error';
      default: return '';
    }
  }
  ```

  In PR 8 we add Details + Replace controls; for now Retry + Remove are enough.

- [ ] **Step 4: Run tests to verify they pass**

  Run: `npm --prefix app run test -- ActiveFileCard.test --run`
  Expected: PASS.

- [ ] **Step 5: Commit**

  ```bash
  git add app/src/components/ActiveFileCard.tsx app/src/components/ActiveFileCard.test.tsx
  git commit -m "[item 56] ActiveFileCard: status indicator, metadata, Retry/Remove (PR 7 minimal)"
  ```

### Task 7.5: Create `WorkspaceShell.tsx`

**Files:**
- Create: `app/src/components/WorkspaceShell.tsx`
- Create: `app/src/components/WorkspaceShell.test.tsx`

- [ ] **Step 1: Write the failing test**

  ```tsx
  import { describe, it, expect, beforeEach, vi } from 'vitest';
  import { render, screen } from '@testing-library/react';

  vi.mock('@/ipc/client', () => ({ probe: vi.fn() }));
  vi.mock('@tauri-apps/api/webview', () => ({
    getCurrentWebview: () => ({ onDragDropEvent: () => Promise.resolve(() => {}) }),
  }));

  import { resetWorkspaceForTest, useWorkspace } from '@/state/workspace';
  import { WorkspaceShell } from './WorkspaceShell';

  beforeEach(() => resetWorkspaceForTest());

  describe('WorkspaceShell', () => {
    it('shows EmptyState when status is EMPTY', () => {
      render(<WorkspaceShell />);
      expect(screen.getByText(/drop a video here/i)).toBeInTheDocument();
    });

    it('shows ActiveFileCard when a file is loaded', () => {
      useWorkspace.setState({ status: 'READY', path: '/x/hello.mp4' });
      render(<WorkspaceShell />);
      expect(screen.getByText('hello.mp4')).toBeInTheDocument();
    });
  });
  ```

- [ ] **Step 2: Run the test to verify it fails**

  Run: `npm --prefix app run test -- WorkspaceShell.test --run`
  Expected: FAIL.

- [ ] **Step 3: Implement**

  ```tsx
  import React from 'react';
  import { useWorkspace } from '@/state/workspace';
  import { useDropTarget } from '@/hooks/useDropTarget';
  import { EmptyState } from './EmptyState';
  import { ActiveFileCard } from './ActiveFileCard';

  export function WorkspaceShell() {
    const status = useWorkspace((s) => s.status);
    const loadFile = useWorkspace((s) => s.loadFile);
    const { isDragOver } = useDropTarget({
      onDrop: (path) => {
        if (status === 'EMPTY') {
          void loadFile(path);
        }
        // PR 8 wires the ReplaceFileDialog for non-EMPTY drops.
      },
    });

    return (
      <div className="workspace-shell">
        <header className="workspace-shell__header">Studio Sound</header>
        <main className="workspace-shell__main">
          {status === 'EMPTY' ? (
            <EmptyState onDrop={loadFile} isDragOver={isDragOver} />
          ) : (
            <ActiveFileCard />
          )}
        </main>
        <footer className="workspace-shell__footer" />
      </div>
    );
  }
  ```

- [ ] **Step 4: Run tests to verify they pass**

  Run: `npm --prefix app run test -- WorkspaceShell.test --run`
  Expected: PASS.

- [ ] **Step 5: Commit**

  ```bash
  git add app/src/components/WorkspaceShell.tsx app/src/components/WorkspaceShell.test.tsx
  git commit -m "[item 57] WorkspaceShell layout + drop handling (EMPTY-only in PR 7)"
  ```

### Task 7.6: Replace `App.tsx` body with `<WorkspaceShell />`

**Files:**
- Modify: `app/src/App.tsx:27-38`

- [ ] **Step 1: Replace the placeholder body**

  In `App.tsx`, replace the existing JSX body (preserving the dev-only `Diagnostics` Ctrl+Shift+D mount) with:

  ```tsx
  import { WorkspaceShell } from './components/WorkspaceShell';
  // existing Diagnostics import unchanged

  function App() {
    return (
      <>
        <WorkspaceShell />
        {/* existing Diagnostics dev overlay unchanged */}
      </>
    );
  }
  export default App;
  ```

- [ ] **Step 2: Verify typecheck + build**

  Run: `npm --prefix app run typecheck && npm --prefix app run build`
  Expected: exit 0 for both.

- [ ] **Step 3: Commit**

  ```bash
  git add app/src/App.tsx
  git commit -m "[item 58] Replace App.tsx placeholder with WorkspaceShell (Diagnostics preserved)"
  ```

### Task 7.7: Manual UI smoke test (golden path)

**Files:** (no file change)

- [ ] **Step 1: Run the dev shell**

  Run: `cd app && npm run tauri dev`
  Expected: window opens; drop zone visible.

- [ ] **Step 2: Drop a fixture file**

  Drop `test-assets/tiny-h264-aac-stereo.mp4` onto the window.
  Expected: card transitions: probing dot → green dot; metadata shows h264/aac/duration.

- [ ] **Step 3: Drop an unsupported fixture**

  Drop `test-assets/tiny-no-audio.mp4`.

  Actually — the workspace is not EMPTY anymore; this drop is silently ignored in PR 7. Use the Remove button first, then drop again. Expected: yellow dot + "Unsupported" label.

- [ ] **Step 4: Drop a corrupt fixture**

  Drop `test-assets/corrupt-truncated.mp4` (after Remove).
  Expected: red dot + "Error" label; Retry button visible.

- [ ] **Step 5: Item 59 complete — no commit**

  If any step fails, halt and debug before continuing.

### Task 7.8: Full test sweep for PR 7

**Files:** (no file change)

- [ ] **Step 1: Frontend tests**

  Run: `npm --prefix app run test -- --run`
  Expected: all pass.

- [ ] **Step 2: Typecheck**

  Run: `npm --prefix app run typecheck`
  Expected: exit 0.

- [ ] **Step 3: Item 60 complete**

### Task 7.9: Item 61 — PR 7 ready for review (no further commits)

- [ ] **Step 1: Confirm branch state**

  Run: `git log --oneline feature/phase-3-pr6..HEAD`
  Expected: ~8 commits matching items 53–58.

---

## PR 8: Workspace UI secondary surfaces (DiagnosticsDrawer + ReplaceFileDialog + ErrorPanel + UnsupportedPanel)

**Branch:** `feature/phase-3-pr8`
**Base:** `feature/phase-3-pr7`
**Depends on:** PR 7
**Items:** 62–69
**Goal:** Add the secondary panels and dialogs that hang off the workspace shell — the right-side Diagnostics drawer, the Replace-file confirmation dialog (shown when a drop arrives while a file is already loaded), the structured `ErrorPanel` for RPC-error verdicts, and the `UnsupportedPanel` for the hybrid success-with-supported=false case. Wire `ActiveFileCard` and `WorkspaceShell` to use them.

This PR finishes PRD §8.5 (error UI) and §8.6 (replace flow) and closes out Phase 3.

### Task 8.1: Item 62 — `DiagnosticsDrawer` component

**Files:**
- Create: `app/src/workspace/components/DiagnosticsDrawer.tsx` [new]
- Create: `app/src/workspace/components/DiagnosticsDrawer.test.tsx` [new]

- [ ] **Step 1: Write the failing test**

  Create `app/src/workspace/components/DiagnosticsDrawer.test.tsx`:

  ```tsx
  import { fireEvent, render, screen, waitFor } from '@testing-library/react';
  import { describe, expect, it, vi, beforeEach } from 'vitest';
  import { DiagnosticsDrawer } from './DiagnosticsDrawer';

  const writeText = vi.fn().mockResolvedValue(undefined);
  Object.assign(navigator, { clipboard: { writeText } });

  const sample = {
    path: '/tmp/a.mp4',
    container: 'mov',
    durationMs: 12345,
    audio: { codec: 'aac', sampleRateHz: 48000, channelCount: 2 },
    video: { codec: 'h264', widthPx: 1920, heightPx: 1080, fps: 30 },
    compatibility: { supported: true, issues: [] },
    probeId: '01H...ULID',
  };

  describe('DiagnosticsDrawer', () => {
    beforeEach(() => writeText.mockClear());

    it('renders nothing when closed', () => {
      const { container } = render(
        <DiagnosticsDrawer open={false} onClose={() => {}} result={sample} />
      );
      expect(container.firstChild).toBeNull();
    });

    it('renders metadata sections when open', () => {
      render(<DiagnosticsDrawer open={true} onClose={() => {}} result={sample} />);
      expect(screen.getByText(/probeId/)).toBeInTheDocument();
      expect(screen.getByText(/01H\.\.\.ULID/)).toBeInTheDocument();
      expect(screen.getByText(/h264/)).toBeInTheDocument();
      expect(screen.getByText(/aac/)).toBeInTheDocument();
    });

    it('copies diagnostics JSON to the clipboard and shows a 2s toast', async () => {
      vi.useFakeTimers();
      render(<DiagnosticsDrawer open={true} onClose={() => {}} result={sample} />);
      fireEvent.click(screen.getByRole('button', { name: /copy diagnostics/i }));
      await waitFor(() => expect(writeText).toHaveBeenCalledTimes(1));
      expect(JSON.parse(writeText.mock.calls[0][0])).toMatchObject({ path: '/tmp/a.mp4' });
      expect(screen.getByText(/copied/i)).toBeInTheDocument();
      vi.advanceTimersByTime(2100);
      await waitFor(() => expect(screen.queryByText(/copied/i)).toBeNull());
      vi.useRealTimers();
    });

    it('invokes onClose on Esc and on backdrop click and on the close button', () => {
      const onClose = vi.fn();
      render(<DiagnosticsDrawer open={true} onClose={onClose} result={sample} />);
      fireEvent.keyDown(window, { key: 'Escape' });
      fireEvent.click(screen.getByTestId('diagnostics-backdrop'));
      fireEvent.click(screen.getByRole('button', { name: /close diagnostics/i }));
      expect(onClose).toHaveBeenCalledTimes(3);
    });
  });
  ```

- [ ] **Step 2: Run test to verify it fails**

  Run: `npm --prefix app run test -- --run DiagnosticsDrawer`
  Expected: FAIL — file not found / component not exported.

- [ ] **Step 3: Write minimal implementation**

  Create `app/src/workspace/components/DiagnosticsDrawer.tsx`:

  ```tsx
  import { useEffect, useState } from 'react';
  import type { ProbeResult } from '../../ipc/probe';

  interface Props {
    open: boolean;
    onClose: () => void;
    result: ProbeResult;
  }

  export function DiagnosticsDrawer({ open, onClose, result }: Props) {
    const [copied, setCopied] = useState(false);

    useEffect(() => {
      if (!open) return;
      const handler = (e: KeyboardEvent) => {
        if (e.key === 'Escape') onClose();
      };
      window.addEventListener('keydown', handler);
      return () => window.removeEventListener('keydown', handler);
    }, [open, onClose]);

    useEffect(() => {
      if (!copied) return;
      const t = setTimeout(() => setCopied(false), 2000);
      return () => clearTimeout(t);
    }, [copied]);

    if (!open) return null;

    const handleCopy = async () => {
      await navigator.clipboard.writeText(JSON.stringify(result, null, 2));
      setCopied(true);
    };

    return (
      <div className="diagnostics-overlay">
        <div
          data-testid="diagnostics-backdrop"
          className="diagnostics-backdrop"
          onClick={onClose}
        />
        <aside className="diagnostics-drawer" role="dialog" aria-label="Diagnostics">
          <header className="diagnostics-drawer__header">
            <h2>Diagnostics</h2>
            <button onClick={onClose} aria-label="Close diagnostics">
              ×
            </button>
          </header>
          <section className="diagnostics-drawer__body">
            <dl>
              <dt>probeId</dt>
              <dd>{result.probeId}</dd>
              <dt>path</dt>
              <dd>{result.path}</dd>
              <dt>container</dt>
              <dd>{result.container ?? '—'}</dd>
              <dt>durationMs</dt>
              <dd>{result.durationMs ?? '—'}</dd>
              <dt>audio</dt>
              <dd>
                {result.audio
                  ? `${result.audio.codec} · ${result.audio.sampleRateHz ?? '?'}Hz · ${
                      result.audio.channelCount ?? '?'
                    }ch`
                  : '—'}
              </dd>
              <dt>video</dt>
              <dd>
                {result.video
                  ? `${result.video.codec} · ${result.video.widthPx ?? '?'}×${
                      result.video.heightPx ?? '?'
                    } · ${result.video.fps ?? '?'}fps`
                  : '—'}
              </dd>
              <dt>compatibility</dt>
              <dd>
                {result.compatibility.supported
                  ? 'supported'
                  : `unsupported: ${result.compatibility.issues
                      .map((i) => i.code)
                      .join(', ')}`}
              </dd>
            </dl>
          </section>
          <footer className="diagnostics-drawer__footer">
            <button onClick={handleCopy}>Copy diagnostics</button>
            {copied && <span role="status">Copied</span>}
          </footer>
        </aside>
      </div>
    );
  }
  ```

- [ ] **Step 4: Run test to verify it passes**

  Run: `npm --prefix app run test -- --run DiagnosticsDrawer`
  Expected: PASS.

- [ ] **Step 5: Commit**

  ```bash
  git add app/src/workspace/components/DiagnosticsDrawer.tsx app/src/workspace/components/DiagnosticsDrawer.test.tsx
  git commit -m "[item 62] Add DiagnosticsDrawer with copy-to-clipboard toast"
  ```

### Task 8.2: Item 63 — `ReplaceFileDialog` component

**Files:**
- Create: `app/src/workspace/components/ReplaceFileDialog.tsx` [new]
- Create: `app/src/workspace/components/ReplaceFileDialog.test.tsx` [new]

- [ ] **Step 1: Write the failing test**

  Create `app/src/workspace/components/ReplaceFileDialog.test.tsx`:

  ```tsx
  import { fireEvent, render, screen } from '@testing-library/react';
  import { describe, expect, it, vi } from 'vitest';
  import { ReplaceFileDialog } from './ReplaceFileDialog';

  describe('ReplaceFileDialog', () => {
    it('renders nothing when no incoming path', () => {
      const { container } = render(
        <ReplaceFileDialog
          incomingPath={null}
          currentFilename="old.mp4"
          onConfirm={() => {}}
          onCancel={() => {}}
        />
      );
      expect(container.firstChild).toBeNull();
    });

    it('shows incoming filename + current filename', () => {
      render(
        <ReplaceFileDialog
          incomingPath="/tmp/new.mp4"
          currentFilename="old.mp4"
          onConfirm={() => {}}
          onCancel={() => {}}
        />
      );
      expect(screen.getByText(/new\.mp4/)).toBeInTheDocument();
      expect(screen.getByText(/old\.mp4/)).toBeInTheDocument();
    });

    it('invokes onConfirm with the incoming path on Replace and onCancel otherwise', () => {
      const onConfirm = vi.fn();
      const onCancel = vi.fn();
      render(
        <ReplaceFileDialog
          incomingPath="/tmp/new.mp4"
          currentFilename="old.mp4"
          onConfirm={onConfirm}
          onCancel={onCancel}
        />
      );
      fireEvent.click(screen.getByRole('button', { name: /replace/i }));
      expect(onConfirm).toHaveBeenCalledWith('/tmp/new.mp4');
      fireEvent.click(screen.getByRole('button', { name: /cancel/i }));
      expect(onCancel).toHaveBeenCalledTimes(1);
      fireEvent.keyDown(window, { key: 'Escape' });
      expect(onCancel).toHaveBeenCalledTimes(2);
    });
  });
  ```

- [ ] **Step 2: Run test to verify it fails**

  Run: `npm --prefix app run test -- --run ReplaceFileDialog`
  Expected: FAIL — component not found.

- [ ] **Step 3: Write minimal implementation**

  Create `app/src/workspace/components/ReplaceFileDialog.tsx`:

  ```tsx
  import { useEffect } from 'react';

  interface Props {
    incomingPath: string | null;
    currentFilename: string | null;
    onConfirm: (path: string) => void;
    onCancel: () => void;
  }

  function basename(p: string): string {
    const m = p.match(/[^/\\]+$/);
    return m ? m[0] : p;
  }

  export function ReplaceFileDialog({ incomingPath, currentFilename, onConfirm, onCancel }: Props) {
    useEffect(() => {
      if (!incomingPath) return;
      const handler = (e: KeyboardEvent) => {
        if (e.key === 'Escape') onCancel();
      };
      window.addEventListener('keydown', handler);
      return () => window.removeEventListener('keydown', handler);
    }, [incomingPath, onCancel]);

    if (!incomingPath) return null;

    return (
      <div className="dialog-overlay">
        <div className="dialog-backdrop" onClick={onCancel} />
        <div className="dialog" role="dialog" aria-modal="true" aria-labelledby="replace-title">
          <h2 id="replace-title">Replace current file?</h2>
          <p>
            Replace <strong>{currentFilename ?? 'current file'}</strong> with{' '}
            <strong>{basename(incomingPath)}</strong>?
          </p>
          <div className="dialog__actions">
            <button onClick={onCancel}>Cancel</button>
            <button onClick={() => onConfirm(incomingPath)} className="primary">
              Replace
            </button>
          </div>
        </div>
      </div>
    );
  }
  ```

- [ ] **Step 4: Run test to verify it passes**

  Run: `npm --prefix app run test -- --run ReplaceFileDialog`
  Expected: PASS.

- [ ] **Step 5: Commit**

  ```bash
  git add app/src/workspace/components/ReplaceFileDialog.tsx app/src/workspace/components/ReplaceFileDialog.test.tsx
  git commit -m "[item 63] Add ReplaceFileDialog modal for non-EMPTY drop confirmation"
  ```

### Task 8.3: Item 64 — `ErrorPanel` component (one component, branches on code)

**Files:**
- Create: `app/src/workspace/components/ErrorPanel.tsx` [new]
- Create: `app/src/workspace/components/ErrorPanel.test.tsx` [new]

The panel handles every RPC error code from PR 1: `FILE_NOT_FOUND`, `ACCESS_DENIED`, `CORRUPT_MEDIA`, `FFPROBE_FAILURE`, `FFPROBE_MISSING`. Each branch shows the PRD §8.5 copy and the affordances (Retry / Open Details).

- [ ] **Step 1: Write the failing test**

  Create `app/src/workspace/components/ErrorPanel.test.tsx`:

  ```tsx
  import { fireEvent, render, screen } from '@testing-library/react';
  import { describe, expect, it, vi } from 'vitest';
  import { ErrorPanel } from './ErrorPanel';

  function panel(code: string) {
    return {
      code,
      message: `msg for ${code}`,
      details: code === 'FFPROBE_FAILURE' ? { exitCode: 1, stderrTail: 'bad' } : undefined,
    };
  }

  describe('ErrorPanel', () => {
    it.each([
      ['FILE_NOT_FOUND', /file not found/i],
      ['ACCESS_DENIED', /can.?t read/i],
      ['CORRUPT_MEDIA', /corrupt|unreadable/i],
      ['FFPROBE_FAILURE', /couldn.?t analyze|ffprobe/i],
      ['FFPROBE_MISSING', /ffprobe.*missing|reinstall/i],
    ])('renders headline for %s', (code, headline) => {
      render(
        <ErrorPanel error={panel(code)} onRetry={() => {}} onOpenDetails={() => {}} />
      );
      expect(screen.getByRole('heading')).toHaveTextContent(headline);
    });

    it('hides Retry for FFPROBE_MISSING (terminal: not user-recoverable)', () => {
      render(
        <ErrorPanel error={panel('FFPROBE_MISSING')} onRetry={() => {}} onOpenDetails={() => {}} />
      );
      expect(screen.queryByRole('button', { name: /retry/i })).toBeNull();
    });

    it('invokes onRetry and onOpenDetails', () => {
      const onRetry = vi.fn();
      const onOpenDetails = vi.fn();
      render(
        <ErrorPanel error={panel('FFPROBE_FAILURE')} onRetry={onRetry} onOpenDetails={onOpenDetails} />
      );
      fireEvent.click(screen.getByRole('button', { name: /retry/i }));
      fireEvent.click(screen.getByRole('button', { name: /details/i }));
      expect(onRetry).toHaveBeenCalledTimes(1);
      expect(onOpenDetails).toHaveBeenCalledTimes(1);
    });

    it('falls back to a generic headline for unknown codes', () => {
      render(
        <ErrorPanel
          error={{ code: 'UNKNOWN_CODE', message: 'x' }}
          onRetry={() => {}}
          onOpenDetails={() => {}}
        />
      );
      expect(screen.getByRole('heading')).toHaveTextContent(/something went wrong/i);
    });
  });
  ```

- [ ] **Step 2: Run test to verify it fails**

  Run: `npm --prefix app run test -- --run ErrorPanel`
  Expected: FAIL.

- [ ] **Step 3: Write minimal implementation**

  Create `app/src/workspace/components/ErrorPanel.tsx`:

  ```tsx
  import type { ProbeError } from '../../ipc/probe';

  interface Props {
    error: ProbeError;
    onRetry: () => void;
    onOpenDetails: () => void;
  }

  interface Copy {
    headline: string;
    body: string;
    retry: boolean;
  }

  function copyFor(code: string): Copy {
    switch (code) {
      case 'FILE_NOT_FOUND':
        return {
          headline: 'File not found',
          body: 'The file you dropped is no longer at that location. It may have been moved or deleted.',
          retry: true,
        };
      case 'ACCESS_DENIED':
        return {
          headline: "Can't read this file",
          body: 'Studio Sound does not have permission to read this file. Check the file permissions and try again.',
          retry: true,
        };
      case 'CORRUPT_MEDIA':
        return {
          headline: 'Corrupt or unreadable media',
          body: 'This file appears to be truncated or corrupt. Try re-exporting or re-downloading it.',
          retry: true,
        };
      case 'FFPROBE_FAILURE':
        return {
          headline: "Couldn't analyze this file",
          body: 'ffprobe exited with an error. Open Details to see the stderr tail.',
          retry: true,
        };
      case 'FFPROBE_MISSING':
        return {
          headline: 'ffprobe is missing',
          body: 'The bundled ffprobe binary could not be located. Reinstall Studio Sound.',
          retry: false,
        };
      default:
        return {
          headline: 'Something went wrong',
          body: 'An unexpected error occurred while analyzing this file.',
          retry: true,
        };
    }
  }

  export function ErrorPanel({ error, onRetry, onOpenDetails }: Props) {
    const c = copyFor(error.code);
    return (
      <section className="error-panel" role="region" aria-label="Error">
        <h2>{c.headline}</h2>
        <p>{c.body}</p>
        <div className="error-panel__actions">
          {c.retry && <button onClick={onRetry}>Retry</button>}
          <button onClick={onOpenDetails}>Details</button>
        </div>
      </section>
    );
  }
  ```

- [ ] **Step 4: Run test to verify it passes**

  Run: `npm --prefix app run test -- --run ErrorPanel`
  Expected: PASS.

- [ ] **Step 5: Commit**

  ```bash
  git add app/src/workspace/components/ErrorPanel.tsx app/src/workspace/components/ErrorPanel.test.tsx
  git commit -m "[item 64] Add ErrorPanel branching on ProbeError code"
  ```

### Task 8.4: Item 65 — `UnsupportedPanel` component (success-with-supported=false)

**Files:**
- Create: `app/src/workspace/components/UnsupportedPanel.tsx` [new]
- Create: `app/src/workspace/components/UnsupportedPanel.test.tsx` [new]

This panel is NOT triggered by an RPC error. It's shown when `result.compatibility.supported === false`. Per the hybrid model, the probe succeeded; the issues list explains why the file is unsupported.

- [ ] **Step 1: Write the failing test**

  Create `app/src/workspace/components/UnsupportedPanel.test.tsx`:

  ```tsx
  import { fireEvent, render, screen } from '@testing-library/react';
  import { describe, expect, it, vi } from 'vitest';
  import { UnsupportedPanel } from './UnsupportedPanel';

  describe('UnsupportedPanel', () => {
    it('lists each compatibility issue with code + message', () => {
      render(
        <UnsupportedPanel
          issues={[
            { code: 'NO_AUDIO_STREAM', message: 'File has no audio streams' },
            { code: 'UNSUPPORTED_CODEC', message: 'opus is not supported in this build' },
          ]}
          onReplace={() => {}}
          onOpenDetails={() => {}}
        />
      );
      expect(screen.getByText(/no audio streams/i)).toBeInTheDocument();
      expect(screen.getByText(/opus is not supported/i)).toBeInTheDocument();
      expect(screen.getByText(/NO_AUDIO_STREAM/)).toBeInTheDocument();
      expect(screen.getByText(/UNSUPPORTED_CODEC/)).toBeInTheDocument();
    });

    it('invokes onReplace and onOpenDetails', () => {
      const onReplace = vi.fn();
      const onOpenDetails = vi.fn();
      render(
        <UnsupportedPanel
          issues={[{ code: 'UNSUPPORTED_CONTAINER', message: 'wav containers are not supported' }]}
          onReplace={onReplace}
          onOpenDetails={onOpenDetails}
        />
      );
      fireEvent.click(screen.getByRole('button', { name: /replace.*file/i }));
      fireEvent.click(screen.getByRole('button', { name: /details/i }));
      expect(onReplace).toHaveBeenCalledTimes(1);
      expect(onOpenDetails).toHaveBeenCalledTimes(1);
    });

    it('renders fallback copy when issues is empty (defensive)', () => {
      render(<UnsupportedPanel issues={[]} onReplace={() => {}} onOpenDetails={() => {}} />);
      expect(screen.getByRole('heading')).toHaveTextContent(/unsupported/i);
    });
  });
  ```

- [ ] **Step 2: Run test to verify it fails**

  Run: `npm --prefix app run test -- --run UnsupportedPanel`
  Expected: FAIL.

- [ ] **Step 3: Write minimal implementation**

  Create `app/src/workspace/components/UnsupportedPanel.tsx`:

  ```tsx
  import type { CompatibilityIssue } from '../../ipc/probe';

  interface Props {
    issues: CompatibilityIssue[];
    onReplace: () => void;
    onOpenDetails: () => void;
  }

  export function UnsupportedPanel({ issues, onReplace, onOpenDetails }: Props) {
    return (
      <section className="unsupported-panel" role="region" aria-label="Unsupported file">
        <h2>Unsupported file</h2>
        <p>Studio Sound can read this file's metadata, but cannot process it for editing.</p>
        {issues.length > 0 && (
          <ul className="unsupported-panel__issues">
            {issues.map((issue) => (
              <li key={issue.code}>
                <code>{issue.code}</code>
                <span>{issue.message}</span>
              </li>
            ))}
          </ul>
        )}
        <div className="unsupported-panel__actions">
          <button onClick={onReplace} className="primary">
            Replace file
          </button>
          <button onClick={onOpenDetails}>Details</button>
        </div>
      </section>
    );
  }
  ```

- [ ] **Step 4: Run test to verify it passes**

  Run: `npm --prefix app run test -- --run UnsupportedPanel`
  Expected: PASS.

- [ ] **Step 5: Commit**

  ```bash
  git add app/src/workspace/components/UnsupportedPanel.tsx app/src/workspace/components/UnsupportedPanel.test.tsx
  git commit -m "[item 65] Add UnsupportedPanel for hybrid success-with-supported=false verdicts"
  ```

### Task 8.5: Item 66 — Wire `ActiveFileCard` to branch on verdict and open Diagnostics

**Files:**
- Modify: `app/src/workspace/components/ActiveFileCard.tsx`
- Modify: `app/src/workspace/components/ActiveFileCard.test.tsx`

After PR 7, `ActiveFileCard` showed metadata for READY and a Retry button for ERROR but did not mount `ErrorPanel`, `UnsupportedPanel`, or `DiagnosticsDrawer`. This item wires those three together.

- [ ] **Step 1: Extend the existing test**

  Append cases to `app/src/workspace/components/ActiveFileCard.test.tsx`:

  ```tsx
  // existing imports already present:
  // import { ActiveFileCard } from './ActiveFileCard';
  // import { useWorkspaceStore } from '../store/workspace';

  it('renders ErrorPanel when state is ERROR', () => {
    useWorkspaceStore.setState({
      state: 'ERROR',
      path: '/tmp/x.mp4',
      filename: 'x.mp4',
      result: null,
      error: { code: 'FFPROBE_FAILURE', message: 'failed' },
    });
    render(<ActiveFileCard />);
    expect(screen.getByRole('region', { name: /error/i })).toBeInTheDocument();
  });

  it('renders UnsupportedPanel when result.compatibility.supported is false', () => {
    useWorkspaceStore.setState({
      state: 'READY',
      path: '/tmp/x.mp4',
      filename: 'x.mp4',
      error: null,
      result: {
        path: '/tmp/x.mp4',
        container: 'wav',
        durationMs: 1000,
        audio: null,
        video: null,
        compatibility: {
          supported: false,
          issues: [{ code: 'NO_AUDIO_STREAM', message: 'no audio' }],
        },
        probeId: 'p1',
      },
    });
    render(<ActiveFileCard />);
    expect(screen.getByRole('region', { name: /unsupported/i })).toBeInTheDocument();
  });

  it('opens DiagnosticsDrawer when the Details button is clicked', async () => {
    useWorkspaceStore.setState({
      state: 'READY',
      path: '/tmp/x.mp4',
      filename: 'x.mp4',
      error: null,
      result: {
        path: '/tmp/x.mp4',
        container: 'mov',
        durationMs: 1000,
        audio: { codec: 'aac', sampleRateHz: 48000, channelCount: 2 },
        video: null,
        compatibility: { supported: true, issues: [] },
        probeId: 'p1',
      },
    });
    render(<ActiveFileCard />);
    expect(screen.queryByRole('dialog', { name: /diagnostics/i })).toBeNull();
    fireEvent.click(screen.getByRole('button', { name: /details/i }));
    expect(screen.getByRole('dialog', { name: /diagnostics/i })).toBeInTheDocument();
  });
  ```

- [ ] **Step 2: Run tests to verify they fail**

  Run: `npm --prefix app run test -- --run ActiveFileCard`
  Expected: FAIL on the three new cases.

- [ ] **Step 3: Update the component**

  Edit `app/src/workspace/components/ActiveFileCard.tsx` to add panel branching and the diagnostics drawer. The structure:

  ```tsx
  import { useState } from 'react';
  import { useWorkspaceStore } from '../store/workspace';
  import { DiagnosticsDrawer } from './DiagnosticsDrawer';
  import { ErrorPanel } from './ErrorPanel';
  import { UnsupportedPanel } from './UnsupportedPanel';

  export function ActiveFileCard() {
    const { state, filename, result, error, retry, clearFile } = useWorkspaceStore();
    const [diagOpen, setDiagOpen] = useState(false);

    if (state === 'EMPTY') return null;

    const showError = state === 'ERROR' && error;
    const showUnsupported = state === 'READY' && result && !result.compatibility.supported;
    const showReady = state === 'READY' && result && result.compatibility.supported;
    const probing = state === 'PROBING' || state === 'RETRYING';

    return (
      <>
        <article className="active-file-card">
          <header>
            <span
              className={`status-dot status-dot--${
                showError ? 'error' : showUnsupported ? 'warn' : probing ? 'probing' : 'ready'
              }`}
            />
            <span className="filename">{filename}</span>
            <div className="actions">
              {result && (
                <button onClick={() => setDiagOpen(true)} aria-label="Details">
                  Details
                </button>
              )}
              <button onClick={() => clearFile()} aria-label="Remove">
                Remove
              </button>
            </div>
          </header>

          {probing && <p>Analyzing…</p>}

          {showReady && (
            <dl className="metadata">
              <dt>Container</dt>
              <dd>{result.container ?? '—'}</dd>
              <dt>Duration</dt>
              <dd>{result.durationMs ? `${(result.durationMs / 1000).toFixed(2)}s` : '—'}</dd>
              <dt>Audio</dt>
              <dd>
                {result.audio
                  ? `${result.audio.codec} · ${result.audio.sampleRateHz}Hz · ${result.audio.channelCount}ch`
                  : '—'}
              </dd>
              <dt>Video</dt>
              <dd>
                {result.video
                  ? `${result.video.codec} · ${result.video.widthPx}×${result.video.heightPx}`
                  : '—'}
              </dd>
            </dl>
          )}

          {showError && (
            <ErrorPanel
              error={error}
              onRetry={() => retry()}
              onOpenDetails={() => setDiagOpen(true)}
            />
          )}

          {showUnsupported && (
            <UnsupportedPanel
              issues={result.compatibility.issues}
              onReplace={() => clearFile()}
              onOpenDetails={() => setDiagOpen(true)}
            />
          )}
        </article>

        {result && (
          <DiagnosticsDrawer open={diagOpen} onClose={() => setDiagOpen(false)} result={result} />
        )}
      </>
    );
  }
  ```

  Notes:
  - `UnsupportedPanel.onReplace` clears the workspace so the next drop is treated as a fresh load (and not as a replace-while-loaded — that flow is opened in PR 8 Task 8.6 for *new* drops, but the Replace button here is the explicit user gesture to clear).
  - The Diagnostics drawer is mounted at this level so both READY and ERROR branches can open it.

- [ ] **Step 4: Run tests to verify they pass**

  Run: `npm --prefix app run test -- --run ActiveFileCard`
  Expected: PASS (all old + new cases).

- [ ] **Step 5: Commit**

  ```bash
  git add app/src/workspace/components/ActiveFileCard.tsx app/src/workspace/components/ActiveFileCard.test.tsx
  git commit -m "[item 66] Wire ActiveFileCard to ErrorPanel, UnsupportedPanel, DiagnosticsDrawer"
  ```

### Task 8.6: Item 67 — Wire `WorkspaceShell` to show `ReplaceFileDialog` for non-EMPTY drops

**Files:**
- Modify: `app/src/workspace/components/WorkspaceShell.tsx`
- Modify: `app/src/workspace/components/WorkspaceShell.test.tsx`
- Modify: `app/src/workspace/hooks/useDropTarget.ts` (signature change — return the dropped path instead of auto-dispatching)

After PR 7, `useDropTarget` called `loadFile(path)` directly. To support the replace flow, the hook must surface the dropped path so `WorkspaceShell` can decide whether to `loadFile` (when EMPTY) or open the `ReplaceFileDialog` (when not EMPTY).

- [ ] **Step 1: Update `useDropTarget` test for the new signature**

  Edit `app/src/workspace/hooks/useDropTarget.test.ts`:

  - Replace assertions on `loadFile` being called with assertions that the `onPath` callback passed to the hook receives the dropped path.
  - Keep the multi-file ignore case (the hook should call `onPath` with the first path only, OR ignore — pick the LLD answer: **multi-file drops are ignored entirely in Phase 3**, so `onPath` is NOT called for multi-file payloads).

  Specifically:

  ```ts
  it('calls onPath with the dropped file path for single-file drops', () => {
    const onPath = vi.fn();
    renderHook(() => useDropTarget({ onPath }));
    simulateTauriDrop(['/tmp/a.mp4']);
    expect(onPath).toHaveBeenCalledWith('/tmp/a.mp4');
  });

  it('does not call onPath for multi-file drops', () => {
    const onPath = vi.fn();
    renderHook(() => useDropTarget({ onPath }));
    simulateTauriDrop(['/tmp/a.mp4', '/tmp/b.mp4']);
    expect(onPath).not.toHaveBeenCalled();
  });
  ```

- [ ] **Step 2: Run hook tests to confirm they fail**

  Run: `npm --prefix app run test -- --run useDropTarget`
  Expected: FAIL.

- [ ] **Step 3: Update `useDropTarget` to use the callback signature**

  Edit `app/src/workspace/hooks/useDropTarget.ts`:

  ```ts
  export interface UseDropTargetOpts {
    onPath: (path: string) => void;
  }

  export function useDropTarget({ onPath }: UseDropTargetOpts) {
    useEffect(() => {
      const unlisten = getCurrentWebview().onDragDropEvent((evt) => {
        if (evt.payload.type !== 'drop') return;
        const paths = evt.payload.paths;
        if (paths.length !== 1) return; // ignore multi-file drops in Phase 3
        onPath(paths[0]);
      });
      return () => {
        unlisten.then((fn) => fn());
      };
    }, [onPath]);
  }
  ```

- [ ] **Step 4: Run hook tests to confirm they pass**

  Run: `npm --prefix app run test -- --run useDropTarget`
  Expected: PASS.

- [ ] **Step 5: Update `WorkspaceShell` test**

  Edit `app/src/workspace/components/WorkspaceShell.test.tsx`:

  ```tsx
  it('calls loadFile when a drop arrives and workspace is EMPTY', () => {
    useWorkspaceStore.setState({ state: 'EMPTY', path: null, filename: null, result: null, error: null });
    render(<WorkspaceShell />);
    act(() => simulateTauriDrop(['/tmp/a.mp4']));
    expect(useWorkspaceStore.getState().state).toBe('PROBING');
    expect(useWorkspaceStore.getState().path).toBe('/tmp/a.mp4');
  });

  it('opens ReplaceFileDialog when a drop arrives and workspace is not EMPTY', () => {
    useWorkspaceStore.setState({
      state: 'READY',
      path: '/tmp/old.mp4',
      filename: 'old.mp4',
      result: { /* minimal valid result */ },
      error: null,
    });
    render(<WorkspaceShell />);
    act(() => simulateTauriDrop(['/tmp/new.mp4']));
    expect(screen.getByRole('dialog')).toHaveTextContent(/replace/i);
    expect(screen.getByText(/new\.mp4/)).toBeInTheDocument();
  });

  it('Replace button confirms replacement via replaceFile()', () => {
    const replaceFile = vi.fn();
    useWorkspaceStore.setState({
      state: 'READY',
      path: '/tmp/old.mp4',
      filename: 'old.mp4',
      result: { /* ... */ },
      error: null,
      replaceFile,
    } as never);
    render(<WorkspaceShell />);
    act(() => simulateTauriDrop(['/tmp/new.mp4']));
    fireEvent.click(screen.getByRole('button', { name: /replace/i }));
    expect(replaceFile).toHaveBeenCalledWith('/tmp/new.mp4');
    expect(screen.queryByRole('dialog')).toBeNull();
  });

  it('Cancel closes the dialog without changing state', () => {
    useWorkspaceStore.setState({
      state: 'READY',
      path: '/tmp/old.mp4',
      filename: 'old.mp4',
      result: { /* ... */ },
      error: null,
    });
    render(<WorkspaceShell />);
    act(() => simulateTauriDrop(['/tmp/new.mp4']));
    fireEvent.click(screen.getByRole('button', { name: /cancel/i }));
    expect(screen.queryByRole('dialog')).toBeNull();
    expect(useWorkspaceStore.getState().path).toBe('/tmp/old.mp4');
  });
  ```

- [ ] **Step 6: Run shell tests to confirm they fail**

  Run: `npm --prefix app run test -- --run WorkspaceShell`
  Expected: FAIL.

- [ ] **Step 7: Update `WorkspaceShell.tsx`**

  Edit `app/src/workspace/components/WorkspaceShell.tsx`:

  ```tsx
  import { useState } from 'react';
  import { useWorkspaceStore } from '../store/workspace';
  import { useDropTarget } from '../hooks/useDropTarget';
  import { EmptyState } from './EmptyState';
  import { ActiveFileCard } from './ActiveFileCard';
  import { ReplaceFileDialog } from './ReplaceFileDialog';

  export function WorkspaceShell() {
    const { state, filename, loadFile, replaceFile } = useWorkspaceStore();
    const [incoming, setIncoming] = useState<string | null>(null);

    useDropTarget({
      onPath: (path) => {
        if (state === 'EMPTY') {
          loadFile(path);
        } else {
          setIncoming(path);
        }
      },
    });

    return (
      <main className="workspace-shell">
        {state === 'EMPTY' ? <EmptyState /> : <ActiveFileCard />}
        <ReplaceFileDialog
          incomingPath={incoming}
          currentFilename={filename}
          onConfirm={(path) => {
            replaceFile(path);
            setIncoming(null);
          }}
          onCancel={() => setIncoming(null)}
        />
      </main>
    );
  }
  ```

- [ ] **Step 8: Run shell tests to confirm they pass**

  Run: `npm --prefix app run test -- --run WorkspaceShell`
  Expected: PASS.

- [ ] **Step 9: Commit**

  ```bash
  git add \
    app/src/workspace/hooks/useDropTarget.ts \
    app/src/workspace/hooks/useDropTarget.test.ts \
    app/src/workspace/components/WorkspaceShell.tsx \
    app/src/workspace/components/WorkspaceShell.test.tsx
  git commit -m "[item 67] Show ReplaceFileDialog on non-EMPTY drops via useDropTarget callback"
  ```

### Task 8.7: Item 68 — Full Phase 3 test sweep

**Files:** (no file change)

- [ ] **Step 1: Frontend tests**

  Run: `npm --prefix app run test -- --run`
  Expected: all pass.

- [ ] **Step 2: Frontend typecheck**

  Run: `npm --prefix app run typecheck`
  Expected: exit 0.

- [ ] **Step 3: Frontend lint**

  Run: `npm --prefix app run lint`
  Expected: exit 0.

- [ ] **Step 4: Sidecar unit tests**

  Run: `cd sidecar && go test ./...`
  Expected: all pass.

- [ ] **Step 5: Sidecar integration tests (real ffprobe)**

  Run: `cd sidecar && STUDIO_FFPROBE_PATH=$(node -e "console.log(require('path').resolve('../app/src-tauri/binaries/ffprobe-x86_64-apple-darwin'))") go test -tags=integration ./...`

  (Adjust the triple to match the host platform; on macOS arm64 use `ffprobe-aarch64-apple-darwin`.)

  Expected: all integration tests pass.

- [ ] **Step 6: Rust check**

  Run: `cd app/src-tauri && cargo check && cargo test`
  Expected: exit 0.

- [ ] **Step 7: Codegen drift check**

  Run: `npm run gen:schemas && git status --porcelain schemas app/src/ipc/generated sidecar/internal/ipc/generated app/src-tauri/src/ipc/generated.rs`
  Expected: no output (clean tree).

- [ ] **Step 8: Manual smoke — happy path**

  Run: `npm run tauri:dev` (or the project's dev command). Drop `test-assets/tiny-h264-aac-stereo.mp4`. Verify: green dot, metadata visible, Details opens drawer, Copy diagnostics succeeds.

- [ ] **Step 9: Manual smoke — unsupported**

  After Remove, drop `test-assets/tiny-no-audio.mp4`. Verify: yellow dot, `UnsupportedPanel` with `NO_AUDIO_STREAM` issue, Replace clears the workspace.

- [ ] **Step 10: Manual smoke — error**

  After Remove, drop `test-assets/corrupt-truncated.mp4`. Verify: red dot, `ErrorPanel` with CORRUPT_MEDIA copy, Retry re-runs probe.

- [ ] **Step 11: Manual smoke — replace**

  With any file loaded, drop a second file. Verify: `ReplaceFileDialog` shows both filenames; Replace transitions to PROBING on the new file; Cancel leaves the original file intact.

- [ ] **Step 12: Manual smoke — multi-file drop is ignored**

  With workspace EMPTY, drop two files at once. Verify: nothing happens (state remains EMPTY).

- [ ] **Step 13: Item 68 complete — commit if any docs were touched, otherwise no commit**

  If the sweep surfaced doc tweaks (e.g. updating `docs/troubleshooting/ffprobe.md` with a newly-observed failure mode), commit them:

  ```bash
  git add <touched-docs>
  git commit -m "[item 68] Phase 3 test sweep doc updates"
  ```

  Otherwise skip the commit — item 68 is the verification gate, not a code change.

### Task 8.8: Item 69 — PR 8 ready for review (no further commits)

- [ ] **Step 1: Confirm branch state**

  Run: `git log --oneline feature/phase-3-pr7..HEAD`
  Expected: ~6–7 commits covering items 62–67 (and optionally 68 if docs were touched).

- [ ] **Step 2: Confirm no untracked output from codegen**

  Run: `git status --porcelain`
  Expected: empty.

- [ ] **Step 3: Item 69 complete — PR 8 ready**

  This PR closes Phase 3. Cumulative review surface across the 8 PRs covers:

  - Structured IPC errors end-to-end (PR 1)
  - Bundled ffprobe + supervisor env wiring (PR 2)
  - Subprocess runner with cross-platform group kill + capped stdio (PR 3)
  - Parser/normalizer/compat with hybrid unsupported model (PR 4)
  - Sidecar `media.probe` handler + integration tests with real ffprobe (PR 5)
  - Tauri command + zustand store with state machine (PR 6)
  - Workspace shell core flow: EmptyState, ActiveFileCard, drop target (PR 7)
  - Secondary surfaces: DiagnosticsDrawer, ReplaceFileDialog, ErrorPanel, UnsupportedPanel (PR 8)




