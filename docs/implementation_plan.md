# Phased Implementation Plan — Speech Cleanup Desktop App

**Product:** Local-first Windows/Mac desktop utility that ingests a video file, cleans the spoken audio, and exports a remuxed upload-ready video.

**Tech stack (per proposal):** Tauri 2 (desktop shell) + React/TypeScript (UI) + Go sidecar (heavy media processing) + FFmpeg (decode/probe/remux) + ONNX Runtime (inference) + DeepFilterNet baseline / RNNoise low-CPU mode (enhancement models).

**Target end state of this plan:** the 12-week MVP described in §"Milestone plan" of the proposal — drag in video, A/B preview clean speech, export remuxed video locally, with batch queue, signed/notarized installers, and basic crash reporting.

---

## How to read this document

Each phase is sized to:

- Be **big enough** to justify its own LLD and a dedicated Claude Code implementation plan.
- Be **small enough** to be code-reviewed and QA-tested in isolation.
- Produce **something demonstrable** through the app/UI or a CLI surface that QA can drive without internal knowledge of the code.

The dependency graph is given so multiple developers can work in parallel. Phases on the same level of the graph have no dependency on each other.

### Dependency graph (high level)

```
P0 ── P1 ──┬── P2  (test corpus, runs in parallel from day 1)
           ├── P3 ── P4 ──┬── P7 ── P9 ──┐
           │              │              ├── P10 ── P12 ── P13 ── P14
           │              └── P8 ────────┤
           ├── P5 ────────────────────────┤
           ├── P6 ────────────────────────┘
           │
           └── P11 (depends on P4 + P7)
```

P0 and P1 are sequential and must land before anything else. After that, four parallel tracks open up: **media pipeline (P3→P4)**, **UI shell (P5, P6)**, **test corpus (P2)**, and the model/DSP track (P7, P8) which kicks off as soon as P4 lands.

---

## Phase 0 — Repository, toolchain, and CI bootstrap

**Dependencies:** none.

**Goal/Outcome:** A fresh monorepo with the Tauri 2 shell, a Go sidecar binary, and a React/TypeScript frontend that all build green on Windows and macOS in CI. No product features yet; this is the developer-experience floor that every later phase stands on.

**Description of what is included:**

- Monorepo layout (e.g. `/app` for Tauri+React, `/sidecar` for Go, `/scripts`, `/docs`, `/test-assets`).
- Tauri 2 project initialised with React + TypeScript + Vite. The default window opens with a placeholder "Hello" screen.
- Go sidecar project skeleton with a single `health` command that prints a JSON payload. Build scripts produce `sidecar-win-x64.exe`, `sidecar-mac-arm64`, and `sidecar-mac-x64`.
- Tauri configured to bundle the Go sidecar binary (`tauri.conf.json` `externalBin`).
- Dev tooling: ESLint, Prettier, TypeScript strict mode, `golangci-lint`, `gofmt`, pre-commit hooks.
- Unit-test runners wired up for both sides (Vitest for frontend, `go test` for sidecar) with one trivial passing test each.
- GitHub Actions (or equivalent) running lint + test + build matrix on Windows and macOS for every PR.
- README with one-command local setup.

**Testability:**

- Fresh-clone bootstrap: a developer with no prior context runs the documented setup command and gets a launching Tauri window within 15 minutes on both Windows and macOS.
- CI runs are green on a no-op PR.
- The launched dev app shows the placeholder screen and `npm run tauri dev` does not error.
- A QA tester can manually run the produced sidecar binary from a terminal and observe the JSON health output.

---

## Phase 1 — Frontend ↔ sidecar IPC contract

**Dependencies:** P0.

**Goal/Outcome:** A typed, versioned, stable IPC channel between the React UI and the Go sidecar, so every later phase has a single well-known way to call into Go. After this phase, the placeholder UI can call a Go function and render its result.

**Description of what is included:**

- Decision and documentation of the IPC mechanism (Tauri command invoking sidecar via stdin/stdout JSON-RPC, or Tauri sidecar HTTP on a localhost ephemeral port — whichever the team picks; document the choice).
- A small Tauri command layer in Rust that forwards typed requests to the Go sidecar and returns typed responses to the frontend.
- Shared schema definitions (e.g. JSON Schema or hand-maintained TS + Go types) for request/response envelopes, with a `version` field and standardised error shape (`code`, `message`, `details`).
- A `ping` round-trip command end-to-end (UI button → Tauri command → Go sidecar → response → UI render) plus an `echo` command that takes a string.
- Lifecycle: sidecar spawned on app start, killed cleanly on app quit, restarted on crash, with logs forwarded to a file.
- Structured logging on both sides written to a known per-OS app data directory.

**Testability:**

- A "Diagnostics" hidden screen in the app exposes a "Ping sidecar" button and an "Echo" textbox; both round-trip successfully and display the response.
- Killing the sidecar process externally causes the UI to surface a clean error and the app to respawn it on the next call.
- QA can verify log files appear in the documented location on both OSes after using the diagnostics screen.
- Unit tests cover the envelope encoding/decoding on both sides, including malformed-payload handling.

---

## Phase 2 — Benchmark corpus and audio regression harness

**Dependencies:** P0 (just repo scaffolding; no IPC or processing needed).

**Goal/Outcome:** A reusable set of representative creator clips and an automated harness that any later phase can run to measure "did my change make audio better or worse?" This phase is independently valuable and runs in parallel with the entire UI/IPC track.

**Description of what is included:**

- Curated corpus of 25–40 short representative clips covering: tutorial/screencast with fan noise, talking-head with HVAC, untreated-room reverb, keyboard bleed, low-bitrate source, multi-speaker, music bed, and clean-source control. Stored either in-repo (if size permits) or in a documented external store with a fetch script.
- Metadata sidecar per clip (source description, expected pain category, "do not over-process" flag for music beds).
- Harness CLI (Go, lives in the same sidecar repo or a sibling) that takes an input directory and a processing function and produces metrics: PESQ or a similar speech-quality proxy, loudness (LUFS), peak (dBTP), and a simple "speech-vs-noise SNR" estimate.
- Comparison report: HTML or markdown table showing per-clip before/after metrics, plus aggregate pass/fail thresholds.
- A no-op "passthrough" processor wired up as the first reference, so the harness end-to-end runs and produces a baseline report.

**Testability:**

- QA can run a single command and get an HTML report comparing original vs. processed clips.
- The passthrough run produces near-identical metrics for input and output (sanity check that the harness does not lie).
- The report is checked into a known location (or attached to the CI run) so reviewers can eyeball it.
- A deliberately-degraded fake processor (e.g. attenuates by 6 dB) produces the expected loudness drop in the report — proves the metrics are wired correctly.

---

## Phase 3 — FFmpeg bundling and media probe

**Dependencies:** P1.

**Goal/Outcome:** The app can ingest any common creator video file and report what is inside it (container, video codec, audio codec, sample rate, channels, duration). This is the foundation for every later media operation.

**Description of what is included:**

- LGPL-safe FFmpeg static binary bundled per platform (Windows x64, macOS arm64, macOS x64). Documented build provenance and licence audit.
- Go sidecar wrappers that call `ffprobe` and parse the JSON output into typed structs.
- IPC commands: `media.probe(path)` returns the parsed metadata or a structured "unsupported" / "not found" / "corrupt" error.
- Format support matrix documented: MP4, MOV, MKV, WebM at minimum; H.264 / H.265 / VP9 video; AAC / Opus / PCM audio.
- Path handling correct on both OSes (Windows backslashes, Unicode filenames, paths with spaces, very long paths).
- Telemetry-free; probing happens entirely locally.

**Testability:**

- A minimal "Probe a file" diagnostics screen lets QA pick a file and view the parsed metadata.
- QA can drop the corpus from P2 onto the probe screen and confirm that all 25–40 clips probe successfully.
- Deliberately-corrupt and unsupported files (e.g. a renamed `.txt`) produce the documented error codes, not crashes.
- Automated test: a small fixture set of files (at minimum MP4/H.264/AAC, MOV/H.264/AAC, MKV/H.265/Opus) probes to expected values.

---

## Phase 4 — Audio extract and video remux pipeline

**Dependencies:** P3.

**Goal/Outcome:** Given an input video, the app can extract its audio to a known PCM working format, and given an input video plus a replacement audio file, produce a remuxed output video where the original video stream is preserved and the new audio is correctly placed. This is the core of the export path.

**Description of what is included:**

- Go sidecar function: `media.extract_audio(input_path, output_pcm_path)` — uses FFmpeg to extract to mono or stereo PCM WAV (48 kHz default; documented).
- Go sidecar function: `media.remux(input_video_path, replacement_audio_path, output_path)` — copies the original video stream untouched, re-encodes audio to AAC at a documented bitrate, preserves duration and timestamps.
- Sync handling: explicit verification that audio start matches video start, and drift on long files (>30 min) stays under 40 ms (the proposal's acceptance criterion).
- Progress reporting via IPC events for both extract and remux.
- Cancellation: a long-running extract or remux can be cancelled cleanly.
- Clean temp-file management with a documented working directory.

**Testability:**

- A "Passthrough export" diagnostics flow: pick a video, extract audio, immediately remux that exact audio back, save output. The output should be visually and audibly identical to the input.
- QA can run the passthrough on every clip in the P2 corpus and confirm 100% success.
- Sync drift tested on a synthetic clapperboard clip (>30 min, with a known transient at start and end): drift measured and reported to be under 40 ms.
- Cancellation test: starting an extract on a long clip and clicking cancel terminates the FFmpeg child process and cleans up temp files.
- Progress events arrive at sub-second intervals during a long operation.

---

## Phase 5 — Drag-and-drop ingest UI and file queue

**Dependencies:** P1 (does not need P3 or P4 — uses mocked metadata until those land).

**Goal/Outcome:** The user-facing main screen of the app: drop video files in, see them listed with a thumbnail, name, duration, and a per-file status. No processing yet; this phase is purely the ingest surface.

**Description of what is included:**

- Main window layout: drop zone, file list, header, footer/status bar.
- OS-level drag-and-drop accepting one or many files; click-to-browse fallback.
- Per-file row: filename, duration, "Ready" / "Processing" / "Done" / "Error" status placeholder, remove button.
- File list is in-memory only at this stage; closing the app clears it (persistence is later).
- Empty state, single-file state, and many-file state all designed and implemented.
- Feature-flagged to call P3 `media.probe` if available, otherwise use a mock that returns plausible metadata. This decoupling lets P5 ship before P3 is merged.
- Visual design follows a documented design-token system (colours, spacing, typography); accessibility basics: keyboard navigation through list, focus rings, screen-reader labels.

**Testability:**

- QA drag-drops 1, 5, and 50 files and the UI handles each case gracefully.
- Unsupported file types (e.g. `.docx`, `.txt`) are rejected with a visible inline message.
- Keyboard-only navigation: tab through controls, activate buttons with Enter/Space.
- Screenshots match the design spec (visual regression test suite recommended).
- Demoable in the app on its own without any processing being implemented yet.

---

## Phase 6 — Settings and preferences

**Dependencies:** P1.

**Goal/Outcome:** A persistent settings panel where the user controls non-processing preferences (default output folder, default preset, theme, telemetry opt-in, etc.). Independent surface that can be built in parallel.

**Description of what is included:**

- Settings UI screen reachable from a menu or gear icon.
- Persistent store on disk (e.g. JSON file in the per-OS app config directory) read on app start and written on change.
- Initial settings: default output folder, "open output folder when done" toggle, theme (system/light/dark), telemetry opt-in (default off), default export preset name (placeholder until presets exist).
- Migration story documented: how new settings keys roll out without breaking existing user settings files.
- IPC surface for sidecar to read settings if needed (currently only frontend uses them, but the path is stubbed).

**Testability:**

- QA changes a setting, quits the app, relaunches; the setting persists.
- Manually editing the settings file with an unknown future key does not crash the app on next launch.
- Theme toggle visibly switches the UI without restart.
- Reset-to-defaults button restores known good values.

---

## Phase 7 — Enhancement engine integration (denoise baseline)

**Dependencies:** P4.

**Goal/Outcome:** The Go sidecar can take a PCM audio file and produce a denoised PCM file using a bundled DeepFilterNet ONNX model running through ONNX Runtime. This is the first real "AI" step.

**Description of what is included:**

- ONNX Runtime bundled per platform with appropriate execution providers (CPU baseline, optional CoreML on macOS, optional DirectML on Windows — flagged for later if too risky).
- DeepFilterNet model file bundled with documented licence (Apache-2.0 / MIT).
- Go bindings to ONNX Runtime (via cgo or a maintained Go wrapper — choice documented).
- `audio.enhance(input_pcm, output_pcm, options)` IPC command with a single `intensity` parameter (0.0–1.0) implementing dry/wet mix.
- Chunked processing with overlap-add to handle long files without memory blowup; documented chunk size, overlap, and lookahead trade-offs.
- RNNoise as a low-CPU fallback path, selectable via options.
- Progress events as enhancement runs.
- First-call model warm-up handled (cache the loaded session between calls).

**Testability:**

- Run the P2 corpus through `audio.enhance` and re-run the P2 harness. The aggregate report shows improved speech-quality metrics on the noisy categories versus the passthrough baseline. Internal listening confirms preference on ≥60% of noisy clips (matches the proposal's MVP acceptance criterion).
- Diagnostics screen: pick a video → extract → enhance → save WAV. QA can listen to before/after pairs.
- A 30-minute file processes without crashing or memory growth past a documented bound.
- Cancellation works mid-enhancement.

---

## Phase 8 — DSP finishing chain (loudness, EQ, gentle compression)

**Dependencies:** P4. (Does **not** depend on P7 — operates on PCM regardless of source.)

**Goal/Outcome:** A deterministic DSP chain that takes a PCM file and outputs one with consistent loudness, controlled peaks, and a tone-shaped voice band. Independent of the AI denoiser, so the two can land in either order.

**Description of what is included:**

- High-pass filter (e.g. ~80 Hz) to remove rumble.
- Voice presence/clarity tilt EQ.
- Gentle wide-band compression with documented ratio/threshold/attack/release.
- Loudness normalisation to a YouTube-friendly target (e.g. −14 LUFS integrated) with a true-peak ceiling (e.g. −1 dBTP).
- Implemented as either a chained FFmpeg filtergraph or a small Go DSP module — choice documented based on benchmark.
- `audio.finish(input_pcm, output_pcm, target_lufs, peak_ceiling)` IPC command.

**Testability:**

- Run the P2 harness on `finish`-only output: loudness lands within ±0.5 LU of target; peaks never exceed ceiling.
- Quiet input gets brought up; hot input gets tamed; both stay artefact-free in listening tests.
- Diagnostics screen exposes the chain so QA can A/B before/after.
- Deterministic: running twice on the same input produces bit-identical output.

---

## Phase 9 — A/B preview engine

**Dependencies:** P7 (and ideally P8, but can stub).

**Goal/Outcome:** The user-facing trust mechanism. The user picks a file, hits "Preview," waits a few seconds, and can instantly toggle Original / Cleaned with a single click while looking at a waveform. This is the first time the product feels real.

**Description of what is included:**

- Preview is computed on a short range (e.g. 30–45 s window centred on a detected speech-rich segment, or the first 30 s if speech detection isn't ready yet).
- UI: waveform display, transport controls (play/pause, seek), a big A/B toggle, an intensity slider tied to P7's dry/wet.
- Audio playback plumbed through the system's default device with low-latency switching between original and cleaned buffers.
- Preview never blocks the rest of the app; runs as an async sidecar job with progress.
- Preview cache: re-running preview on the same file with the same settings is instant.

**Testability:**

- QA opens a clip, hits Preview, hears the difference within ~10 s on a modern laptop.
- A/B toggle is gapless or near-gapless (documented latency target, e.g. <100 ms).
- Intensity slider audibly changes the result without re-running the model from scratch (because dry/wet is a mix, not a re-run).
- Preview survives clicking around, scrubbing, and toggling many times without leaks or crashes.
- Demoable as a complete user-facing feature.

---

## Phase 10 — Export pipeline with platform presets

**Dependencies:** P4, P7, P8.

**Goal/Outcome:** The end-to-end happy path the entire product exists for: drop video → click Export → get a remuxed upload-ready MP4 written to disk. Single-file only at this phase; batch comes next.

**Description of what is included:**

- Export flow UI: from the file row in the queue, pick a preset (initial set: "Clean speech – YouTube," "Clean speech – LinkedIn," with documented loudness/peak targets per the §"Pricing and go-to-market" platform section), confirm output location, hit Export.
- Pipeline: extract audio (P4) → enhance (P7) → finish (P8) → remux (P4) → write to user's chosen location.
- Progress UI showing the current stage and overall percentage; cancellable.
- Error surfacing: any pipeline stage failing produces a clear message and leaves no partial files.
- Output filename convention documented (e.g. `{originalname}-cleaned.mp4`, with collision handling).
- "Open output folder" button after success.

**Testability:**

- QA exports every clip from the P2 corpus end-to-end. 95%+ produce playable, sync-correct, loudness-correct output.
- The exported file plays in VLC, QuickTime, and Windows Media Player.
- Sync drift on a long file is verified to be under 40 ms (acceptance criterion from the proposal's MVP table).
- Cancellation mid-export leaves no half-written output and no orphan temp files.
- A re-export of the same file with the same preset is bit-identical (deterministic pipeline).
- This is the first phase that meets the proposal's MVP acceptance bar end-to-end and can be demoed to a creator for feedback.

---

## Phase 11 — Speech analysis and quality meter

**Dependencies:** P4, P7.

**Goal/Outcome:** The app gives the user honest, up-front feedback about the source quality before they commit to an export, and can avoid over-processing non-speech sections. This is the proposal's "expectation management" layer.

**Description of what is included:**

- Speech/non-speech segmentation on the extracted audio (e.g. simple energy + spectral heuristic, or a small VAD model — choice documented).
- A "source quality" estimate derived from SNR, reverb estimate, clipping detection, and bitrate; rendered as a meter in the file row (e.g. Poor / Fair / Good / Excellent).
- Music-bed awareness: non-speech sections are processed less aggressively (mix toward dry, or skip enhancement entirely).
- Warning surfaced when source is too degraded for the "studio" promise — links to a help article in the app.
- Exposed via IPC so the export pipeline can use the segmentation for music-aware processing.

**Testability:**

- The P2 corpus's "do not over-process" music-bed clips show audibly less processing on the music sections after this phase lands.
- Deliberately bad source clips (very low bitrate, heavy clipping) trigger the "too degraded" warning.
- The quality meter for clean-source control clips reads "Excellent" and for the worst untreated-room clips reads "Poor."
- QA can verify the warning copy matches the help article it links to.

---

## Phase 12 — Batch queue

**Dependencies:** P10.

**Goal/Outcome:** The user can drop many files at once and walk away while the app processes them sequentially, with sensible failure handling. This unlocks the course-creator and small-agency personas.

**Description of what is included:**

- Queue UI: list of files, each with status (Queued / Running / Done / Failed / Skipped), per-file progress, an aggregate progress bar, and an ETA estimate.
- Sequential processing by default; parallel mode flagged off for MVP unless benchmarked safe.
- Per-file failure does not stop the queue; failed items are skipped with a visible error and can be retried.
- Pause / resume / cancel-all controls.
- "Notify on completion" via OS notification (respects Settings opt-in from P6).
- Summary report at end: N succeeded, M failed, total time, total time saved estimate.

**Testability:**

- QA queues 20 files including 2 deliberately-broken ones; the queue completes the 18 good files and clearly marks the 2 failures.
- Pausing and resuming mid-queue works and the in-flight file resumes cleanly (or is restarted from the start of that file — documented behaviour either way).
- The system notification fires on completion when opted in, and does not fire when opted out.
- A 50-file overnight run completes without leaks; memory and temp-disk usage stay within documented bounds.

---

## Phase 13 — Packaging, signing, notarisation, and auto-update

**Dependencies:** P12 (logically — should run on the feature-complete app, though the work of setting up signing infrastructure can begin earlier in parallel; the test-and-verify part needs the real app).

**Goal/Outcome:** Real, distributable installers for Windows and macOS that pass platform trust checks and can update themselves. The proposal calls this out as a launch blocker because of SmartScreen / Gatekeeper warnings.

**Description of what is included:**

- Windows: signed `.msi` or `.exe` installer (Authenticode signing certificate provisioned and documented).
- macOS: signed and notarised `.dmg` per Apple's documented notarisation workflow, for both arm64 and x64; universal build or two installers — choice documented.
- Auto-update mechanism wired up (Tauri updater) with signed update manifests served from a documented endpoint.
- First-run experience: installer creates Start Menu / Applications entries; uninstaller cleans up.
- Versioning scheme documented (semver, with build metadata).
- Release process documented end-to-end so a future developer can cut a release.

**Testability:**

- Fresh Windows 11 VM and fresh macOS VM: download installer from a staging URL, install, launch, no SmartScreen / Gatekeeper warnings.
- Trigger an update from version N to N+1; the running app picks it up, downloads, restarts, and is on N+1. Roll-back path documented.
- Uninstall removes app files and optionally user data (per documented choice).
- Notarisation log inspected; no warnings.

---

## Phase 14 — Crash reporting, basic telemetry, and licensing plumbing

**Dependencies:** P13 (so reports come from real installed builds and licensing keys can be tied to real distribution).

**Goal/Outcome:** Commercial-readiness baseline: the developer can see crashes from real users, count export volume in aggregate, and gate "Pro" features behind a licence key. Required to launch under the proposal's freemium/trial pricing model.

**Description of what is included:**

- Crash reporter (e.g. Sentry or self-hosted equivalent) wired into both Tauri and the Go sidecar, behind the telemetry opt-in from P6.
- Minimal anonymous usage counters: app launches, exports completed, aggregate crash rate. No file content, no path content, no transcripts ever leave the machine.
- Licence-key system: free tier (capped exports, per the proposal) versus Pro tier (uncapped). Offline-friendly licence validation. Trial state tracked locally.
- "Enter licence key" UI in Settings; "Buy Pro" link to the marketing site (real Paddle/merchant integration not in scope for this phase — just the UI surface and key-entry flow).
- Privacy policy page accessible from Settings.

**Testability:**

- Trigger a deliberate crash; the report shows up in the dashboard within minutes (when telemetry is on).
- With telemetry off, no network calls leave the machine during a full export — verified by network capture during QA.
- Free tier hits the export cap and presents the upgrade prompt; entering a valid test licence key removes the cap.
- Invalid / tampered keys are rejected with a clear message.
- This phase concludes the proposal's MVP scope and matches the "crash-free sessions >99% on beta" success metric in the milestone table.

---

## Mapping back to the proposal's 12-week milestone plan

| Proposal milestone weeks | Phases that deliver it |
|---|---|
| Weeks 1–2: corpus, harness, probe/extract/remux CLI | P0, P1, P2, P3, P4 |
| Weeks 3–4: first enhancement engine + leveling | P7, P8 |
| Weeks 5–6: Tauri shell, drag/drop, preview, export | P5, P6, P9, P10 |
| Weeks 7–8: presets, artifact controls, quality meter, batch queue | P11, P12 |
| Weeks 9–10: private beta with creators | (beta cycle on top of P12 build) |
| Weeks 11–12: payments, signing/notarisation, launch | P13, P14 |

## Parallelisation guide for staffing

- **Developer A (full-stack lead):** P0 → P1 → P3 → P4. Owns the media-pipeline spine.
- **Developer B (frontend-leaning):** P5 → P6 → P9, then P10 with Developer A and P12.
- **Developer C (audio/ML-leaning):** P2 (early), then P7 and P8 in parallel, then P11.
- **Developer D (release engineer, can be part-time):** P13 setup work begins as soon as P0 lands, finalises after P12.

P14 lands at the very end and can be picked up by whoever frees up first.
