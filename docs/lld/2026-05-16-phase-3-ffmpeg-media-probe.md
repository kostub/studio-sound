# Phase 3 — FFmpeg Bundling & Media Probe — Low-Level Design

**Status:** Draft (LLD)
**Date:** 2026-05-16
**PRD:** [docs/reqs/phase_3_ffmpeg_media_probe_prd_and_ui_design.md](../reqs/phase_3_ffmpeg_media_probe_prd_and_ui_design.md)
**Research / HLD:** [docs/research/2026-05-16-phase-3-ffmpeg-media-probe.md](../research/2026-05-16-phase-3-ffmpeg-media-probe.md)
**IPC contract reference:** [docs/ipc-contract.md](../ipc-contract.md)

---

## 1. Summary / Goals

Phase 3 turns the Phase 0/1 desktop shell into a single-file media workspace that ingests one creator video at a time, runs a bundled `ffprobe` against it locally, and surfaces canonical metadata plus a compatibility verdict. The IPC layer gains a new `media.probe` method; the frontend grows a full workspace UI (empty state, active-file card, diagnostics drawer, replace-file modal, accessibility-conformant error UX).

**In scope (per resolved OQs):**
- Bundle `ffprobe` only (no `ffmpeg`) from BtbN/FFmpeg-Builds LGPL 7.x.
- Static video-file icon as the placeholder thumbnail; no poster-frame extraction.
- Full workspace UI (folds the previously-Phase-5 UI scope in).
- Upgrade Tauri command error serialisation to structured `SerializableIpcError` now.
- Zustand-based workspace state machine.
- Commit small CC0/CC-BY media fixtures under `test-assets/`.

**Non-goals (deferred):**
- Per-probe cancellation wiring (deferred to Phase 4 when long-running jobs land).
- `ffmpeg` bundling, poster-frame extraction, waveforms, preview playback, transcoding.
- macOS notarisation; only co-signing of the bundled `ffprobe` is in scope so first-launch works in dev/CI.
- On-disk PII scrubbing in logs (telemetry policy = no network egress; existing logger unchanged).

The canonical `MediaProbeResult` shape introduced here is intended to remain stable through later phases.

---

## 2. Current Code-base Findings

Verified against `feature/phase-3` (working tree from `master` plus the Phase 3 research/PRD docs):

- The repo has a Tauri 2 + React/TS shell in `app/` and a Go sidecar in `sidecar/`, bridged by NDJSON over stdin/stdout. The sidecar is bundled as `binaries/studio-sidecar` per platform `[verified: app/src-tauri/tauri.conf.json:36]`. The capability `shell:allow-spawn` is scoped to the sidecar `[verified: app/src-tauri/capabilities/default.json:8-19]`.
- Sidecar binaries live under `app/src-tauri/binaries/` with target-triple suffixes (`studio-sidecar-x86_64-pc-windows-msvc.exe`, `studio-sidecar-x86_64-apple-darwin`, `studio-sidecar-aarch64-apple-darwin`) `[verified: scripts/build-sidecar.mjs:10-26]`. The directory is currently empty in source (.gitkeep only) and populated at build time `[verified: ls output]`.
- The IPC contract is mature: schemas in `schemas/` are the single source of truth; codegen produces TS, Go, and Rust types `[verified: scripts/gen-schemas.mjs:32-59]`. Existing schemas use `$defs.<Name>Payload` / `$defs.<Name>Result` with `additionalProperties: false` and length-bounded strings `[verified: schemas/system.echo.schema.json]`.
- Three system methods exist today: `system.ping`, `system.echo`, `system.shutdown`. No `media.*` namespace yet. Registration happens in `cli.Run`'s `case "serve":` block `[verified: sidecar/internal/cli/cli.go:52-56]`.
- Handler pattern: schema → generated types in `sidecar/internal/ipc/generated/<name>.go` → handler in `sidecar/internal/ipc/handlers/<name>.go` (currently `echo.go`, `ping.go`, `shutdown.go`) `[verified: ls sidecar/internal/ipc/handlers/, docs/ipc-contract.md:199-235]`. Handlers validate via `*ipc.Validator` from `internal/ipc/validator.go:19-30`, which wraps `github.com/santhosh-tekuri/jsonschema/v5` `[verified: sidecar/internal/ipc/validator.go]`.
- The dispatcher enforces a 64-handler in-flight cap (`maxConcurrentDispatch`), recovers from panics, and surfaces structured RPC errors via `*ipc.RPCError` `[verified: sidecar/internal/ipc/dispatcher.go:44, 188-204]`. The reserved error-code list lives in `errors.go` `[verified: sidecar/internal/ipc/errors.go:8-16]` and is mirrored in `docs/ipc-contract.md:43-55`.
- The Rust supervisor spawns the sidecar via `app.shell().sidecar("studio-sidecar")`, sets `STUDIO_LOG_FILE` on the child, and handles NDJSON decode + broadcast fan-out + capped-pending pool (64) + per-method timeouts + auto-restart with exponential backoff `[verified: app/src-tauri/src/ipc/supervisor.rs:104-138, 220-240]`, `[verified: app/src-tauri/src/ipc/client.rs:30, 120-188]`.
- Tauri commands currently return `Result<serde_json::Value, String>` and stringify the underlying `IpcError` via `.map_err(|e| e.to_string())` `[verified: app/src-tauri/src/commands.rs:38-45, 51-60]`. The Rust `IpcError::Other { code, message, details }` already carries structured fields `[verified: app/src-tauri/src/ipc/error.rs:19-25]` but they are lost at the Tauri boundary. The TS `toIpcError` defaults to `code: 'UNKNOWN'` for string rejections `[verified: app/src/ipc/client.ts:42-51]`.
- The frontend is the Phase 0 placeholder `[verified: app/src/App.tsx:27-38]` plus an opt-in `Diagnostics` panel toggled by `Ctrl+Shift+D` in dev builds `[verified: app/src/components/Diagnostics.tsx]`. No state-management library is present; React's built-in hooks only. No drop zone, file row, drawer, modal, or media model exists. `test-assets/` is empty (`.gitkeep`) `[verified: ls test-assets/]`.
- The existing `IpcError` interface on the TS side already has the shape `{ code: string; message: string; details?: unknown }` so a structured error upgrade requires no frontend type changes `[verified: app/src/ipc/client.ts:35-39]`.
- CI runs on Windows + macOS Intel + macOS Apple Silicon, builds all three sidecar binaries, runs `go test -tags=integration ./...`, and asserts codegen produces a clean tree `[verified: .github/workflows/ci.yml]`. There is no FFmpeg fetch step today.
- The Tauri shell plugin is enabled (`tauri-plugin-shell = "2"` in `Cargo.toml`) `[verified: app/src-tauri/Cargo.toml:18]`. The dialog plugin is **not** present and must be added if "Browse files" uses the Tauri dialog API.
- The sidecar logger is `slog` JSON over `lumberjack` rotating files; no current scrubbing of paths or filenames `[verified: sidecar/internal/logger/logger.go:14-32]`. This is acceptable under the resolved telemetry policy (no network egress; on-disk PII is permitted).

---

## 3. Proposed Design

### 3.1 Data model changes

No persisted data model (no DB, no on-disk state). Two new in-memory / wire-only types:

#### `MediaProbeResult` (canonical, wire-stable)

Defined in `schemas/media.probe.schema.json` (`$defs.ProbePayload`, `$defs.ProbeResult`). Generated into TS / Go / Rust by `npm run gen:schemas`. Shape (per PRD §11.3, plus minor refinements):

```ts
interface ProbePayload {
  path: string;  // absolute path; min 1, max 4096
}

interface ProbeResult {
  id: string;                    // ULID generated by sidecar; stable per probe
  path: string;                  // echo of input
  filename: string;              // basename(path)
  sizeBytes: number;             // from os.Stat
  durationSeconds?: number;      // optional (§13.5 — missing duration is allowed)

  container: {
    format: string;              // ffprobe format.format_name (e.g. "mov,mp4,m4a,3gp,3g2,mj2")
    longName: string;            // ffprobe format.format_long_name
  };

  video?: {
    codec: string;               // ffprobe stream.codec_name (e.g. "h264")
    width: number;
    height: number;
    fps: number;                 // computed from r_frame_rate "num/den"
    bitrate?: number;            // bits per second
    pixelFormat?: string;        // ffprobe stream.pix_fmt
  };

  audio?: {                      // present iff at least one audio stream
    codec: string;
    channels: number;
    sampleRate: number;          // Hz
    bitrate?: number;            // bps
    layout?: string;             // pass-through ffprobe channel_layout string verbatim (OQ-8)
    trackIndex: number;          // ffprobe stream.index of the selected default track
    trackCount: number;          // total audio streams in the file
    tracks: Array<{              // all audio streams, for the diagnostics drawer (PRD §13.4)
      index: number;
      codec: string;
      channels: number;
      sampleRate: number;
      bitrate?: number;
      layout?: string;
      title?: string;            // from stream.tags.title if present
      language?: string;         // from stream.tags.language if present
      isDefault: boolean;        // matches the selected trackIndex
    }>;
  };

  compatibility: {
    supported: boolean;
    issues: string[];            // human-readable; non-empty when supported=false
    warnings: string[];          // human-readable; non-blocking notes
  };
}
```

Notes:
- `audio.layout` is documented as **pass-through** (OQ-8 resolution). Schema uses `"type": "string"` with no enum; UI must treat it as opaque.
- `durationSeconds` is optional to satisfy PRD §13.5.
- `audio.tracks` is mandatory in the result when `audio` is present so the diagnostics drawer can render the multi-track list deterministically.

#### `SerializableIpcError` (Rust → TS error wire format)

In `app/src-tauri/src/ipc/error.rs` (new struct alongside the existing enum):

```rust
#[derive(Debug, serde::Serialize)]
pub struct SerializableIpcError {
    pub code: String,
    pub message: String,
    #[serde(skip_serializing_if = "Option::is_none")]
    pub details: Option<serde_json::Value>,
}
```

Plus `impl From<IpcError> for SerializableIpcError` mapping every enum variant to a `(code, message)` pair (existing strings reused: `PROTOCOL_VERSION_MISMATCH`, `MALFORMED_ENVELOPE`, `SIDECAR_UNAVAILABLE`, `SIDECAR_BUSY`, `TIMEOUT`, `UNKNOWN_METHOD`, etc.; `IpcError::Other` passes through unchanged).

### 3.2 API contract

#### New IPC method: `media.probe`

Schema: `schemas/media.probe.schema.json` (new file). Follows the convention in `docs/ipc-contract.md:152-183`:

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
    "ProbeResult": { /* shape from §3.1 */ }
  }
}
```

Add `media.probe.schema.json` to the `schemaFiles` array in `scripts/gen-schemas.mjs:12-17`.

#### New error codes (added to `sidecar/internal/ipc/errors.go` and the table in `docs/ipc-contract.md:43-55`)

| Code | Emitter | Meaning |
|---|---|---|
| `FILE_NOT_FOUND` | Go (locator/runner) | Path does not exist when probe begins. |
| `ACCESS_DENIED` | Go (runner stat / ffprobe stderr classifier) | File present but not readable. |
| `UNSUPPORTED_CONTAINER` | Go (compat) | Container format not in allow-list. |
| `UNSUPPORTED_CODEC` | Go (compat) | Audio codec not in allow-list. |
| `NO_AUDIO_STREAM` | Go (compat) | File has zero audio streams. |
| `CORRUPT_MEDIA` | Go (parser / nonzero exit + stderr classifier) | ffprobe failed to parse media. |
| `FFPROBE_FAILURE` | Go (runner spawn / unexpected exit) | ffprobe could not be invoked or crashed. |
| `FFPROBE_MISSING` | Rust (supervisor spawn) | Bundled ffprobe binary absent at startup. |

`UNKNOWN` is already present implicitly via the existing client mapping.

#### New Tauri command: `media_probe`

In `app/src-tauri/src/commands.rs`:

```rust
#[tauri::command]
pub async fn media_probe(
    path: String,
    client: State<'_, Arc<IpcClient>>,
) -> Result<Value, SerializableIpcError> {
    let payload = serde_json::json!({ "path": path });
    client
        .call("media.probe", payload, default_timeout("media.probe"))
        .await
        .map_err(SerializableIpcError::from)
}
```

`default_timeout` gets a new arm: `"media.probe" => Duration::from_secs(30)` (well above PRD §11.7's 3s target, allows cold-start ffprobe under macOS Gatekeeper).

The existing commands (`ipc_ping`, `ipc_echo`, `ipc_shutdown`) **also** change return type to `Result<Value, SerializableIpcError>`. The frontend `toIpcError` continues to accept the existing string-rejection path for backward compat but the new structured path becomes the default.

### 3.3 Class / module changes

#### `schemas/media.probe.schema.json` `[new]`
- Defines `$defs.ProbePayload` and `$defs.ProbeResult` per §3.2.

#### `scripts/gen-schemas.mjs` `[verified: scripts/gen-schemas.mjs:12-17]`
- Append `'media.probe.schema.json'` to `schemaFiles`.

#### `sidecar/internal/media/` `[new package]`

Each file is small and single-purpose. The package has **zero** dependency on the `ipc` package — it is a pure media library, importable from tests without spinning up a dispatcher.

##### `sidecar/internal/media/locator.go` `[new]`
- `func ResolveFFprobe() (string, error)`
  - Reads `STUDIO_FFPROBE_PATH` from env. If non-empty, `os.Stat` it and return; if missing, return `ErrFFprobeMissing`.
  - If env unset, return `ErrFFprobeMissing` (no `PATH` lookup — we never silently use a system ffprobe).
  - Called from: `sidecar/internal/media/runner.go::Run` `[new]`.
- `var ErrFFprobeMissing = errors.New("ffprobe binary not found")`

##### `sidecar/internal/media/runner.go` `[new]`
- `type RunResult struct { Stdout []byte; ExitCode int; StderrTail string }`
- `func Run(ctx context.Context, ffprobePath, mediaPath string) (*RunResult, error)`
  - Builds `exec.CommandContext(ctx, ffprobePath, "-v", "error", "-hide_banner", "-print_format", "json", "-show_format", "-show_streams", mediaPath)`.
  - Reads stdout into a `bytes.Buffer` capped at 1 MiB; reads stderr tail (last 4 KiB) for the classifier.
  - Per-probe deadline: `ctx` is expected to carry a 10s timeout (set by the handler, see `media_probe.go`).
  - On non-Windows: sets `cmd.SysProcAttr = &syscall.SysProcAttr{Setpgid: true}` and on cancel/timeout calls `syscall.Kill(-cmd.Process.Pid, syscall.SIGKILL)`.
  - On Windows: sets `cmd.SysProcAttr = &windows.SysProcAttr{CreationFlags: windows.CREATE_NEW_PROCESS_GROUP}` and on cancel/timeout invokes `taskkill /T /F /PID <pid>` via `exec.Command`.
  - Returns `(*RunResult, error)`. Error wraps `os.ErrNotExist`, `os.ErrPermission`, `context.DeadlineExceeded`, or a generic `"ffprobe spawn failed"` so the error mapper can classify.
  - Called from: `sidecar/internal/media/media.go::Probe` `[new]`.

  **Cross-platform subprocess hygiene:** the Windows-specific code path lives in `runner_windows.go` (build-tag `//go:build windows`) and the Unix code path in `runner_unix.go` (build-tag `//go:build !windows`). Both expose a single `killGroup(cmd *exec.Cmd) error` helper.

##### `sidecar/internal/media/parser.go` `[new]`
- Mirrors the load-bearing subset of ffprobe's JSON:
  ```go
  type ffprobeOutput struct {
      Format  ffprobeFormat   `json:"format"`
      Streams []ffprobeStream `json:"streams"`
  }
  type ffprobeFormat struct {
      FormatName     string `json:"format_name"`
      FormatLongName string `json:"format_long_name"`
      Duration       string `json:"duration"`   // string in ffprobe JSON; parse with strconv.ParseFloat
      Size           string `json:"size"`
      BitRate        string `json:"bit_rate"`
  }
  type ffprobeStream struct {
      Index         int               `json:"index"`
      CodecType     string            `json:"codec_type"`     // "video" | "audio" | "subtitle" | "data"
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
  ```
- `func Parse(stdout []byte) (*ffprobeOutput, error)` — decodes via `encoding/json` with no `DisallowUnknownFields` (tolerate ffprobe version skew).
- Returns `errParseFailed` (wrapped) on any decode failure so the caller can map to `CORRUPT_MEDIA`.

##### `sidecar/internal/media/normalize.go` `[new]`
- `func Normalize(in *ffprobeOutput, path string, sizeBytes int64) (*MediaProbeResult, error)`
  - Computes `filename` from `filepath.Base(path)`.
  - Parses `format.duration` as float64; if empty/unparseable, leaves `DurationSeconds` nil (PRD §13.5).
  - Selects the **default audio track** per PRD §13.4: first stream where `codec_type == "audio"` and `disposition.default == 1`; falls back to the first audio stream by `index` ascending if no default flag is set.
  - Maps `r_frame_rate` `"num/den"` into a float64 (fps).
  - Returns a fully-populated `*MediaProbeResult` from §3.1; `Compatibility` is left empty for `compat.go` to fill.
- Called from: `sidecar/internal/media/media.go::Probe` `[new]`.

##### `sidecar/internal/media/compat.go` `[new]`
- `type Verdict struct { Supported bool; Issues []string; Warnings []string; DominantCode string }`
- `func Evaluate(r *MediaProbeResult) Verdict`
  - Allow-list (PRD §15.1 plus implementation_plan.md):
    - Container `format_name` substring match: any of `"mov,mp4,m4a,3gp,3g2,mj2"`, `"matroska,webm"`, `"webm"`. (ffprobe reports comma-separated lists; substring match against the canonical short names.)
    - Audio codecs: `aac`, `opus`, `pcm_s16le`, `pcm_s24le`, `pcm_f32le`, `mp3`, `vorbis`, `flac`.
  - Sets `Supported=false` and `DominantCode = "UNSUPPORTED_CONTAINER"` when container is outside the allow-list.
  - Sets `Supported=false` and `DominantCode = "NO_AUDIO_STREAM"` when `r.Audio == nil`.
  - Sets `Supported=false` and `DominantCode = "UNSUPPORTED_CODEC"` when audio codec is outside the allow-list.
  - Adds non-blocking `Warnings` (no `Supported` change) for: missing duration, multi-track audio (informational), bitrate=0.
  - Called from: `sidecar/internal/media/media.go::Probe` `[new]`.

##### `sidecar/internal/media/errors.go` `[new]`
- `func MapRunError(err error, stderrTail string) *ipc.RPCError`
  - Pattern: `errors.Is(err, os.ErrNotExist)` → `FILE_NOT_FOUND`.
  - `errors.Is(err, os.ErrPermission)` → `ACCESS_DENIED`.
  - `errors.Is(err, context.DeadlineExceeded)` → `FFPROBE_FAILURE` with message "probe exceeded 10s deadline".
  - `errors.Is(err, ErrFFprobeMissing)` → `FFPROBE_FAILURE` with code `FFPROBE_FAILURE` and message "ffprobe binary not located" (the `FFPROBE_MISSING` supervisor-side code is distinct; this in-handler code is reached only if env var resolution succeeded at supervisor-spawn but the file vanished afterwards).
  - stderr classifier (lowercase substring match on `stderrTail`):
    - `"invalid data found"`, `"moov atom not found"`, `"end of file"` → `CORRUPT_MEDIA`.
    - `"permission denied"` → `ACCESS_DENIED`.
    - default → `FFPROBE_FAILURE`.
- `func MapVerdict(v Verdict) *ipc.RPCError` — uses `v.DominantCode` and a fixed message per code.
- `func MapParseError(err error) *ipc.RPCError` — wraps as `CORRUPT_MEDIA`.

  **Note:** `errors.go` imports `internal/ipc` only for the `RPCError` symbol and code constants — this is the **one** place the media package touches IPC types. The wiring is kept in this file (rather than in handler glue) to keep the mapping table in a single, testable location.

##### `sidecar/internal/media/media.go` `[new]`
- `func Probe(ctx context.Context, ffprobePath, mediaPath string) (*MediaProbeResult, error)`
  - `os.Stat(mediaPath)` first; on `ErrNotExist`/`ErrPermission` return mapped RPC error before spawning.
  - Apply a 10s per-probe deadline derived from `ctx`.
  - `runner.Run(ctx, ffprobePath, mediaPath)`.
  - `parser.Parse(stdout)`.
  - `normalize.Normalize(parsed, path, size)`.
  - `compat.Evaluate(result)` → fills `result.Compatibility`. If `Verdict.Supported == false` **and** `Verdict.DominantCode != ""`, return `(nil, *ipc.RPCError)` mapped via `errors.MapVerdict` so unsupported files surface as RPC errors (matching PRD §11.5).
  - Otherwise return `(result, nil)` even when `result.Compatibility.Supported == false` is unreachable (compat populates `supported=true` on the happy path).

  **Design note (snap call resolved during drafting):** unsupported files surface as RPC errors (not as a successful response with `supported=false`). Reason: PRD §11.4 shows `UNSUPPORTED_CODEC` as an error envelope, and PRD §8.5's error UX maps each code to a distinct screen. Returning success-with-`supported=false` would force the UI to branch on two axes (ok vs error, then supported vs unsupported). The error-axis-only contract is simpler and matches the PRD example payloads.

##### `sidecar/internal/ipc/handlers/media_probe.go` `[new]`
- Thin adapter following the `echo.go` template `[verified: sidecar/internal/ipc/handlers/echo.go]`:
  - `const probeSchema = "..."` — inlined `ProbePayload` shape for `*ipc.Validator`.
  - `var probeValidator *ipc.Validator` (lazy `sync.Once`).
  - `var ffprobeOnce sync.Once; var ffprobePath string; var ffprobeErr error` — resolve once at first invocation.
  - `func ProbeHandler(ctx context.Context, id string, payload json.RawMessage) (any, error)`:
    - Validate payload → decode into a local `probePayload struct { Path string }`.
    - Emit `probe_started` log event (with `path`).
    - `path` resolution: pass through verbatim (no symlink resolution; Tauri provides absolute paths).
    - Apply a 10s deadline via `ctx, cancel := context.WithTimeout(ctx, 10*time.Second)`.
    - Call `media.Probe(ctx, ffprobePath, payload.Path)`.
    - Emit `probe_completed` (with `duration_ms`) or `probe_failed` (with `code`).
    - Return `result` or RPC error.

##### `sidecar/internal/cli/cli.go` `[verified: sidecar/internal/cli/cli.go:52-56]`
- Modified: add a new line inside the `setup` closure: `d.Register("media.probe", handlers.ProbeHandler)`.

##### `sidecar/internal/ipc/errors.go` `[verified: sidecar/internal/ipc/errors.go:8-16]`
- Modified: add the new code constants from §3.2.

##### `app/src-tauri/src/ipc/error.rs` `[verified: app/src-tauri/src/ipc/error.rs]`
- Modified: add `SerializableIpcError` struct and `impl From<IpcError> for SerializableIpcError` (mapping table in §3.1).

##### `app/src-tauri/src/commands.rs` `[verified: app/src-tauri/src/commands.rs]`
- Modified:
  - All three existing commands change return type from `Result<Value, String>` to `Result<Value, SerializableIpcError>`.
  - New command `media_probe(path: String, client: State<'_, Arc<IpcClient>>)` per §3.2.
  - `default_timeout` gains the `"media.probe" => Duration::from_secs(30)` arm.
- Imports: `use crate::ipc::error::SerializableIpcError;`.

##### `app/src-tauri/src/ipc/supervisor.rs` `[verified: app/src-tauri/src/ipc/supervisor.rs:104-138, 220-240]`
- Modified `Supervisor::spawn`: after resolving `log_dir`, also resolve `ffprobe_path` via `app.path().resolve("binaries/<target-triple-prefixed-name>", BaseDirectory::Resource)` — note the target-triple is fixed at compile-time via `cfg!` matching (`x86_64-pc-windows-msvc`, `x86_64-apple-darwin`, `aarch64-apple-darwin`). If the file is missing, return `IpcError::Other { code: "FFPROBE_MISSING", message: "bundled ffprobe binary not found at <path>", details: None }`.
- Modified `spawn_child`: add `.env("STUDIO_FFPROBE_PATH", ffprobe_path.to_string_lossy().to_string())` to the command builder (alongside the existing `STUDIO_LOG_FILE`).
- The `SpawnContext` struct gains an `ffprobe_path: PathBuf` field so respawn (`spawn_child`) reuses the resolved path without re-querying `app.path()`.

##### `app/src-tauri/src/lib.rs` `[verified: app/src-tauri/src/lib.rs]`
- Modified: register the new command in `tauri::generate_handler![..., commands::media_probe]`.

##### `app/src-tauri/capabilities/default.json` `[verified: app/src-tauri/capabilities/default.json]`
- Modified: add the `dialog:default` permission for the "Browse files" flow. No new permission for ffprobe — the webview never invokes it directly; the Rust supervisor spawns the sidecar which invokes ffprobe, and that path is already covered by the existing `shell:allow-spawn` scoped to `studio-sidecar`.

##### `app/src-tauri/Cargo.toml` `[verified: app/src-tauri/Cargo.toml]`
- Modified: add `tauri-plugin-dialog = "2"` to `[dependencies]`.

##### `app/package.json` `[verified: app/package.json]`
- Modified: add `@tauri-apps/plugin-dialog: "^2.0.0"` to `dependencies`. Add `zustand: "^4.5.0"` to `dependencies`.

##### `app/src/ipc/client.ts` `[verified: app/src/ipc/client.ts]`
- Modified:
  - `toIpcError` handles the new structured error shape: if `err` is an object with `code` and `message`, return it as-is; otherwise fall back to the existing string path.
  - New `probe(path: string): Promise<ProbeResult>` wrapper invoking `media_probe`, with `IpcError`-throwing semantics matching the existing wrappers.

##### `app/src/state/workspace.ts` `[new]`
- Zustand store. States per PRD §12: `EMPTY | FILE_LOADED | PROBING | READY | ERROR | RETRYING | REMOVED`. Single union-discriminator on the `status` field; `result?: ProbeResult; error?: IpcError; path?: string` co-existing fields.
- Selectors: `useWorkspaceStatus()`, `useWorkspaceFile()`.
- Actions (each a thunk-style function on the store):
  - `loadFile(path: string)` — transitions EMPTY → FILE_LOADED → PROBING → READY|ERROR.
  - `replaceFile(path: string)` — confirms (UI gates the confirmation modal) then `clearFile()` then `loadFile(path)`.
  - `clearFile()` — transitions to EMPTY.
  - `retry()` — re-runs probe on the same path; transitions ERROR → RETRYING → READY|ERROR.
- Cancellation is **not** wired in Phase 3 (OQ-9 resolved: deferred). `replaceFile` while a probe is in flight will wait for the in-flight probe to settle before issuing the next one; the UI shows a non-blocking "completing previous probe…" hint.

##### `app/src/components/EmptyState.tsx` `[new]`
- Drop zone + browse CTA + supported-formats line + privacy line (PRD §8.2).
- Browse CTA uses `@tauri-apps/plugin-dialog`'s `open({ filters: [{ name: 'Video', extensions: ['mp4','mov','mkv','webm'] }], multiple: false })`.

##### `app/src/components/ActiveFileCard.tsx` `[new]`
- Renders the workspace card (PRD §8.3). Thumbnail is a static SVG video-file icon (OQ-2 — no poster frames).
- Status indicator: dot + text label per `useWorkspaceStatus()`.
- Buttons: Details (opens drawer), Retry (visible iff status === ERROR), Remove (clears workspace).

##### `app/src/components/DiagnosticsDrawer.tsx` `[new]`
- Right-side slide drawer (PRD §8.4). Renders container, video, audio (single track summary) plus the full `audio.tracks` list (PRD §13.4).
- "Copy diagnostics" button writes a serialised text representation to clipboard, then swaps to a checkmark with a 2s toast (PRD §8.4 copy feedback).
- Closes on Escape, outside-click, explicit X button (PRD §10).

##### `app/src/components/ReplaceFileDialog.tsx` `[new]`
- Modal triggered when a drop or browse selects a new file and the workspace is in `READY`/`PROBING`/`ERROR` (anything not `EMPTY`). Calls `replaceFile(path)` on confirm.

##### `app/src/components/ErrorPanel.tsx` `[new]`
- One component, switch on `error.code` to render the right variant (PRD §8.5).
- Variants render the message + guidance + CTAs from PRD §8.5.

##### `app/src/components/WorkspaceShell.tsx` `[new]`
- Top-level layout (header / workspace / footer). Mounts `EmptyState` when `status === EMPTY`, otherwise `ActiveFileCard`. Hosts the `DiagnosticsDrawer` and `ReplaceFileDialog` portals.

##### `app/src/hooks/useDropTarget.ts` `[new]`
- Subscribes to Tauri's `getCurrentWebview().onDragDropEvent()` for OS-level drops (per PRD §9.1; HTML5 drop is flaky in webview2 on Windows for some file types).
- Rejects directories (no `path` set or known directory indicator from the event) and multi-file drops (only first supported file is accepted; user gets the inline "current version supports one file at a time" message, PRD §9.1).
- Returns the dragged file path on drop and an `isDragOver` flag for visual state.

##### `app/src/App.tsx` `[verified: app/src/App.tsx]`
- Modified: replace the Phase 0 placeholder body with `<WorkspaceShell />`. The dev-only Diagnostics overlay (Ctrl+Shift+D) remains untouched.

##### `app/src/styles/tokens.css` `[new]`
- Small token file: radius (cards 16px, buttons 12px), spacing (4/8/12/16/24/32), motion (120ms / 200ms / 320ms), per PRD §14.4. Imported once from `app/src/main.tsx`.

#### Build-time integration

##### `scripts/fetch-ffprobe.mjs` `[new]`
- Reads `third_party/ffprobe.lock.json` `[new]`: `{ "version": "7.x.y", "platforms": { "windows-amd64": { "url": "...", "sha256": "...", "innerPath": "bin/ffprobe.exe" }, ... } }`.
- Source: BtbN/FFmpeg-Builds LGPL 7.x release artifacts (OQ-5).
- For each platform target, downloads the archive into a local cache (`~/.cache/studio-sound/ffprobe/<sha256-prefix>/...`), verifies the SHA-256, extracts the `innerPath`, and copies to `app/src-tauri/binaries/ffprobe-<target-triple>{,.exe}`.
- Skips download if the destination file already has the expected SHA-256.
- Exits 0 on success; non-zero with a diagnostic on checksum mismatch.

##### `scripts/build-sidecar.mjs` `[verified: scripts/build-sidecar.mjs]`
- **Unchanged.** ffprobe fetch is a separate script invoked from `npm run setup` and from CI.

##### `app/src-tauri/tauri.conf.json` `[verified: app/src-tauri/tauri.conf.json:36]`
- Modified: `bundle.externalBin` becomes `["binaries/studio-sidecar", "binaries/ffprobe"]`. Tauri will look for the target-triple-suffixed file (`binaries/ffprobe-x86_64-pc-windows-msvc.exe`, etc.) and place it next to the sidecar in the bundle.

##### `package.json` `[verified: scripts]`
- Modified: add `"ffprobe:fetch": "node scripts/fetch-ffprobe.mjs"` to `scripts`. Update `setup` to also run it: `setup: "node scripts/check-prereqs.mjs && npm install && npm run ffprobe:fetch && npm run sidecar:build"`.

##### `.github/workflows/ci.yml` `[verified: .github/workflows/ci.yml]`
- Modified: add a new step `"Fetch ffprobe"` running `npm run ffprobe:fetch` between `"Install dependencies"` and `"Generate schemas"`. Cache the BtbN downloads via `actions/cache` keyed on the contents of `third_party/ffprobe.lock.json`. Add a corresponding `"Verify ffprobe artifacts"` step (one paragraph check) before the existing `"Build sidecar"` step.
- Modified: the `"Integration tests (sidecar E2E)"` step exports `STUDIO_FFPROBE_PATH` pointing at the host-platform binary so `go test -tags=integration ./...` can find ffprobe under `internal/media`.

##### `test-assets/` `[verified: empty]`
- Modified: commit a small fixture set (CC0/CC-BY clips), total well under 30 MB. Suggested seed set:
  - `tiny-h264-aac-stereo.mp4` (~1 MB, 5s, 1280×720, AAC stereo, default-flagged track).
  - `tiny-h264-aac-multitrack.mov` (~3 MB, 5s, two AAC stereo audio streams, track 1 default-flagged with tag `title="Microphone"`, track 2 tag `title="Game Audio"`).
  - `tiny-vp9-opus.webm` (~1 MB, 5s).
  - `tiny-no-audio.mp4` (~1 MB, 5s, video-only) — for `NO_AUDIO_STREAM`.
  - `corrupt-truncated.mp4` (~512 KiB, truncated mid-stream) — for `CORRUPT_MEDIA`.
  - `unicode-name-🎥-интервью.mp4` (~1 MB) — for the Unicode-path test (PRD §13.2).

##### `docs/ipc-contract.md` `[verified]`
- Modified: append `media.probe` to the "Available methods" section following the existing template; add the eight new error codes to the reserved-codes table.

##### `docs/adr/2026-05-16-ffprobe-bundling.md` `[new]`
- Records the LGPL bundling decision: source family (BtbN/FFmpeg-Builds), pin format, audit posture, lock-file mechanism.

##### `README.md` and `app/src/components/AboutPanel.tsx` (or equivalent licenses-page surface)
- Modified / new: reference the bundled `ffprobe` version and the LGPL text. PRD §11.6 mandates this in README + about screen + licenses page. Concrete licenses surface inside the app is **deferred** to a follow-up — Phase 3 must at least update README + commit `third_party/LICENSE.ffmpeg-lgpl.txt`.

### 3.4 Logic flow

#### Happy path (drop → READY)

1. User drops a file. `useDropTarget` receives the event from `onDragDropEvent`, extracts the first path, rejects multi-file drops.
2. `App` shell asks `workspace`: if `status !== EMPTY`, mount `ReplaceFileDialog`; user confirms; store calls `replaceFile(path)`.
3. `workspace.loadFile(path)` transitions to `PROBING`; UI renders `ActiveFileCard` in spinner state within <100ms (PRD §11.7).
4. `loadFile` calls `client.probe(path)` → Tauri `invoke('media_probe', { path })` → `commands::media_probe` → `IpcClient::call("media.probe", { path }, 30s)`.
5. Supervisor (already alive, child env already carries `STUDIO_FFPROBE_PATH`) writes the NDJSON envelope to the sidecar's stdin.
6. Dispatcher routes to `handlers.ProbeHandler` (registered via `cli.Run`). Handler validates payload, applies a 10s deadline, calls `media.Probe`.
7. `media.Probe` → `os.Stat(path)` → `runner.Run(ctx, ffprobePath, path)` spawns ffprobe with the fixed argument set, reads bounded stdout, kills the process group on cancel.
8. `parser.Parse(stdout)` → `normalize.Normalize(parsed, path, size)` → `compat.Evaluate(result)` fills `compatibility`.
9. Handler returns the `*MediaProbeResult`; dispatcher encodes the response; Rust supervisor's broadcast routes it to the pending one-shot; the Tauri command resolves with the JSON value; the TS wrapper returns a typed `ProbeResult`.
10. Store transitions to `READY`; React re-renders `ActiveFileCard` with metadata. The status icon swaps from spinner to green dot with an opacity fade.

#### Error path (corrupt file)

1. Steps 1–7 same.
2. ffprobe exits non-zero with `"Invalid data found when processing input"` in stderr.
3. `runner.Run` returns a non-nil error. `errors.MapRunError` classifies via stderr-tail substring match → `CORRUPT_MEDIA`.
4. `media.Probe` returns `(nil, *ipc.RPCError{Code:"CORRUPT_MEDIA", ...})`.
5. `ProbeHandler` emits `probe_failed{code:"CORRUPT_MEDIA"}` log event and returns the RPC error.
6. Dispatcher encodes an error envelope; the IPC client surfaces `IpcError::Other{code:"CORRUPT_MEDIA"}`; the Tauri command boundary maps it to `SerializableIpcError{code:"CORRUPT_MEDIA", message:"..."}`.
7. Frontend `client.probe()` throws `IpcError`; the store catches and transitions to `ERROR` with the error attached.
8. `ActiveFileCard` renders `ErrorPanel` variant for `CORRUPT_MEDIA` (PRD §8.5 message + Retry/Remove CTAs).

#### Retry

1. User clicks Retry. Store transitions ERROR → RETRYING; runs `client.probe(path)` again.
2. Outcome routes the store to `READY` or back to `ERROR`.

#### Replace file

1. User drops a new file while workspace is non-`EMPTY`. `ReplaceFileDialog` confirms.
2. Store sets status to `REMOVED` momentarily (single React tick) to ensure components unmount cleanly, then `loadFile(newPath)` runs the standard PROBING → READY/ERROR flow.

#### Cold start (FFPROBE_MISSING)

1. App launches; `Supervisor::spawn` resolves `binaries/ffprobe-<target-triple>`; if absent, returns `IpcError::Other{code:"FFPROBE_MISSING"}`.
2. `lib.rs::run` (in `setup`) propagates the error; the Tauri window still mounts (because supervisor failure does not panic the app — it returns an error from `Supervisor::spawn` which is logged and the supervisor state is `None`).

  **Snap-call resolved during drafting:** the existing `Supervisor::spawn` returns `Result<Supervisor>` and the setup uses `?`, so the app currently **does** panic at startup if the sidecar binary is missing `[verified: app/src-tauri/src/lib.rs:21-29]`. Keep that behaviour for FFPROBE_MISSING — it's a misconfigured dev/CI environment, not a user-visible runtime path. The supervisor pre-flight check is appropriate.

---

## 4. Open Questions

All resolved during interactive drafting (OQ-1…OQ-10 from the research doc). Remaining items are **assumptions** that surfaced during LLD verification; please confirm them before handing this doc to `lld-to-plan`.

- **Assumption:** macOS Gatekeeper co-signing of the bundled `ffprobe` is in scope for Phase 3 but **does not require full notarisation** — only the ad-hoc / Developer-ID codesign step needed for first-launch invocation under the existing (unsigned) Tauri build. Full notarisation lives in a later release-hardening phase.
- **Assumption:** the `audio` field on `MediaProbeResult` is **omitted** (rather than `null`) when no audio stream exists. This is consistent with the existing schema convention (`additionalProperties: false`, fields marked optional). The handler returns the error `NO_AUDIO_STREAM` for supported containers without audio; `audio` is omitted in the rare case where a downstream caller asks for an unsupported-container-error result (where the response is an error envelope anyway, so `audio` is moot).
- **Assumption:** "ffprobe target-triple matching" at supervisor spawn uses `cfg!(target_os = "...")` + `cfg!(target_arch = "...")` rather than reading the runtime triple. Cross-compilation produces a binary with the build-time triple baked in, so this is correct for any reasonable build configuration.
- **Assumption:** the `"Browse files"` flow requires `tauri-plugin-dialog` (new dependency) and the `dialog:default` capability. The current capability set does not include it. Confirm adding both.
- **Assumption:** the `app/src/components/AboutPanel.tsx` license-surface work is deferred out of Phase 3 in favor of README + `third_party/LICENSE.ffmpeg-lgpl.txt` only. PRD §11.6 names README, about screen, and licenses page — confirm we can defer the in-app surfaces to a follow-up.
- **Assumption:** the existing string-error path in `app/src/ipc/client.ts::toIpcError` stays in place (defensive fallback) even though all current Tauri commands now return `SerializableIpcError`. Removing the string branch would break compatibility if any future command forgets the structured return. Confirm keeping the fallback.
- **Assumption:** unsupported files surface as **RPC errors** (`UNSUPPORTED_CONTAINER`, `UNSUPPORTED_CODEC`, `NO_AUDIO_STREAM`) rather than success-with-`supported=false`. See the design note in §3.3's `media.go` block. Confirm this contract before generated code lands.

---

## 5. Risks / Trade-offs

| Risk / trade-off | Mitigation |
|---|---|
| **macOS Gatekeeper flags bundled ffprobe on first launch.** | Co-sign ffprobe during the Tauri build via post-build script. Document the `xattr -d com.apple.quarantine` manual fallback in `docs/troubleshooting.md`. |
| **Windows long-path edge cases (>260 chars).** | `runner.Run` prefixes the media path with `\\?\` on Windows when `len(path) > 240` (heuristic; the actual MAX_PATH boundary is 260 but we want headroom). Integration test in CI covers a path >280 chars. |
| **ffprobe version skew vs. parser.** | Parser uses `encoding/json` without `DisallowUnknownFields`. Lock file pins the upstream build; renovate-style updates require a manual lock bump + CI test pass. |
| **Subprocess orphans on crash.** | `runner.go` runs ffprobe in a new process group (Unix) / process group with CREATE_NEW_PROCESS_GROUP (Windows) and uses pgid-kill / `taskkill /T` on cancel. Integration test "kill the parent process while ffprobe is running" must show no orphan ffprobe. |
| **Bundle size grows ~60 MB across 3 platforms.** | Acceptable for a desktop creator tool. Documented in `docs/adr/2026-05-16-ffprobe-bundling.md`. |
| **Tauri permission scope creep.** | Webview never invokes ffprobe directly. Only the new `dialog:default` is added; `shell:allow-spawn` stays scoped to `studio-sidecar`. |
| **Trade-off: unsupported files as RPC errors vs success-with-supported=false.** | Chose RPC errors for the simpler UI contract (one axis: ok/err) and PRD §11.4 fidelity. Downside: a tool that wants the full `MediaProbeResult` for an unsupported file (e.g. diagnostics-only) can't get it. Phase 3 has no such consumer; revisit if needed. |
| **Trade-off: Zustand vs `useReducer + Context`.** | Zustand adds one dependency but is ~1KB and makes the side-effect orchestration on `replaceFile` trivial. Swappable later if the project standardises on something else. |

---

## 6. Edge cases / Error handling

| Case | PRD ref | Planned handling |
|---|---|---|
| Path with non-ASCII / emoji | §13.2 | UTF-8 paths flow end-to-end. Integration test uses `unicode-name-🎥-интервью.mp4`. |
| Windows path >260 chars | §13.3 | `runner.go` prefixes `\\?\` on Windows when path is long. CI integration test. |
| Multiple audio tracks | §13.4 | `normalize.go` selects first stream with `disposition.default == 1`, falls back to lowest-index audio stream. All tracks listed in `audio.tracks[]`. Diagnostics drawer renders the full list with the default flag. |
| Missing duration | §13.5 | `normalize.go` leaves `durationSeconds` unset; UI renders "Unknown duration". Compat does not mark unsupported. |
| Drop multiple files | §9.1 | `useDropTarget` accepts only the first path; UI shows an inline "current version supports one file at a time" message. |
| Drop while probe in flight | §9.1 (replace) | `ReplaceFileDialog` confirms; replace waits for the in-flight probe to settle (no cancellation in P3; OQ-9). UI shows "completing previous probe…" hint. |
| Renamed `.txt → .mp4` | §15.1 | ffprobe exits non-zero or returns no streams. `errors.MapRunError`/`compat.Evaluate` → `CORRUPT_MEDIA`. |
| 0-byte file | (PRD implied) | `os.Stat` returns size=0; runner.Run proceeds; ffprobe returns an error; classified as `CORRUPT_MEDIA`. |
| Permission denied | §8.5 | `os.Stat` returns `ErrPermission` → `ACCESS_DENIED` before spawn. ffprobe's own "permission denied" stderr is also classified to `ACCESS_DENIED`. |
| Symlink chains | (implicit) | Pass-through; ffprobe follows. No symlink resolution in the sidecar. |
| File deleted between drop and probe | §8.5 | `os.Stat` returns `ErrNotExist` → `FILE_NOT_FOUND`. |
| ffprobe hangs (pathological input) | §11.7 / §11.8 | 10s context deadline; process group killed; `FFPROBE_FAILURE` with message "probe exceeded 10s deadline". |
| Oversize ffprobe stdout (>1 MiB) | (impl detail) | Bounded stdout buffer in `runner.go`; classified as `CORRUPT_MEDIA` (extra streams encoded as JSON beyond 1 MiB is pathological; treat as malformed). |
| Bundled ffprobe missing in dev build | §11.8 | Supervisor returns `FFPROBE_MISSING` at startup; app fails to launch (matches existing sidecar-missing behaviour). |
| `media.probe` called while ffprobe still being notarised (macOS) | §11.6 | First invocation triggers a 1–3s Gatekeeper scan; 30s Tauri-side timeout absorbs it. |

---

## 7. Testing Strategy

### Unit tests (Go, `go test ./...`)

- `sidecar/internal/media/locator_test.go`:
  - `ResolveFFprobe` returns the env var value when set and file exists.
  - Returns `ErrFFprobeMissing` when env var is unset or path missing.
  - Does **not** consult `$PATH`.
- `sidecar/internal/media/runner_test.go`:
  - Uses a small test-helper binary (a Go-built `cmd/fakeffprobe`) that emits a controllable stdout / exit code / stderr / runtime, set via env vars.
  - Verifies: success path, non-zero exit + stderr classification, oversize stdout truncation, deadline-exceeded → kill, oversize stderr tail clipped to 4 KiB.
- `sidecar/internal/media/parser_test.go`:
  - Table-driven over captured ffprobe JSON fixtures (committed under `sidecar/internal/media/testdata/`, NOT in `test-assets/`). Covers H.264+AAC, VP9+Opus, multi-track AAC, missing duration, missing audio, missing bit_rate fields, ffprobe ≥7.0 vs ≥6.0 output skew.
- `sidecar/internal/media/normalize_test.go`:
  - Default-track selection (default-flag vs first-by-index).
  - r_frame_rate parsing (`30000/1001` → 29.97).
  - Optional fields omitted when absent.
- `sidecar/internal/media/compat_test.go`:
  - Allow-list table tests: each supported container × codec combination → supported=true.
  - Unsupported container → `DominantCode == "UNSUPPORTED_CONTAINER"`.
  - No audio streams → `NO_AUDIO_STREAM`.
  - Unsupported codec → `UNSUPPORTED_CODEC`.
  - Warnings populated correctly for missing duration / multi-track / bitrate=0.
- `sidecar/internal/media/errors_test.go`:
  - `MapRunError` classifier table: `os.ErrNotExist`, `os.ErrPermission`, `context.DeadlineExceeded`, stderr substrings.
- `sidecar/internal/ipc/handlers/media_probe_test.go`:
  - Validator rejects missing `path`, empty `path`, oversize `path` (>4096).
  - Successful invocation against a tiny fixture via `cmd/fakeffprobe`.

### Integration tests (Go, `//go:build integration`, mirrors `sidecar/internal/ipc/integration_test.go`)

- New file `sidecar/internal/media/integration_test.go`:
  - Spawns the real bundled host-platform ffprobe (path via `STUDIO_FFPROBE_PATH` env, exported in the CI step).
  - For each fixture in `test-assets/`, asserts: duration ±10ms, container.format substring, video codec, audio codec, channels, sample rate, `compatibility.supported`.
  - Asserts deterministic error codes for `corrupt-truncated.mp4` (`CORRUPT_MEDIA`), `tiny-no-audio.mp4` (`NO_AUDIO_STREAM`), `unicode-name-🎥-интервью.mp4` (probe succeeds), a synthesized long Windows path (`CORRUPT_MEDIA` if invalid, else success), missing-file probe (`FILE_NOT_FOUND`).
  - "Kill ffprobe mid-probe" test (using `SIGKILL` on the ffprobe PID picked up from `/proc` / `ps`) — asserts the next probe still succeeds (no orphan / lock).
- An existing-pattern E2E test at `sidecar/internal/ipc/integration_test.go` gains one new case: a `media.probe` envelope round-trip against a fixture, asserting the response envelope shape.

### Rust tests

- `app/src-tauri/src/ipc/error.rs`:
  - `From<IpcError> for SerializableIpcError` produces the expected `(code, message)` for each variant.
  - Round-trips through `serde_json` and back to a hash-map shape with the expected keys.
- `app/src-tauri/src/commands.rs`:
  - `default_timeout("media.probe") == 30s`.
- `app/src-tauri/src/ipc/supervisor.rs`:
  - Existing tests untouched.
  - New test: `spawn_child` builds a command with `STUDIO_FFPROBE_PATH` set to the resolved path. (Skipped on CI if ffprobe absent; the test fakes the binary by creating a stub `binaries/ffprobe-<triple>` text file.)

### Frontend tests (Vitest + jsdom)

- `app/src/state/workspace.test.ts`:
  - State machine: every legal transition (EMPTY → FILE_LOADED → PROBING → READY; → ERROR → RETRYING → READY/ERROR; READY → REMOVED → EMPTY) fires correctly.
  - Illegal transitions (e.g. EMPTY → READY) are no-ops or throw (TBD: throw — caught one extra contract surface).
  - `loadFile` calls a mocked `client.probe` once; `replaceFile` confirmation gate is honored.
- `app/src/ipc/client.test.ts`:
  - `toIpcError` accepts structured `{ code, message }` objects without mutation.
  - `toIpcError` falls back to `{ code: "UNKNOWN", message: ... }` for string rejections.
  - `probe(path)` invokes `media_probe` with `{ path }`.
- `app/src/components/DiagnosticsDrawer.test.tsx`:
  - Renders all metadata fields for a fixture `ProbeResult`.
  - Renders the multi-track audio list when `audio.tracks.length > 1`.
  - Closes on Escape, outside-click, and explicit X button.
  - "Copy diagnostics" swaps to checkmark + toast for 2s.
- `app/src/components/ErrorPanel.test.tsx`:
  - For each error code, renders the PRD §8.5 message and the correct CTAs.
- `app/src/components/ActiveFileCard.test.tsx`:
  - Status indicator matches store status (Gray/Spinner/Green/Yellow/Red dots + labels).
  - Retry button visibility tied to ERROR state.
- Accessibility smoke (axe-core via `@axe-core/react` or `vitest-axe`): tab order through Empty → Card → Drawer, no critical violations.

### Manual QA (PRD §15.1 matrix)

- Cross-platform smoke: drop each fixture on Windows + macOS Intel + Apple Silicon. Expected verdicts per fixture documented in `test-assets/README.md`.
- 4 GiB file probe — manual only (too big for CI). Asserts <3s probe time and no UI freeze.
- 50 simultaneous drops — manual (not in CI). UI honors single-file MVP (first accepted, others ignored).
