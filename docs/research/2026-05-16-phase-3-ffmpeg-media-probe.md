# Phase 3 — FFmpeg Bundling & Media Probe — Code Research & High-Level Design

**Status:** Draft (research / HLD)
**Date:** 2026-05-16
**PRD:** [docs/reqs/phase_3_ffmpeg_media_probe_prd_and_ui_design.md](../reqs/phase_3_ffmpeg_media_probe_prd_and_ui_design.md)
**Implementation plan reference:** [docs/implementation_plan.md](../implementation_plan.md) §"Phase 3" (lines 111–131) and §"Phase 5" (lines 160–179)
**Prior LLD (foundations only):** [docs/lld/2026-05-11-phase-0-bootstrap.md](../lld/2026-05-11-phase-0-bootstrap.md)
**IPC contract:** [docs/ipc-contract.md](../ipc-contract.md) and [docs/adr/2026-05-14-ipc-contract.md](../adr/2026-05-14-ipc-contract.md)

This document captures what exists in the codebase today, then proposes a high-level approach for Phase 3. Detailed file/function-level design is deferred to a follow-up LLD.

---

## 1. Summary

Phase 3 turns the existing Phase 0/1 desktop shell into a single-file media workspace that can ingest a creator video, run `ffprobe` against it locally, and surface trusted metadata plus a compatibility verdict. The PRD also describes the surrounding ingest UI (empty state, workspace card, diagnostics drawer, error states, replace-file flow). Under the new "single-file MVP" product decision, that UI work appears to fold into Phase 3 rather than the older parallel Phase 5 plan (see open question OQ-1).

The design splits cleanly along the existing process boundary:

- **Sidecar (Go)** owns subprocess management, ffprobe invocation, JSON parsing, normalization into the canonical `MediaProbeResult`, and the error taxonomy. It is the only layer that knows about ffprobe.
- **Tauri command layer (Rust)** owns binary path resolution, capability gating, and the typed Tauri command wrapper around `media.probe`. It does not know what ffprobe is.
- **Frontend (React/TS)** owns the workspace state machine, drag-and-drop, the workspace card, diagnostics drawer, and error UX. It consumes the canonical `MediaProbeResult` shape and renders it.

Schemas drive the contract across all three sides via the existing `npm run gen:schemas` pipeline ([scripts/gen-schemas.mjs:1](../../scripts/gen-schemas.mjs)).

---

## 2. Current Codebase Findings

Verified against the tree at `master`:

- The repo has a Tauri 2 + React/TS shell in [app/](../../app) and a Go sidecar in [sidecar/](../../sidecar), bridged by NDJSON over stdin/stdout. [verified: app/src-tauri/tauri.conf.json:36, sidecar/internal/ipc/dispatcher.go:46]
- The IPC contract is mature: schemas in [schemas/](../../schemas) are the single source of truth; codegen produces TS, Go, and Rust types via [scripts/gen-schemas.mjs:12-17](../../scripts/gen-schemas.mjs). [verified]
- Three system methods exist today: `system.ping`, `system.echo`, `system.shutdown`. No `media.*` namespace yet. [verified: sidecar/internal/cli/cli.go:52-56]
- The Rust supervisor handles spawn, NDJSON framing, broadcast fan-out, request multiplexing, capped-pending pool (64), per-method timeouts, and auto-restart with exponential backoff. [verified: app/src-tauri/src/ipc/supervisor.rs:40-46, app/src-tauri/src/ipc/client.rs:30]
- Tauri commands currently return `serde_json::Value` on success and stringified errors via `.map_err(|e| e.to_string())`. The frontend wraps this in an `IpcError { code, message }` but in practice only ever sees `code: "UNKNOWN"` because the original `IpcError::Other { code, … }` is flattened. The code comments call out that "Phase 6 will introduce structured error serialisation" ([app/src-tauri/src/commands.rs:11](../../app/src-tauri/src/commands.rs), [app/src/ipc/client.ts:42](../../app/src/ipc/client.ts)).
- The Go dispatcher enforces a 64-handler in-flight cap (`maxConcurrentDispatch`), recovers from panics, and validates payloads via JSON Schema using the `santhosh-tekuri/jsonschema/v5` library. [verified: sidecar/internal/ipc/dispatcher.go:44, sidecar/internal/ipc/validator.go]
- Handler pattern is well-established: schema file → generated types → handler in `internal/ipc/handlers/<name>.go` → register in `cli.Run`. [verified: docs/ipc-contract.md:152-229]
- The sidecar is bundled per-platform via Tauri's `externalBin` mechanism. Capability `shell:allow-spawn` is whitelisted only for the sidecar with `args: ["serve"]`. [verified: app/src-tauri/tauri.conf.json:36, app/src-tauri/capabilities/default.json:8-19]
- Cross-platform sidecar builds are produced by [scripts/build-sidecar.mjs:10-26](../../scripts/build-sidecar.mjs) for `windows/amd64`, `darwin/amd64`, `darwin/arm64`. No bundled third-party binaries exist yet — `app/src-tauri/binaries/` contains only `.gitkeep`. [verified: ls output]
- The current frontend UI is the Phase 0 placeholder ([app/src/App.tsx:27-38](../../app/src/App.tsx)) plus an opt-in Diagnostics panel toggled by `Ctrl+Shift+D` in dev builds. There is no drop zone, file row, drawer, or modal. No state-management library (Zustand/Redux) is present; only React's built-in hooks.
- No state-store or media model exists. `test-assets/` exists but is empty (`.gitkeep` only). [verified]
- CI runs on Windows + macOS Intel + macOS Apple Silicon, builds all three sidecar binaries, and asserts codegen produces a clean tree. [verified: .github/workflows/ci.yml]
- `docs/implementation_plan.md` originally framed Phase 3 as just ffprobe bundling + IPC + a minimal "Probe a file" diagnostics screen, with the full ingest UI living in Phase 5 (lines 160–179). The new PRD's "single-file MVP" decision and rich UI sections (8.x, 9.x) appear to fold P5's UI work into P3.

---

## 3. High-Level Architecture

```
┌────────────────────────────────────────────────────────────────────┐
│  Frontend (React/TS)                                               │
│  ┌──────────────┐  ┌──────────────┐  ┌──────────────────────────┐  │
│  │ Drop zone /  │→ │ Workspace    │→ │ Diagnostics Drawer       │  │
│  │ Empty state  │  │ state store  │  │ + Error UX + Retry       │  │
│  └──────────────┘  └──────┬───────┘  └──────────────────────────┘  │
│                           │                                        │
│                           ▼                                        │
│              app/src/ipc/client.ts :: probe(path)                  │
└─────────────────────────────────────┬──────────────────────────────┘
                                      │ invoke("media_probe", { path })
┌─────────────────────────────────────▼──────────────────────────────┐
│  Tauri (Rust)                                                      │
│  ┌────────────────────────────────────────────────────────────┐    │
│  │ ipc::commands::media_probe                                 │    │
│  │   • per-method timeout (probe budget, e.g. 30s)            │    │
│  │   • (optional) structured-error pass-through               │    │
│  └─────────────────────┬──────────────────────────────────────┘    │
│                        ▼                                           │
│         IpcClient::call("media.probe", payload)                    │
│                        ▼                                           │
│         Supervisor (NDJSON over stdin/stdout)                      │
│         • Resolves and exports STUDIO_FFPROBE_PATH to child env    │
└────────────────────────┬───────────────────────────────────────────┘
                         │
┌────────────────────────▼───────────────────────────────────────────┐
│  Sidecar (Go)                                                      │
│  ┌────────────────────────────────────────────────────────────┐    │
│  │ ipc/handlers/media_probe.go                                │    │
│  │   • schema-validate payload                                │    │
│  │   • call internal/media.Probe(ctx, path)                   │    │
│  └─────────────────────┬──────────────────────────────────────┘    │
│                        ▼                                           │
│  internal/media/                                                   │
│   • locator.go   — finds ffprobe via STUDIO_FFPROBE_PATH / lookup  │
│   • runner.go    — spawns ffprobe, bounded I/O, timeout, cancel    │
│   • parser.go    — ffprobe JSON → typed Go structs                 │
│   • normalize.go — codec/container canonicalisation                │
│   • compat.go    — supported / issues / warnings policy            │
│   • errors.go    — maps runner/parser failures → RPC error codes   │
└────────────────────────┬───────────────────────────────────────────┘
                         │ executes
                         ▼
                ffprobe (bundled per-platform)
```

### 3.1 Data flow for the happy path

1. User drops a file. Frontend captures the path, transitions workspace to `PROBING`, renders the workspace card with a spinner within <100ms.
2. Frontend calls `probe(path)` → Tauri `media_probe` command → `IpcClient::call("media.probe", { path }, timeout=30s)`.
3. Supervisor sends NDJSON envelope to sidecar stdin.
4. Sidecar handler validates payload, calls `media.Probe(ctx, path)`.
5. `media.Probe` resolves ffprobe binary (env var first, then default search), spawns it with `-v error -hide_banner -print_format json -show_format -show_streams`, reads stdout with a bounded buffer, waits with a per-probe timeout (e.g. 10s).
6. Parser decodes ffprobe JSON into typed structs; normalizer maps codec names (e.g. `h264` → "H.264"); compatibility evaluator emits `supported`, `issues`, `warnings`.
7. Handler returns the canonical `MediaProbeResult`; dispatcher wraps it in a response envelope; Rust resolves the pending future; frontend store transitions to `READY` and re-renders.

### 3.2 Error path

A failure at any stage produces a typed RPC error in the response envelope. The frontend store maps each code to the PRD's user-facing message and CTA (PRD §8.5):

| Code | Source | User message |
|---|---|---|
| `FILE_NOT_FOUND` | runner stat | "We couldn't locate this file." |
| `ACCESS_DENIED` | runner stat | "We don't have permission to access this file." |
| `UNSUPPORTED_CONTAINER` | compat | "This file type isn't supported yet." |
| `UNSUPPORTED_CODEC` | compat | "We can't read this audio format." |
| `NO_AUDIO_STREAM` | compat | "No audio track detected." |
| `CORRUPT_MEDIA` | parser / nonzero exit + stderr classifier | "We couldn't read this file." |
| `FFPROBE_FAILURE` | runner spawn / unexpected exit | "Internal media probe error." |
| `UNKNOWN` | catch-all | generic fallback |

These codes need to be added to the existing reserved-error-codes table in [docs/ipc-contract.md](../ipc-contract.md). They follow the existing `SNAKE_CASE` convention.

---

## 4. Component-by-Component Approach

### 4.1 FFprobe bundling

**Goal:** Ship a pinned `ffprobe` per platform alongside the sidecar so the app works fully offline with no system FFmpeg.

**Recommended approach — Option A: Bundle as Tauri `externalBin`, fetched at build time:**

- Add a `scripts/fetch-ffprobe.mjs` that downloads platform-specific binaries from a vetted LGPL-safe source (candidates: BtbN/FFmpeg-Builds LGPL builds, evermeet.cx for macOS, gyan.dev for Windows) and verifies SHA-256 checksums against a committed manifest.
- Stage the binaries into `app/src-tauri/binaries/` with target-triple suffixes that match the sidecar pattern, e.g.:
  - `ffprobe-x86_64-pc-windows-msvc.exe`
  - `ffprobe-x86_64-apple-darwin`
  - `ffprobe-aarch64-apple-darwin`
- Append `ffprobe` to `bundle.externalBin` in `tauri.conf.json` so Tauri ships them in the installer.
- Add a `shell:allow-execute` (or equivalent) permission scoped to the bundled ffprobe sidecar entry in `capabilities/default.json`, OR (preferred) avoid letting the webview spawn ffprobe at all — only the Go sidecar invokes it, so we just need the Rust supervisor to resolve its on-disk path and pass it to the Go child via env var. In that case the capability isn't needed because Tauri's shell plugin isn't doing the spawn.
- The Rust supervisor uses `app.path().resolve(...)` (or `tauri::api::path::resource_dir`) to find the bundled path, exports it to the sidecar child via env var `STUDIO_FFPROBE_PATH`, alongside the existing `STUDIO_LOG_FILE`.
- CI step: `npm run sidecar:build` already exists; add `npm run ffprobe:fetch` (or roll into setup) that uses the manifest. Cache the downloads in CI to avoid repeated network fetches.
- License compliance: commit the LGPL text and a `THIRD_PARTY_NOTICES.md` per PRD §11.6 "must be documented in: README, about screen, licenses page".

**Alternative — Option B: Build FFmpeg from source.**
LGPL-clean but heavyweight, slow CI, and overkill for an ffprobe binary that is widely distributed in stable LGPL builds. Reject unless legal review requires it.

**Alternative — Option C: Embed via Go cgo bindings (e.g. `goav`).**
Compiles ffmpeg/libav into the sidecar binary itself. Pros: single artifact, no subprocess. Cons: cgo, large binary, harder cross-compile (especially Windows), licensing surface harder to audit. Reject for Phase 3.

**Open question OQ-3** below: do we bundle just `ffprobe`, or also the `ffmpeg` binary (needed in P4 anyway)?

### 4.2 Sidecar — media package

A new `sidecar/internal/media/` package with focused, testable units:

- `locator.go` — resolves the ffprobe executable. Order: `STUDIO_FFPROBE_PATH` env var → relative to sidecar exe (for dev) → return locator error (no `PATH` lookup; we never want to silently use a system ffprobe of unknown version/origin).
- `runner.go` — wraps `os/exec.CommandContext` to invoke ffprobe with a fixed argument set, bounded stdout buffer (cap to e.g. 1 MiB to defend against pathological output), per-probe timeout via context, kills the process on cancel, returns `(stdout []byte, exitCode int, stderrTail string, err error)`. Importantly: never blocks the dispatcher's goroutine on a hung child — the context deadline must terminate.
- `parser.go` — defines Go structs that mirror ffprobe's JSON (`format`, `streams[]` with type-specific fields), decodes via `encoding/json` with `DisallowUnknownFields=false` to tolerate ffprobe version skew. Unit-tested against captured JSON samples.
- `normalize.go` — maps raw ffprobe codec/container strings to the canonical `MediaProbeResult` shape (PRD §11.3). Selects the default audio track per PRD §13.4.
- `compat.go` — applies the supported-format policy (allow-list of containers and codecs from PRD §15.1 + implementation_plan.md). Returns `(supported bool, issues []string, warnings []string)` and the dominant failure code.
- `errors.go` — small mapping helpers from runner errors / parser errors / compat failures to `ipc.RPCError` values.
- `media.go` — top-level `Probe(ctx, path) (*MediaProbeResult, error)` that wires the pieces.

This package has zero IPC dependency — it is a pure media library. The IPC handler in `internal/ipc/handlers/media_probe.go` is a thin adapter (schema validate → call `media.Probe` → return result or RPC error).

**Concurrency:** the existing dispatcher cap of 64 in-flight requests ([sidecar/internal/ipc/dispatcher.go:44](../../sidecar/internal/ipc/dispatcher.go)) is already well above the PRD's "5 simultaneous probes" target. No new gate needed; each probe runs in its own goroutine. (Note: PRD §11.10 says "one active probe job" while §11.7 says "Probe concurrency: 5 simultaneous". This is internal architectural slack to absorb a retry-in-flight; user-facing UX remains single-file.)

**Subprocess hygiene:** PRD §11.8 "Must Always: clean subprocesses, no orphan ffprobe processes." We get most of this from `exec.CommandContext`, but on Windows the default behavior leaves grandchildren orphaned if ffprobe itself spawned helpers. Defensive measure: on Windows, configure `SysProcAttr.CreationFlags = windows.CREATE_NEW_PROCESS_GROUP` and call `taskkill /T` on cancel; on Unix, set a process group and `syscall.Kill(-pgid, SIGKILL)` on cancel. Document this in the LLD.

### 4.3 IPC — schema, handler, errors

- **Schema:** new `schemas/media.probe.schema.json` defining a `ProbePayload` (`{ path: string }`) and `ProbeResult` matching the PRD's `MediaProbeResult` shape (PRD §11.3). Use `additionalProperties: false` and bounded string lengths to keep with existing conventions.
- **Generated types:** `npm run gen:schemas` regenerates TS, Go, and Rust types in the same way as existing methods.
- **Handler:** `sidecar/internal/ipc/handlers/media_probe.go`, registered in `cli.Run`'s `case "serve":` block.
- **Error codes:** add the seven new codes listed in §3.2 to `sidecar/internal/ipc/errors.go` and to the reserved-codes table in [docs/ipc-contract.md](../ipc-contract.md).
- **No new envelope features needed** — existing v=1 envelope carries everything.

### 4.4 Tauri command layer

- Add `media_probe(path: String)` command in `app/src-tauri/src/commands.rs`, registered in `lib.rs::run`.
- Per-method timeout: budget ~30s (well above the 3s PRD §11.7 target, but allows for big files + first-run cold start of ffprobe on macOS where Gatekeeper may scan it).
- Update `default_timeout` table accordingly.
- Update `capabilities/default.json`: no change needed for the webview unless we decide to let the webview spawn ffprobe directly (we won't — see §4.1). The webview only invokes Tauri commands, and Tauri commands talk to the supervisor.
- The supervisor changes: in `spawn_child`, after resolving the log file path, also resolve the ffprobe binary path (target-triple suffixed under the app bundle's `binaries/` dir) and set `STUDIO_FFPROBE_PATH` in the child's environment. Surface a clear `IpcError::Other { code: "FFPROBE_MISSING", ... }` if the binary is absent at supervisor spawn time (this catches dev builds where someone forgot to run `npm run ffprobe:fetch`).

### 4.5 Frontend — state, ingest, UI

**State store:** a single workspace store with the state machine from PRD §12:

```
EMPTY → FILE_LOADED → PROBING → READY → REMOVED → EMPTY
                          ↓
                        ERROR → RETRYING → READY | ERROR
```

Recommend a small Zustand store (Zustand is ~1KB, well-suited for a single in-memory workspace and easy to test). If the project later wants to standardize on Redux Toolkit or Jotai, swapping is mechanical. Alternative: plain `useReducer` in a Context — fine but less ergonomic for the side-effect orchestration (cancel-in-flight probe on replace, etc.).

**Components (all new under `app/src/components/`):**

- `WorkspaceShell` — owns the workspace state, top-level layout (header / workspace / footer).
- `EmptyState` — drop zone + browse CTA + supported-formats + privacy line (PRD §8.2).
- `ActiveFileCard` — single file with thumbnail (see OQ-2 re poster frames), filename, summary line, status indicator, Details/Retry/Remove actions (PRD §8.3).
- `DiagnosticsDrawer` — right-side slide drawer with full metadata, multi-track audio list, copy-diagnostics button + success toast (PRD §8.4).
- `ReplaceFileDialog` — modal confirming destructive replace when a file is already active (PRD §9.1).
- `ErrorPanel` (variants for unsupported / corrupt / no-audio / missing / access-denied / generic) reusing a base error component (PRD §8.5).

**Drag-and-drop:** Tauri 2 exposes `getCurrentWebview().onDragDropEvent()` for OS-level drops; HTML5 drag/drop also works inside the webview. Use the Tauri event for correctness on Windows (HTML5 drop can be flaky in webview2 with certain file types). Reject non-file drops and multiple files (PRD §9.1).

**File chooser:** use `@tauri-apps/plugin-dialog` with a filter on `.mp4/.mov/.mkv/.webm`. (This adds a new Tauri plugin dependency — minor capability change.)

**Accessibility:** PRD §10 — every status maps to an icon + text label, drawer responds to Escape and outside-click, full keyboard tab order, ARIA labels for screen readers.

**Visual system:** PRD §14 sketches tokens (radius, spacing, motion). Define them in a small `app/src/styles/tokens.css` plus per-component CSS modules or a tiny utility CSS. No need for Tailwind or a UI library in Phase 3 (deferred to a later design system pass if the team wants one).

### 4.6 IPC error propagation to the frontend (UI-driven decision)

PRD §8.5 requires the UI to distinguish error codes in order to render the right user message + CTA. Today, Tauri command errors are flattened to strings ([app/src-tauri/src/commands.rs:11](../../app/src-tauri/src/commands.rs)). Two viable approaches:

**Approach A (recommended): upgrade now to structured error serialisation.** Define a `SerializableIpcError { code, message, details }` in Rust that derives `Serialize`, and change the commands' return type to `Result<Value, SerializableIpcError>`. The frontend `IpcError` type already matches. This is exactly what the existing code comment defers to "Phase 6"; surfacing it now is cheap (~30 LOC), removes a parsing-of-error-strings smell, and unblocks every later phase's error UX.

**Approach B: keep strings, parse on the frontend.** Embed `"{code}: {message}"` and parse it in `toIpcError` in `app/src/ipc/client.ts`. Works but is fragile. Reject unless there's a reason to defer.

This decision affects scope but not architecture — surface as a callout in the LLD.

### 4.7 Testing strategy

- **Sidecar unit tests** (Go, `go test ./...`):
  - `parser_test.go` — table-driven tests over captured ffprobe JSON for fixture files (committed JSON, not media), proves the parser handles AAC, Opus, PCM, missing fields, multiple audio tracks, missing duration.
  - `compat_test.go` — exhaustive cases for the supported/unsupported policy.
  - `normalize_test.go` — codec/container canonicalisation.
  - `runner_test.go` — fakes ffprobe via a tiny test helper binary or a mocked `exec.LookPath` to verify timeout, cancellation, oversize output, nonzero exit + stderr classification.
- **Sidecar integration tests** (Go with build tag `integration`, mirrors existing pattern at [sidecar/internal/ipc/integration_test.go](../../sidecar/internal/ipc/integration_test.go)):
  - Spawns the real bundled ffprobe against a small set of fixture media files in `test-assets/`.
  - Asserts duration, codec, channels, sample rate per fixture.
  - Asserts deterministic error codes for the corrupt and renamed-`.txt` cases.
- **Test fixtures:** rather than committing media binaries (large; LFS-unfriendly for a small project), generate them at test time via the bundled ffmpeg/ffprobe — but ffprobe alone can't generate media. Two paths: (a) commit a tiny ~10-30 MB corpus of CC0/CC-BY clips under `test-assets/`, or (b) use a one-time "synthesize fixtures" Make target that requires ffmpeg installed on the dev machine. Lean toward (a) — small enough not to bloat the repo, deterministic, no extra dev prereq.
- **Rust tests:** standard `#[cfg(test)]` modules in `commands.rs` for the new probe command + supervisor env-var pass-through.
- **Frontend tests** (Vitest + jsdom):
  - Workspace state machine transitions (drop → probing → ready / error / retry / replace).
  - Error code → user message mapping.
  - Diagnostics drawer renders multi-track audio.
  - Accessibility smoke (keyboard tab order, escape closes drawer).
  - No mocking of Tauri `invoke` beyond a thin stub; mock at the `app/src/ipc/client.ts` boundary.

### 4.8 Observability

The existing structured slog logger ([sidecar/internal/logger/logger.go](../../sidecar/internal/logger/logger.go)) is sufficient. Emit the events PRD §11.9 specifies — `probe_started`, `probe_completed` (with duration_ms), `probe_failed` (with code) — and a `ffprobe_invocation` debug event with cmd line and exit code for support cases. Never log file paths at non-debug levels (privacy / PII).

---

## 5. Cross-Cutting Concerns

### 5.1 Path handling

PRD §13.2-§13.3 list Unicode paths and >260-char Windows paths. The Go stdlib handles Unicode on all platforms via UTF-8 paths. Windows long-path support needs:
- Use `\\?\` prefix when invoking ffprobe with paths near or over 260 chars — Go's `os/exec` doesn't add this automatically.
- Verify Tauri's path-string round-trip preserves non-ASCII characters on Windows (recent Tauri versions do, but worth a fixture test).

### 5.2 Subprocess lifecycle

Covered in §4.2. Test plan must include "kill -9 the sidecar mid-probe" and "kill the parent process while ffprobe is running" — the latter must not leave a zombie ffprobe.

### 5.3 Security / capability surface

No new web-facing surface. The webview never reaches ffprobe directly; only the Go sidecar (spawned by the supervisor) can. This keeps the Tauri allow-list minimal: we keep `shell:allow-spawn` scoped to the sidecar and (optionally) add `dialog:allow-open` for the Browse-files flow.

### 5.4 macOS gatekeeper / quarantine

A first-launch invocation of an unsigned bundled ffprobe will hit Gatekeeper. Options: (a) co-sign ffprobe as part of the Tauri build (requires entry in `Cargo.toml`/`tauri.conf.json` signing config or a post-build script), (b) strip quarantine attribute at install time, (c) document the manual `xattr -d com.apple.quarantine` workaround. Recommend (a) wired into the existing macOS notarization story (which is itself deferred to a later phase per the LLD non-goals).

### 5.5 Bundle size

A single LGPL `ffprobe` binary is ~15-25 MB per platform. Bundling all three target binaries plus the sidecar adds ~80 MB total to the installer. Acceptable for a desktop creator tool, but worth confirming. If `ffmpeg` is also bundled (see OQ-3), expect another ~25 MB per platform.

### 5.6 Reproducibility / LGPL audit

PRD §17 lists LGPL licensing as a risk. The mitigation: pinned upstream URL + SHA-256 checksum in a `third_party/ffprobe.lock.json` (or similar), committed to git, verified by the fetch script. README + about screen + licenses page reference the chosen build's source URL, version, and license text.

---

## 6. Phasing Within Phase 3

If the work needs to be broken into incremental landings, a natural sequence:

1. **Bundling first** — fetch script, manifest, CI integration, capability tweak, supervisor env-var pass-through. Smoke-tested by a no-op `media.probe` handler that just returns `{ pong: "ffprobe at <path>" }`.
2. **Sidecar media package + IPC** — full `media.probe` end-to-end against fixtures. UI is still the placeholder; a developer tool (dev-only diagnostics screen) exercises it.
3. **Frontend workspace** — empty state, drop zone, workspace card, basic ready/error rendering wired to real `media.probe`.
4. **Diagnostics drawer + error UX polish** — full PRD UI conformance, accessibility, copy-diagnostics, retry, replace-file modal.
5. **Edge-case hardening** — Unicode paths, long Windows paths, missing duration, multi-track audio default selection, subprocess kill tests.

Each step is independently shippable behind a flag if needed, but the natural cut is one PR per step (LLD/plan will refine).

---

## 7. Recommended Approaches Summary

| Decision | Recommendation | Rationale |
|---|---|---|
| FFprobe sourcing | Download pinned LGPL builds at build time; verify by SHA-256 | Lowest CI cost; clean license posture; no system FFmpeg dependency |
| FFprobe → sidecar discovery | Supervisor sets `STUDIO_FFPROBE_PATH` env var | Mirrors existing `STUDIO_LOG_FILE` pattern; avoids `PATH` ambiguity |
| Subprocess invocation | `exec.CommandContext` with bounded stdout + per-probe timeout + process-group kill | Matches PRD §11.8 reliability bar |
| IPC method | Single `media.probe` returning canonical `MediaProbeResult` | Mirrors PRD §11.3; future phases (extract/remux) add siblings under `media.*` |
| Error propagation | Upgrade Tauri commands to structured `SerializableIpcError` now | Required for PRD §8.5 user-message mapping; trivial change; unblocks future phases |
| Frontend state | Small Zustand store + state machine | Single source of truth; testable; tiny |
| UI scope | Treat PRD §8/§9 as in-scope for Phase 3 (folds prior P5 work in) | PRD's single-file MVP eliminates the queue UI P5 would have built; doing both phases together avoids throwaway mocked-data work |
| Test fixtures | Commit small CC0/CC-BY clips under `test-assets/` | Deterministic; no extra dev prereq |
| Concurrency cap | Reuse existing 64 dispatcher cap; no new gate | PRD §11.7 "5 simultaneous" is internal slack, not user-facing |
| Poster frame thumbs | Defer pending OQ-2; if accepted, requires `ffmpeg`, not just `ffprobe` | Affects bundling scope |

---

## 8. Open Questions

These are listed in priority order. The first three meaningfully affect scope; the rest are confirmations or implementation-detail preferences.

- **OQ-1 (scope, high impact):** The PRD's UI sections (§8, §9, §13.4) describe a complete workspace UI — empty state, drop zone, workspace card, diagnostics drawer, replace-file modal, error UX, accessibility. The earlier `docs/implementation_plan.md` framed this as Phase 5 work that mocks Phase 3's `media.probe`. Under the new "single-file MVP" decision, does Phase 3 now also deliver the full workspace UI, or does it ship only the `media.probe` IPC + a minimal diagnostics screen, leaving the workspace UI to a separate landing?
- **OQ-2 (scope):** PRD §3 lists "thumbnail extraction beyond lightweight poster frame support" as a non-goal — implying poster frames *are* in scope. The mockups (PRD §8.1, §8.3) show thumbnails. But generating poster frames requires `ffmpeg`, not just `ffprobe`. Should Phase 3 deliver poster frames (and therefore bundle ffmpeg), or render a static icon placeholder until Phase 4 brings full ffmpeg?
- **OQ-3 (bundling):** Related to OQ-2: do we bundle only `ffprobe` in Phase 3 (smaller, matches §11.6 literally) and add `ffmpeg` in Phase 4, or bundle both now to avoid revisiting the licence-audit + fetch-script work twice? The implementation_plan.md text says "FFmpeg static binary bundled per platform", suggesting both — but PRD §11.6 says just ffprobe.
- **OQ-4 (IPC error UX):** Approach A in §4.6 upgrades Tauri command error serialisation now (≈30 LOC) so the frontend can distinguish error codes for PRD §8.5 user-message mapping. The code comments defer this to "Phase 6". Confirm we surface it now rather than parse error strings.
- **OQ-5 (FFmpeg version pin):** PRD §11.6 says "FFmpeg 7.x". Pick a specific upstream build family (e.g. BtbN/FFmpeg-Builds 7.1 LGPL static) and commit the URL + checksum manifest? Any constraints from legal/compliance on the build source?
- **OQ-6 (state library):** Zustand vs `useReducer` + Context vs Redux Toolkit. Any team preference?
- **OQ-7 (test fixtures):** Commit small CC0/CC-BY media clips under `test-assets/` (recommended), or synthesize at test time via ffmpeg, or use Git LFS?
- **OQ-8 (sample-rate / channel-layout normalization):** ffprobe reports `channel_layout` as strings like `"stereo"`, `"5.1"`, `"mono"`. The PRD's `MediaProbeResult.audio.layout` field is optional and undocumented. Define the canonical strings explicitly in the schema, or pass through ffprobe's strings verbatim?
- **OQ-9 (cancellation):** PRD §11.10 lists cancellation as supported via job abstractions. Phase 3's `media.probe` is short (<3s typical) — is per-probe cancellation in scope, or deferred to Phase 4 where long-running extract/remux makes it essential?
- **OQ-10 (logs and telemetry):** PRD §2 says "Ensure all probing occurs locally with zero telemetry." The existing sidecar log file contains paths and codecs. Confirm this satisfies "zero telemetry" (no network egress) rather than "no on-disk metadata" — the latter would force a more constrained logging policy.

---

## 9. Risks & Mitigations

| Risk | Likelihood | Impact | Mitigation |
|---|---|---|---|
| LGPL licence audit drags | Medium | Schedule slip | Pick a well-known LGPL build family up front; commit lock manifest |
| macOS Gatekeeper flags bundled ffprobe | High | First-launch failure | Co-sign ffprobe in the Tauri build pipeline; document fallback |
| Windows long-path edge cases | Medium | Specific files fail | `\\?\` prefix when needed; integration test in CI |
| ffprobe version skew between bundle and parser | Medium | Probe failures on real-world files | Pin upstream version; tolerate unknown JSON fields; capture-and-replay tests |
| Subprocess orphans on crash | Low | Resource leak | Process-group kill; tested on Windows + macOS |
| Bundle size growth (with full ffmpeg) | Medium | Slower downloads, App Store size limits | Strip unused codecs at build time if needed; consider per-platform optional downloads later |
| Tauri permission scope creep | Low | Security review delays | Webview never spawns ffprobe; minimal capability surface |
| Frontend UI scope creep (OQ-1) | Medium | Schedule slip | Resolve OQ-1 before LLD; cap the UI work explicitly |

---

## 10. Deliverables at Phase Exit

Assuming OQ-1 confirms "Phase 3 ships the workspace UI":

- Bundled `ffprobe` per platform with pinned-version manifest and licence notice.
- `media.probe` IPC method end-to-end with canonical `MediaProbeResult`.
- Workspace UI: empty state, drop zone, workspace card, diagnostics drawer, replace-file modal, error UX, accessibility baseline.
- Test fixtures + integration tests for all supported formats and documented failure modes.
- Updated docs: `docs/ipc-contract.md` (new method + error codes), `docs/adr/` (LGPL bundling decision), README/about/licenses (FFmpeg provenance).
- CI: ffprobe fetch + verify; passes on Windows + macOS Intel + Apple Silicon.

This satisfies PRD §16 acceptance criteria.
