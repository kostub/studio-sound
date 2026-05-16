# Studio Sound App — Phase 0 Research & Requirements Document

Version: 0.1  
Status: Research / Bootstrap Planning  
Target Platforms: macOS (Apple Silicon + Intel), Windows x64  
Project Type: Desktop audio application platform bootstrap  
Architecture: Tauri 2 + React/TypeScript frontend + Go sidecar

---

# 1. Purpose of This Document

This document defines the technical research findings, architectural decisions, tooling requirements, repository standards, CI strategy, and bootstrap expectations for Phase 0 of the Studio Sound App.

Phase 0 is intentionally limited to:

- Establishing the monorepo
- Bootstrapping the desktop shell
- Establishing build pipelines
- Validating cross-platform builds
- Establishing developer experience standards
- Wiring frontend + sidecar integration foundations

No product/audio features are implemented in this phase.

This document is intended to become the basis for:

- Low-Level Design (LLD)
- Implementation planning
- CI/CD implementation
- Developer onboarding
- Coding standards

---

# 2. Phase 0 Goals

## Primary Goal

Create a stable, reproducible, cross-platform development foundation for all future phases.

## Success Criteria

A fresh clone of the repository should allow a developer to:

1. Install dependencies
2. Run one bootstrap/setup command
3. Launch a Tauri desktop window
4. Build frontend + sidecar
5. Run tests/linting
6. Successfully pass CI on:
   - Windows x64
   - macOS Intel
   - macOS Apple Silicon

---

# 3. High-Level Architecture

```text
+------------------------------------------------------+
|                 Tauri Desktop Shell                  |
|------------------------------------------------------|
| React + TypeScript + Vite Frontend                  |
|------------------------------------------------------|
| Tauri Rust Core                                      |
|------------------------------------------------------|
| Go Sidecar Process                                   |
|------------------------------------------------------|
| Future Audio Engine / Native DSP                     |
+------------------------------------------------------+
```

## Architectural Decisions

| Area                 | Decision                  | Reason                                   |
| -------------------- | ------------------------- | ---------------------------------------- |
| Desktop framework    | Tauri 2                   | Lightweight, secure, modern Rust backend |
| Frontend             | React + TypeScript + Vite | Fast DX, ecosystem maturity              |
| Native backend       | Go sidecar                | Easier concurrency/process mgmt          |
| Repository structure | Monorepo                  | Unified CI/tooling/versioning            |
| CI                   | GitHub Actions            | Cross-platform matrix support            |
| Package manager      | npm initially             | Simplicity for bootstrap                 |
| Formatting           | Prettier + gofmt          | Standardized code style                  |
| Linting              | ESLint + golangci-lint    | Code quality enforcement                 |
| Testing              | Vitest + go test          | Lightweight bootstrap tests              |

---

# 4. Technology Research

# 4.1 Tauri 2

## Why Tauri 2

Tauri 2 provides:

- Smaller binary sizes than Electron
- Native OS integration
- Strong security model
- Rust-based backend
- Sidecar process support
- Cross-platform desktop packaging
- Good frontend framework support

## Important Tauri 2 Concepts

| Concept                | Relevance                        |
| ---------------------- | -------------------------------- |
| `externalBin`          | Required for Go sidecar bundling |
| Sidecar processes      | Launch/manage Go binary          |
| Commands               | Rust bridge to frontend          |
| Capability system      | Security sandboxing              |
| Dev server integration | Vite integration                 |

## Required Tooling

### Windows

- Visual Studio Build Tools
- WebView2 runtime
- Rust toolchain
- Node.js LTS
- Go 1.22+
- WiX Toolset (for MSI packaging later)

### macOS

- Xcode Command Line Tools
- Rust toolchain
- Node.js LTS
- Go 1.22+
- Apple notarization tools (future phase)

---

# 4.2 React + TypeScript + Vite

## Why This Stack

| Tool       | Reason                     |
| ---------- | -------------------------- |
| React      | Ecosystem maturity         |
| TypeScript | Strict typing              |
| Vite       | Extremely fast dev startup |
| Vitest     | Native Vite test runner    |

## Required Standards

### TypeScript

Strict mode must be enabled.

Required compiler options:

```json
{
  "strict": true,
  "noImplicitAny": true,
  "noUncheckedIndexedAccess": true,
  "exactOptionalPropertyTypes": true
}
```

---

# 4.3 Go Sidecar

## Why Go

Go is selected for the sidecar because:

- Excellent concurrency primitives
- Simple cross-compilation
- Small operational overhead
- Fast compilation
- Good process orchestration
- Strong future suitability for:
  - Audio device management
  - WebSocket services
  - IPC servers
  - Local streaming
  - DSP orchestration

## Initial Requirements

The sidecar must:

- Compile independently
- Expose a trivial `health` command
- Produce JSON output
- Be testable independently
- Support cross-compilation

## Example Output

```json
{
  "status": "ok",
  "version": "0.0.1"
}
```

---

# 5. Repository Structure Research

## Recommended Layout

```text
/
├── app/
│   ├── src/
│   ├── src-tauri/
│   ├── package.json
│   └── vite.config.ts
│
├── sidecar/
│   ├── cmd/
│   │   └── sidecar/
│   ├── internal/
│   ├── go.mod
│   └── Makefile
│
├── scripts/
├── docs/
├── test-assets/
├── .github/
│   └── workflows/
├── README.md
├── .gitignore
└── .editorconfig
```

---

# 6. Monorepo Strategy

## Requirements

The monorepo must:

- Allow independent builds
- Allow shared scripts
- Allow future package sharing
- Keep frontend/backend isolated
- Support CI matrix execution

## Research Findings

A full Nx/Turborepo setup is NOT necessary in Phase 0.

Recommendation:

- Use simple npm workspaces initially
- Re-evaluate at Phase 2 or 3

---

# 7. Sidecar Integration Research

# 7.1 Tauri `externalBin`

Tauri supports bundling external binaries via:

```json
{
  "bundle": {
    "externalBin": ["../sidecar/bin/sidecar"]
  }
}
```

Expected generated binaries:

| Platform    | Binary                |
| ----------- | --------------------- |
| Windows x64 | `sidecar-win-x64.exe` |
| macOS ARM64 | `sidecar-mac-arm64`   |
| macOS x64   | `sidecar-mac-x64`     |

---

# 8. Build System Requirements

# 8.1 Frontend Build

## Requirements

```bash
npm install
npm run build
```

Must produce:

- Optimized Vite assets
- Successful Tauri build
- No lint errors
- No TypeScript errors

---

# 8.2 Sidecar Build

## Required Targets

| Target              | GOOS    | GOARCH |
| ------------------- | ------- | ------ |
| Windows x64         | windows | amd64  |
| macOS Intel         | darwin  | amd64  |
| macOS Apple Silicon | darwin  | arm64  |

## Expected Outputs

```text
sidecar-win-x64.exe
sidecar-mac-x64
sidecar-mac-arm64
```

---

# 9. Tooling Requirements

# 9.1 Node Tooling

## Required Packages

| Package         | Purpose            |
| --------------- | ------------------ |
| eslint          | JS/TS linting      |
| prettier        | Formatting         |
| typescript      | TS compiler        |
| vitest          | Unit testing       |
| @tauri-apps/cli | Tauri tooling      |
| husky           | Git hooks          |
| lint-staged     | Pre-commit linting |

---

# 9.2 Go Tooling

## Required Tools

| Tool          | Purpose          |
| ------------- | ---------------- |
| gofmt         | Formatting       |
| golangci-lint | Lint aggregation |
| go test       | Testing          |

---

# 10. Testing Requirements

# 10.1 Frontend Tests

Framework:

- Vitest

Minimum Requirement:

```ts
expect(true).toBe(true);
```

---

# 10.2 Go Tests

Minimum Requirement:

```go
func TestHealth(t *testing.T)
```

---

# 11. CI/CD Research

# 11.1 GitHub Actions

## Required Matrix

| OS             | Arch                 |
| -------------- | -------------------- |
| windows-latest | x64                  |
| macos-latest   | universal validation |

## Required CI Jobs

- Frontend lint
- Go lint
- Tests
- Build validation

---

# 12. Local Developer Experience

## One-Command Setup

```bash
npm run setup
```

or

```bash
make bootstrap
```

Expected outcome:

- dependencies installed
- sidecar built
- frontend launched

---

# 13. Placeholder UI Requirements

Initial UI:

```text
Studio Sound App

Phase 0 Bootstrap Successful
```

---

# 14. Operational Risks & Research Findings

## Tauri + Go Sidecar Packaging Complexity

Risk:
Cross-platform sidecar naming mismatches.

Mitigation:

- Centralized naming convention
- Explicit CI verification
- Automated artifact validation

---

# 15. Non-Goals

The following are NOT included in Phase 0:

- Audio processing
- Device enumeration
- IPC implementation
- Authentication
- Auto-update
- Telemetry
- State management
- Database integration
- Plugin system
- Native DSP
- Audio streaming
- Recording
- Packaging/signing/notarization
- Release automation

---

# 16. Recommended Initial Versions

| Tool       | Version Recommendation |
| ---------- | ---------------------- |
| Node.js    | 22 LTS                 |
| npm        | bundled                |
| Go         | 1.22+                  |
| Rust       | stable                 |
| Tauri      | 2.x                    |
| React      | 18+                    |
| TypeScript | 5.x                    |

---

# 17. Deliverables Expected From Phase 0

## Repository

- Initialized monorepo
- GitHub repository configured

## Frontend

- Tauri 2 app
- React + TS + Vite
- Placeholder screen

## Sidecar

- Go project
- Health command
- Cross-platform builds

## Tooling

- ESLint
- Prettier
- golangci-lint
- Husky hooks

## Testing

- Vitest
- go test

## CI

- GitHub Actions matrix
- Green builds

## Documentation

- README
- Bootstrap instructions
- Troubleshooting basics

---

# 18. Acceptance Criteria

Phase 0 is considered complete when:

- CI passes on Windows + macOS
- `npm run tauri dev` launches successfully
- Frontend renders placeholder screen
- Sidecar binaries build correctly
- Sidecar health command outputs valid JSON
- Tests pass
- Lint passes
- Fresh developer setup succeeds in under 15 minutes
- README accurately reflects setup steps

---

# 19. Recommended Next Step

Create the Low-Level Design (LLD) covering:

- package.json structure
- Tauri configuration
- CI YAML files
- sidecar build scripts
- TypeScript configs
- developer workflow
- binary naming conventions
