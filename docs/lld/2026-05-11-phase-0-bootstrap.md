# Studio Sound App Phase 0 Bootstrap — Low-Level Design

## 1. Summary / Goals

Phase 0 creates the repository, desktop shell, Go sidecar skeleton, build tooling, CI, and developer bootstrap workflow described in [docs/reqs/phase_0.md](../reqs/phase_0.md).
The goal is a reproducible cross-platform foundation where a fresh clone can install dependencies, run one setup command, launch the app with one run/start command, build the frontend and sidecar, and pass lint/test/build checks on Windows x64 plus macOS Intel and Apple Silicon.
This LLD also aligns with the Phase 0 scope in [docs/implementation_plan.md](../implementation_plan.md), which defines this phase as the developer-experience floor for later product phases.
Non-goals for this phase remain product/audio features, IPC implementation, device handling, state persistence, release signing, notarization, and release automation.

## 2. Current Code-base Findings

- The current repository contains Phase 0 requirements and an implementation plan, but no application source tree yet. [verified: docs/reqs/phase_0.md:1] [verified: docs/implementation_plan.md:1] [inferred: `rg --files` returned only `docs/implementation_plan.md` and `docs/reqs/phase_0.md` before this LLD was added]
- Phase 0 is explicitly limited to monorepo setup, desktop shell bootstrap, build pipelines, cross-platform validation, developer experience standards, and frontend/sidecar integration foundations. [verified: docs/reqs/phase_0.md:14]
- Product and audio features are excluded from Phase 0. [verified: docs/reqs/phase_0.md:23]
- The required technology choices are Tauri 2, React, TypeScript, Vite, a Go sidecar, GitHub Actions, npm, Prettier, gofmt, ESLint, golangci-lint, Vitest, and `go test`. [verified: docs/reqs/phase_0.md:72]
- The recommended repository layout introduces `app/`, `app/src-tauri/`, `sidecar/`, `scripts/`, `docs/`, `test-assets/`, `.github/workflows/`, root `README.md`, root `.gitignore`, and root `.editorconfig`. [verified: docs/reqs/phase_0.md:204]
- The monorepo should allow independent builds, shared scripts, future package sharing, frontend/backend isolation, and CI matrix execution. [verified: docs/reqs/phase_0.md:237]
- The requirements recommend simple npm workspaces for Phase 0 and explicitly defer Nx/Turborepo reconsideration to a later phase. [verified: docs/reqs/phase_0.md:247]
- The Go sidecar must compile independently, expose a trivial `health` command, produce JSON output, be testable independently, and support cross-compilation. [verified: docs/reqs/phase_0.md:183]
- The expected health JSON contains `status: "ok"` and `version: "0.0.1"`. [verified: docs/reqs/phase_0.md:193]
- The expected sidecar output names are `sidecar-win-x64.exe`, `sidecar-mac-x64`, and `sidecar-mac-arm64`. [verified: docs/reqs/phase_0.md:312]
- TypeScript strict mode must include `strict`, `noImplicitAny`, `noUncheckedIndexedAccess`, and `exactOptionalPropertyTypes`. [verified: docs/reqs/phase_0.md:146]
- GitHub Actions must run frontend lint, Go lint, tests, and build validation across Windows and macOS. [verified: docs/reqs/phase_0.md:377]
- The expected one-command developer setup is `npm run setup` or `make bootstrap`, ending with dependencies installed, the sidecar built, and the frontend launched. [verified: docs/reqs/phase_0.md:397]
- The initial UI must render `Studio Sound App` and `Phase 0 Bootstrap Successful`. [verified: docs/reqs/phase_0.md:418]
- Phase 0 acceptance requires CI passing on Windows and macOS, `npm run tauri dev` launching, placeholder UI rendering, sidecar binaries building, sidecar health JSON output, tests passing, lint passing, fresh setup under 15 minutes, and accurate README setup steps. [verified: docs/reqs/phase_0.md:523]
- The phased implementation plan states Phase 0 has no dependencies and includes the monorepo layout, Tauri 2 app, Go sidecar `health` command, sidecar bundling via `externalBin`, dev tooling, unit test runners, CI matrix, and README setup documentation. [verified: docs/implementation_plan.md:38]

## 3. Proposed Design

Phase 0 will create a minimal but complete monorepo. The implementation should use the requirements' simple npm-workspace approach and avoid adding Nx/Turborepo until a later phase needs it.

The main design decision is sidecar binary naming. Tauri 2 sidecar bundling expects binaries matching the configured base name plus a Rust target-triple suffix, so Phase 0 should produce only the Tauri-compatible sidecar binaries:

- `app/src-tauri/binaries/studio-sidecar-x86_64-pc-windows-msvc.exe`
- `app/src-tauri/binaries/studio-sidecar-x86_64-apple-darwin`
- `app/src-tauri/binaries/studio-sidecar-aarch64-apple-darwin`

The requirements' earlier human-readable names are treated as research examples, not additional files to generate. This avoids duplicate binary outputs and makes Tauri bundling the single source of truth.

The developer command surface should be intentionally small:

- One-time setup: `npm run setup` or `make setup`.
- Run the app again: `npm start` or `make run`.

A root `Makefile` is useful as an ergonomic wrapper for developers who have `make`, but npm scripts remain the canonical cross-platform implementation because Windows developers may not have `make` installed.

### 3.1 Data model changes

N/A — Phase 0 has no database, persistent state, user settings store, or domain data model. Database integration and state management are non-goals for this phase.

The only structured data introduced is configuration and command output:

- `package.json` files for npm workspace configuration and scripts.
- `app/tsconfig.json` for strict TypeScript options.
- `app/src-tauri/tauri.conf.json` for Tauri build/dev/bundle configuration.
- `app/src-tauri/capabilities/default.json` for the default desktop capability.
- `.github/workflows/ci.yml` for CI job definitions.
- Go health response JSON:

```json
{
  "status": "ok",
  "version": "0.0.1"
}
```

### 3.2 API contract

No product IPC API is introduced in Phase 0. The frontend must not call into the sidecar through an application IPC contract yet; that belongs to Phase 1.

Phase 0 exposes these bootstrap contracts:

**Root npm scripts**

Defined in `package.json` [new].

```json
{
  "scripts": {
    "setup": "node scripts/check-prereqs.mjs && npm install && npm run sidecar:build",
    "start": "npm run sidecar:build && npm run dev -w app",
    "build": "npm run sidecar:build && npm run build -w app",
    "dev": "npm start",
    "lint": "npm run lint -w app && npm run sidecar:lint",
    "format": "prettier --write . && npm run sidecar:fmt",
    "test": "npm run test -w app && npm run sidecar:test",
    "sidecar:build": "node scripts/build-sidecar.mjs",
    "sidecar:test": "cd sidecar && go test ./...",
    "sidecar:fmt": "cd sidecar && go fmt ./...",
    "sidecar:lint": "cd sidecar && golangci-lint run ./..."
  }
}
```

Commands that run Go module-aware tooling must execute with working directory `sidecar/`. The root scripts may use `cd sidecar && ...` for simple Go commands, while `scripts/build-sidecar.mjs` should set `cwd` explicitly when spawning `go build`.

**App npm scripts**

Defined in `app/package.json` [new].

```json
{
  "scripts": {
    "tauri": "tauri",
    "dev": "tauri dev",
    "build": "tauri build",
    "web:dev": "vite --host 127.0.0.1",
    "web:build": "tsc --noEmit && vite build",
    "lint": "eslint . --max-warnings=0",
    "test": "vitest run",
    "test:watch": "vitest",
    "format": "prettier --write ."
  }
}
```

**Go sidecar CLI**

Binary: `studio-sidecar` from `sidecar/cmd/sidecar` [new].

```bash
studio-sidecar health
```

Success response:

```json
{ "status": "ok", "version": "0.0.1" }
```

Failure responses:

- Unknown command: exit code `2`; stderr contains `unknown command: <name>`.
- JSON encoding failure: exit code `1`; stderr contains `failed to encode health response`.

The `health` command must write only valid JSON to stdout and no informational logs to stdout.

**Sidecar build artifacts**

```text
app/src-tauri/binaries/studio-sidecar-x86_64-pc-windows-msvc.exe
app/src-tauri/binaries/studio-sidecar-x86_64-apple-darwin
app/src-tauri/binaries/studio-sidecar-aarch64-apple-darwin
```

No duplicate `sidecar/bin/sidecar-win-x64.exe`, `sidecar/bin/sidecar-mac-x64`, or `sidecar/bin/sidecar-mac-arm64` artifacts are generated.

**CI contract**

Defined in `.github/workflows/ci.yml` [new].

- Trigger on `pull_request` and `push` to the default branch.
- Run on `windows-latest`, `macos-26-intel`, and `macos-26`.
- Install Node 22, Go 1.22+, Rust stable, and platform Tauri prerequisites available on hosted runners.
- Run `npm ci`, `npm run lint`, `npm run test`, `npm run sidecar:build`, and `npm run build -w app`.
- Verify required sidecar artifact paths exist after `npm run sidecar:build`.

### 3.3 Class / module changes

### package.json [new]

- Root npm workspace manifest.
  - Defines workspaces: `["app"]`.
  - Defines repository-wide scripts listed in section 3.2.
  - Holds dev dependencies that apply at the repository level: `prettier`, `husky`, and `lint-staged`.
  - Uses `"private": true` to prevent accidental publication.

### package-lock.json [new]

- npm lockfile generated from the root workspace install.
  - Must include root tooling and the `app` workspace dependency graph.
  - Must be committed so CI uses `npm ci`.

### Makefile [new]

- Root developer convenience wrapper.
  - new target: `setup`
    - Calls: `npm run setup`.
  - new target: `run`
    - Calls: `npm start`.
  - new target: `build`
    - Calls: `npm run build`.
  - new target: `test`
    - Calls: `npm run test`.
  - new target: `lint`
    - Calls: `npm run lint`.
  - The Makefile must not contain independent build logic; npm scripts remain the source of truth.

### app/package.json [new]

- Frontend workspace manifest.
  - Defines app scripts listed in section 3.2.
  - Runtime dependencies: `@tauri-apps/api`, `react`, `react-dom`.
  - Dev dependencies: `@tauri-apps/cli`, `@vitejs/plugin-react`, `typescript`, `vite`, `vitest`, `jsdom`, `@testing-library/react`, `@testing-library/jest-dom`, `eslint`, `typescript-eslint`, `eslint-plugin-react-hooks`, `eslint-plugin-react-refresh`, `prettier`.

### app/index.html [new]

- Vite HTML entry point.
  - Loads `/src/main.tsx`.
  - Sets document title to `Studio Sound App`.

### app/src/main.tsx [new]

- React entry point.
  - new function: `renderApp() -> void`
    - Called from: module initialization in `app/src/main.tsx` [new]
    - Calls: `react-dom/client::createRoot` [new dependency]
    - Renders: `app/src/App.tsx::App` [new]

### app/src/App.tsx [new]

- Component: `App`
  - new function: `App() -> JSX.Element`
    - Called from: `app/src/main.tsx::renderApp` [new]
    - Returns the Phase 0 placeholder UI with `Studio Sound App` and `Phase 0 Bootstrap Successful`.

### app/src/App.test.tsx [new]

- Test module for the placeholder UI.
  - new test: `rendersPhase0BootstrapMessage`
    - Calls: `@testing-library/react::render` [new dependency]
    - Asserts both required placeholder strings are present.

### app/src/styles.css [new]

- Frontend stylesheet.
  - Defines a minimal centered desktop placeholder layout.
  - Avoids product UI elements that imply Phase 1+ functionality.

### app/vite.config.ts [new]

- Vite configuration.
  - Uses `@vitejs/plugin-react`.
  - Configures Vitest with `jsdom` for React component tests.
  - Leaves build output at Vite's default `dist/` unless Tauri config requires an explicit path.

### app/tsconfig.json [new]

- TypeScript app configuration.
  - Enables `strict`, `noImplicitAny`, `noUncheckedIndexedAccess`, and `exactOptionalPropertyTypes`.
  - Uses modern module resolution compatible with Vite.
  - Includes `src/**/*.ts`, `src/**/*.tsx`, and Vite config files.

### app/tsconfig.node.json [new]

- TypeScript configuration for Vite and tooling config files.
  - Keeps Node/tooling types separate from browser application types.

### app/eslint.config.js [new]

- ESLint flat config.
  - Uses TypeScript-aware linting for `app/src`.
  - Enables React Hooks and React Refresh rules.
  - Fails on warnings in CI through the `app/package.json` lint script.

### app/src-tauri/Cargo.toml [new]

- Tauri Rust crate manifest.
  - Package name: `studio-sound-app`.
  - Dependencies: `tauri`.
  - Build dependency: `tauri-build`.

### app/src-tauri/build.rs [new]

- Rust build script.
  - new function: `main() -> ()`
    - Called by: Cargo build lifecycle [new]
    - Calls: `tauri_build::build()` [new dependency]

### app/src-tauri/src/main.rs [new]

- Tauri application entry point.
  - new function: `main() -> ()`
    - Called from: native process startup [new]
    - Calls: `app/src-tauri/src/lib.rs::run` [new]

### app/src-tauri/src/lib.rs [new]

- Tauri application setup module.
  - new function: `run() -> ()`
    - Called from: `app/src-tauri/src/main.rs::main` [new]
    - Calls: `tauri::Builder::default` and runs the generated Tauri context.
  - Does not define product commands in Phase 0.

### app/src-tauri/tauri.conf.json [new]

- Tauri desktop configuration.
  - `productName`: `Studio Sound App`.
  - `identifier`: `com.studiosound.app`.
  - `build.beforeDevCommand`: `npm run web:dev`.
  - `build.beforeBuildCommand`: `npm run web:build`.
  - `build.devUrl`: Vite dev server URL.
  - `build.frontendDist`: `../dist`.
  - `bundle.externalBin`: `["binaries/studio-sidecar"]`.
  - Window title: `Studio Sound App`.

### app/src-tauri/capabilities/default.json [new]

- Tauri capability configuration.
  - Allows the main window to use default core permissions.
  - Does not grant shell execution or sidecar spawn permissions in Phase 0 because the app does not launch the sidecar yet.

### app/src-tauri/binaries/.gitkeep [new]

- Keeps the Tauri sidecar binary staging directory present in fresh clones.
  - Actual generated binaries remain ignored by `.gitignore`.

### sidecar/go.mod [new]

- Go module definition.
  - Module path: `github.com/studio-sound/sidecar`.
  - Go version: `1.22` minimum.

### sidecar/cmd/sidecar/main.go [new]

- Package: `main`
  - new function: `main() -> ()`
    - Called from: native process startup [new]
    - Calls: `sidecar/internal/cli::Run(os.Args[1:], os.Stdout, os.Stderr, version)` [new]
  - Defines `version = "0.0.1"` as a package variable overrideable by `-ldflags "-X main.version=<version>"`.

### sidecar/internal/cli/cli.go [new]

- Package: `cli`
  - new function: `Run(args []string, stdout io.Writer, stderr io.Writer, version string) int`
    - Called from: `sidecar/cmd/sidecar/main.go::main` [new]
    - Calls: `sidecar/internal/health::Check(version)` [new]
    - Dispatches `health`.
    - Returns POSIX-style exit codes without calling `os.Exit`, so tests can exercise behavior.

### sidecar/internal/health/health.go [new]

- Package: `health`
  - new type: `Response`
    - Fields: `Status string`, `Version string`.
    - JSON names: `status`, `version`.
  - new function: `Check(version string) Response`
    - Called from: `sidecar/internal/cli/cli.go::Run` [new]
    - Returns `Response{Status: "ok", Version: version}`.

### sidecar/internal/health/health_test.go [new]

- Package: `health`
  - new test: `TestHealth(t *testing.T)`
    - Calls: `sidecar/internal/health::Check` [new]
    - Asserts status `ok` and version passthrough.

### sidecar/internal/cli/cli_test.go [new]

- Package: `cli`
  - new test: `TestRunHealthWritesJSON(t *testing.T)`
    - Calls: `sidecar/internal/cli::Run` [new]
    - Asserts exit code `0` and valid JSON stdout.
  - new test: `TestRunUnknownCommand(t *testing.T)`
    - Calls: `sidecar/internal/cli::Run` [new]
    - Asserts exit code `2`, empty stdout, and useful stderr.

### scripts/build-sidecar.mjs [new]

- Cross-platform sidecar build script.
  - new function: `main() -> Promise<void>`
    - Called from: Node process startup [new]
    - Calls: `buildTarget` for `windows/amd64`, `darwin/amd64`, and `darwin/arm64`.
    - Calls: `verifyArtifacts`.
  - new function: `buildTarget(target: BuildTarget) -> Promise<void>`
    - Calls: `go build` with `GOOS`, `GOARCH`, and output path.
    - Writes `app/src-tauri/binaries/studio-sidecar-<target-triple>`.
  - new function: `verifyArtifacts(targets: BuildTarget[]) -> Promise<void>`
    - Ensures all required Tauri bundle artifacts exist.
  - new type: `BuildTarget`
    - Fields: `goos`, `goarch`, `tauriTriple`, `executableExtension`.

### scripts/check-prereqs.mjs [new]

- Developer setup prerequisite checker.
  - new function: `main() -> Promise<void>`
    - Called from: `npm run setup` before dependency installation and build commands.
    - Calls: `node --version`, `npm --version`, `go version`, `rustc --version`, and `cargo --version`.
    - Prints missing prerequisites with direct remediation hints.
  - This script should warn for missing `golangci-lint` locally but allow CI to install it explicitly.

### .github/workflows/ci.yml [new]

- GitHub Actions workflow.
  - Job: `validate`
    - Matrix: `windows-latest`, `macos-26-intel`, `macos-26`.
    - Steps: checkout, setup Node, setup Go, setup Rust, install Tauri prerequisites where needed, `npm ci`, lint, test, sidecar build, app build, artifact verification.
  - Uses `shell: bash` for script consistency except Windows-specific dependency setup steps if required.

### .gitignore [new]

- Ignore generated artifacts.
  - Node: `node_modules/`, `app/dist/`.
  - Rust/Tauri: `app/src-tauri/target/`.
  - Sidecar: `app/src-tauri/binaries/studio-sidecar-*`.
  - Keep `app/src-tauri/binaries/.gitkeep`.

### .editorconfig [new]

- Repository editor defaults.
  - UTF-8, LF line endings, final newline, two-space indentation for JS/TS/JSON/YAML, tabs or gofmt-managed defaults for Go.

### .prettierrc.json [new]

- Prettier configuration.
  - Applies to Markdown, JSON, YAML, JS, TS, TSX, and CSS.

### .prettierignore [new]

- Excludes generated directories and binary outputs.
  - `node_modules`, `app/dist`, `app/src-tauri/target`, `app/src-tauri/binaries/studio-sidecar-*`.

### .golangci.yml [new]

- Go lint configuration.
  - Enables a conservative Phase 0 set: `govet`, `staticcheck`, `ineffassign`, `errcheck`, `unused`.
  - Keeps configuration small so bootstrap does not become lint-policy work.

### README.md [new]

- Developer onboarding document.
  - Includes prerequisites for macOS and Windows.
  - Documents the primary two-command local flow: `npm run setup` once, then `npm start` whenever the developer wants to launch the app.
  - Documents optional Make aliases: `make setup` and `make run`.
  - Documents validation commands separately: `npm run build`, `npm run lint`, `npm run test`, `npm run sidecar:build`, and manual sidecar health verification.
  - Includes troubleshooting basics for missing Rust, Go, WebView2, Xcode Command Line Tools, and sidecar artifact verification failures.

### test-assets/.gitkeep [new]

- Keeps the future test fixture directory present without adding media files in Phase 0.

## 3.4 Logic flow

**Fresh clone setup**

1. Developer installs documented prerequisites from `README.md`.
2. Developer runs `npm run setup` or `make setup`.
3. `scripts/check-prereqs.mjs::main` verifies Node, npm, Go, Rust, and Cargo availability.
4. Root `setup` runs `npm install`.
5. Root `setup` runs `npm run sidecar:build`.
6. Root `package.json` script `sidecar:build` calls `scripts/build-sidecar.mjs::main`.
7. `buildTarget` compiles all three sidecar targets and stages Tauri bundle artifacts.
8. `verifyArtifacts` fails setup if any required sidecar binary is missing.

**Run app after setup**

1. Developer runs `npm start` or `make run`.
2. Root `start` refreshes sidecar binaries with `npm run sidecar:build`.
3. Root `start` launches `npm run dev -w app`.
4. `app/package.json` script `dev` runs `tauri dev`.
5. Tauri runs `npm run web:dev` through `beforeDevCommand`, starts against the Vite dev URL, and opens a native window showing `App`.

**Manual sidecar health check**

1. Developer runs the platform-local Tauri sidecar binary with `health`.
2. `sidecar/cmd/sidecar/main.go::main` passes args and writers into `cli.Run`.
3. `sidecar/internal/cli/cli.go::Run` matches the `health` command.
4. `Run` calls `health.Check(version)`.
5. `Run` JSON-encodes `health.Response` to stdout.
6. Process exits with code `0`.

**Repository build**

1. Developer or CI runs `npm run build`.
2. Root `build` runs `npm run sidecar:build`.
3. Root `build` runs `npm run build -w app`.
4. App build runs `tauri build`.
5. Tauri runs `npm run web:build` through `beforeBuildCommand`.
6. Tauri reads `app/src-tauri/tauri.conf.json`, uses `externalBin: ["binaries/studio-sidecar"]`, and bundles the matching staged target-triple sidecar for the current platform.

**CI validation**

1. GitHub Actions starts the `validate` matrix on `windows-latest`, `macos-26-intel`, and `macos-26`.
2. Each matrix runner installs Node 22, Go 1.22+, and Rust stable.
3. Each runner executes `npm ci`.
4. Each runner executes `npm run lint`.
5. Each runner executes `npm run test`.
6. Each runner executes `npm run sidecar:build`.
7. Each runner verifies all three Tauri bundle sidecar artifacts.
8. Each runner executes `npm run build -w app`.

**Placeholder UI render**

1. `app/src/main.tsx::renderApp` creates the React root.
2. `renderApp` renders `app/src/App.tsx::App`.
3. `App` returns the static Phase 0 placeholder text.
4. Vitest verifies the rendered text in `app/src/App.test.tsx`.

## 4. Open Questions

- Assumption: `npm run setup` should install dependencies and build sidecars, but should not launch the dev app; `npm start` is the one-command npm app launcher after setup, and `make run` is the Makefile alias.
- Assumption: The root `Makefile` is a convenience wrapper only; npm scripts remain canonical so Windows developers are not blocked by missing `make`.
- Assumption: The Tauri bundle binary names should follow target-triple suffixes in `app/src-tauri/binaries/`; the requirements' earlier human-readable names are not generated as extra artifacts.
- Assumption: Node 22 remains the Phase 0 Node baseline even if newer LTS releases are available by implementation time.

## 5. Risks / Trade-offs

- **Binary naming mismatch with requirements examples:** The requirements list readable sidecar names, but Tauri bundling needs target-triple names. The design chooses only Tauri-compatible outputs, which removes duplicate artifacts at the cost of less human-friendly filenames.
- **Make availability on Windows:** A Makefile improves ergonomics for developers who have `make`, but some Windows environments will not. npm scripts remain canonical and README presents Make as optional.
- **Setup command interactivity:** `setup` exits after installing and staging sidecars. This requires a second command to launch the app, but it keeps setup safe for repeated use and avoids a long-running desktop process in bootstrap scripts.
- **Cross-compilation confidence:** `go build` can produce all sidecar binaries from one host, but Tauri desktop build validation still depends on the current runner platform. CI should build app packages per runner and verify Tauri sidecar artifacts separately.
- **macOS runner freshness:** The CI matrix uses explicit `macos-26-intel` and `macos-26` labels, avoiding `macos-latest` drift. If GitHub changes availability for either label, CI will fail loudly and the workflow should be updated deliberately.
- **Tooling weight:** Husky, lint-staged, ESLint, Prettier, golangci-lint, Rust, Go, and Tauri add setup surface area. Phase 0 keeps configuration conservative and documents prerequisites clearly rather than optimizing for minimal installs.
- **No real IPC yet:** The sidecar is bundled and health-checkable, but the UI does not call it. This keeps Phase 0 aligned with the non-goal of IPC implementation and leaves the typed IPC contract to Phase 1.

## 6. Edge cases / Error handling

- Missing Go toolchain: `scripts/check-prereqs.mjs` reports `go` as missing and exits non-zero before sidecar build starts.
- Missing Rust/Cargo toolchain: prerequisite check reports missing `rustc` or `cargo`; Tauri build is not attempted.
- Missing `golangci-lint`: local lint reports installation guidance; CI installs the pinned tool before `npm run lint`.
- Unknown sidecar command: `cli.Run` writes an error to stderr, writes nothing to stdout, and returns exit code `2`.
- JSON encoding failure in health command: `cli.Run` writes a concise stderr message and returns exit code `1`.
- Sidecar output directory missing: `scripts/build-sidecar.mjs::buildTarget` creates `app/src-tauri/binaries/` before writing outputs.
- Sidecar artifact missing after build: `verifyArtifacts` lists the missing path and exits non-zero.
- Windows executable extension mismatch: `BuildTarget.executableExtension` appends `.exe` only for the Windows Tauri bundle artifact.
- Path separators on Windows: build scripts use Node `path.join` and avoid hard-coded `/` when touching the filesystem.
- Vite dev server port in use: Tauri/Vite should surface the startup failure; README troubleshooting should tell developers to stop the conflicting process or override the port consistently in Vite and Tauri config.
- `npm start` launches a long-running dev server: README documents using `Ctrl+C` to stop it and using `npm run build` for non-interactive validation.
- CI runner lacks GUI display: CI uses `tauri build`, lint, and tests; it does not require manual UI interaction.

## 7. Testing Strategy

**Unit tests**

- `sidecar/internal/health/health_test.go::TestHealth`: verifies `health.Check("0.0.1")` returns status `ok` and version `0.0.1`.
- `sidecar/internal/cli/cli_test.go::TestRunHealthWritesJSON`: verifies `cli.Run([]string{"health"}, ...)` returns `0` and writes parseable JSON matching the health contract.
- `sidecar/internal/cli/cli_test.go::TestRunUnknownCommand`: verifies invalid commands return `2`, keep stdout empty, and write a useful stderr error.
- `app/src/App.test.tsx::rendersPhase0BootstrapMessage`: verifies the placeholder UI renders `Studio Sound App` and `Phase 0 Bootstrap Successful`.

**Integration / build validation**

- `npm run sidecar:build`: verifies all three Tauri target-triple bundle artifacts exist.
- `npm run build -w app`: verifies strict TypeScript, Vite production build, Rust/Tauri compile, and sidecar bundle configuration.
- `npm run lint`: verifies ESLint and golangci-lint pass with no warnings treated as acceptable in the frontend.
- `npm run test`: verifies Vitest and `go test ./...` both pass.
- Manual local smoke test: root `npm start`, `make run`, or `npm run tauri dev` from `app/` opens a desktop window and renders the Phase 0 placeholder text.
- Manual sidecar smoke test: run the current-platform sidecar binary with `health` and validate stdout is exactly JSON with `status` and `version`.

**CI coverage**

- Windows x64 runner executes lint, tests, sidecar cross-build, artifact verification, and app build.
- macOS Intel runner executes the same validation.
- macOS Apple Silicon validation executes the same validation where a hosted runner is available; otherwise CI must at least produce and verify `darwin/arm64` sidecar output and document the hosted-runner limitation in README troubleshooting.

**Acceptance mapping**

- CI passes on Windows and macOS: `.github/workflows/ci.yml::validate`.
- `npm run tauri dev` launches from `app/`, and root `npm start` launches the same flow: manual smoke test documented in README.
- Placeholder screen renders: `App.test.tsx` plus manual smoke test.
- Sidecar binaries build correctly: `scripts/build-sidecar.mjs::verifyArtifacts`.
- Sidecar health command outputs valid JSON: CLI unit test plus manual smoke test.
- Tests pass: `npm run test`.
- Lint passes: `npm run lint`.
- Fresh setup under 15 minutes: README setup flow plus `npm run setup` or `make setup`.
- README accurately reflects setup: review checklist in Phase 0 PR.
