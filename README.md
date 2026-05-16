# Studio Sound App

Phase 0 bootstraps the Studio Sound desktop shell, React/Vite frontend, Go sidecar, and validation tooling.

## Prerequisites

- Node.js 22 and npm 10+
- Go 1.22+
- Rust stable with Cargo
- macOS: Xcode Command Line Tools
- Windows: Microsoft C++ Build Tools and WebView2 Runtime
- Optional: `make` for convenience aliases

## Setup

Install dependencies and build the sidecar artifacts:

```bash
npm run setup
```

This runs prerequisite checks, installs npm workspace dependencies, and builds the sidecars into `app/src-tauri/binaries/`.

## Run

Launch the desktop app:

```bash
npm start
```

The first window should show:

- `Studio Sound App`
- `Phase 0 Bootstrap Successful`

## Canonical Commands

```bash
npm run setup
npm start
npm run build
npm run lint
npm run test
npm run sidecar:build
```

Optional Make aliases:

```bash
make setup
make run
make build
make lint
make test
```

## Sidecar Health Smoke Test

Build the sidecars:

```bash
npm run sidecar:build
```

Run the current-platform binary:

```bash
# Windows x64
app/src-tauri/binaries/studio-sidecar-x86_64-pc-windows-msvc.exe health

# macOS Intel
app/src-tauri/binaries/studio-sidecar-x86_64-apple-darwin health

# macOS Apple Silicon
app/src-tauri/binaries/studio-sidecar-aarch64-apple-darwin health
```

Expected stdout:

```text
{"status":"ok","version":"0.0.1"}
```

## Validation

Run the same core checks used by CI:

```bash
npm ci
npm run sidecar:build
npm run test
npm run lint
npm run build -w app
```

## IPC Contract

The Go sidecar communicates with the Tauri shell over Newline-Delimited JSON
(NDJSON) on stdin/stdout. The full wire format specification, available methods
(`system.ping`, `system.echo`, `system.shutdown`), and a step-by-step guide
for adding new methods are documented in [docs/ipc-contract.md](docs/ipc-contract.md).

Design rationale and alternatives considered are captured in the Architecture
Decision Record at [docs/adr/2026-05-14-ipc-contract.md](docs/adr/2026-05-14-ipc-contract.md).

### Logs

Structured JSON logs are written to a rotating log file in a per-OS directory:

| OS      | Path                                       |
| ------- | ------------------------------------------ |
| macOS   | `~/Library/Logs/com.studiosound.app/`      |
| Windows | `%LOCALAPPDATA%\com.studiosound.app\Logs\` |

Press `Cmd/Ctrl+Shift+D` (development builds only) to open the Diagnostics
screen, which has an "Open Logs Folder" button, a live sidecar status indicator,
and Ping / Echo controls for manual round-trip testing.

### Diagnostics screen

The Diagnostics screen is available in development builds only. Activate it by:

- pressing `Cmd+Shift+D` (macOS) or `Ctrl+Shift+D` (Windows/Linux), or
- appending `?diag=1` to the URL in the WebView.

The screen shows the current sidecar state (`Spawning`, `Up`, `Down`,
`Restarting`, or `Failed`), allows sending Ping and Echo requests, and provides
a button to open the log directory.

## Troubleshooting

- `npm run setup` fails on prerequisites: install the missing tool printed by `scripts/check-prereqs.mjs`.
- Tauri build fails on macOS: run `xcode-select --install` and retry.
- Tauri build fails on Windows: install Microsoft C++ Build Tools and ensure WebView2 Runtime is available.
- `npm run lint` cannot find `golangci-lint`: install it locally or use CI, where it is installed by the workflow.
- Sidecar smoke test fails with permission denied on macOS: run `chmod +x app/src-tauri/binaries/studio-sidecar-aarch64-apple-darwin` or the Intel equivalent, then retry.
