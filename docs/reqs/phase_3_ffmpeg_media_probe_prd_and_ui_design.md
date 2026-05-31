# Phase 3 — FFmpeg Bundling & Media Probe
## Detailed Product Requirements Document (PRD), UX Spec, Interaction Design & Technical Design

**Product:** Speech Cleanup Desktop App  
**Phase:** P3 — FFmpeg bundling and media probe  
**Dependencies:** Phase 0 + Phase 1 complete  
**Primary Goal:** Reliably ingest creator video/audio files and expose trusted metadata to the user and downstream pipeline phases.

---

# Product Direction Update — Single-File MVP Strategy

## Strategic Product Decision

The MVP will support a **single-file workflow only**.

Users will:

- upload one file
- preview enhancements
- export the cleaned result
- optionally replace the current file with another

The app will NOT initially support:

- multi-file ingest
- visible processing queues
- batch export workflows
- simultaneous exports

This decision is intentional and prioritizes:

- reduced UX complexity
- faster time-to-market
- stronger focus on core audio quality
- simpler mental model for creators
- lower engineering overhead
- higher reliability for long-running media processing

Internally, the application will still use a job-oriented processing architecture so future queue/batch functionality can be introduced without rewriting the media pipeline.

This means:

- backend processing remains asynchronous
- progress/cancellation architecture remains job-based
- UI is simplified to a single active media workspace

rather than a queue management experience.

---

# 1. Executive Summary

Phase 3 is the first time the application becomes “media-aware.” Prior phases established:

- repository + CI foundation
- desktop shell
- typed IPC bridge between frontend and Go sidecar

Phase 3 introduces:

- bundled FFmpeg/ffprobe runtime
- local media inspection
- metadata extraction
- codec/container compatibility detection
- unsupported/corrupt file handling
- ingestion diagnostics UI
- standardized media object model for all future phases

This phase does **not** process media yet.

It answers one question extremely well:

> “Can we safely understand this file and prepare it for downstream processing?”

This phase becomes the canonical source of truth for:

- duration
- codecs
- audio characteristics
- stream topology
- file validity
- compatibility state

Every later phase depends on the media graph defined here.

---

# 2. Product Goals

## Primary Goals

1. Detect and parse common creator media formats reliably.
2. Return structured metadata within 1–3 seconds for most files.
3. Surface clear errors for unsupported/corrupt files.
4. Provide a UI foundation for future queue/export workflows.
5. Ensure all probing occurs locally with zero telemetry.

---

## Secondary Goals

1. Establish canonical media schema for the app.
2. Make probe results deterministic and testable.
3. Handle difficult filesystem/path edge cases.
4. Build user trust via transparent diagnostics.

---

# 3. Non-Goals

The following are explicitly out of scope for Phase 3:

- waveform rendering
- preview playback
- audio enhancement
- remuxing/export
- transcoding
- batch processing
- thumbnail extraction beyond lightweight poster frame support
- GPU acceleration
- cloud processing
- speech analysis

---

# 4. User Personas

## Persona A — Solo Creator

Uploads:
- OBS recordings
- Zoom recordings
- screen captures
- talking-head YouTube videos

Needs:
- confidence their files work
- quick feedback
- understandable errors

Pain points:
- codec confusion
- broken recordings
- unsupported exports from editing software

---

## Persona B — Agency Editor

Uploads:
- many client files
- mixed formats
- long videos

Needs:
- reliability
- no crashes
- clear compatibility indicators

Pain points:
- unpredictable media containers
- malformed exports

---

## Persona C — Non-Technical User

Needs:
- drag file in
- see “ready” or “problem”
- no codec terminology required

---

# 5. UX Principles

## 5.1 Invisible Complexity

The app should understand technical media details without forcing users to.

The default UX language should avoid:
- “mux”
- “container”
- “stream index”
- “sample format”

Instead use:
- Video format
- Audio format
- Duration
- Ready to process

Advanced details are available via expandable diagnostics.

---

## 5.2 Fast Feedback

User should understand file state within:

- <500ms for optimistic UI acknowledgement
- <3s for completed probe

The app should never appear frozen.

---

## 5.3 Trust Through Transparency

When something fails:

- explain why
- explain what formats work
- explain how to fix it

Never:
- silently fail
- show raw ffmpeg stderr
- expose stack traces

---

## 5.4 Local-First Confidence

Users must clearly understand:

- files stay local
- probing is offline
- nothing uploads

This should appear subtly in the UI.

---

# 6. User Stories

## Core Stories

### US-001 — Probe Supported Video

As a creator,
I want to drag in a video,
so that I can confirm the app recognizes it.

Acceptance Criteria:
- file appears immediately
- metadata loads automatically
- duration visible
- status becomes “Ready”

---

### US-002 — Unsupported Format

As a user,
I want clear feedback when a file is unsupported,
so that I know how to fix it.

Acceptance Criteria:
- clear error state
- file row remains visible
- suggested supported formats shown

---

### US-003 — Corrupt Media

As a user,
I want corrupt files detected safely,
so the app doesn’t crash.

Acceptance Criteria:
- no crash
- deterministic error state
- retry possible

---

### US-004 — Technical Diagnostics

As a power user,
I want to inspect codecs/sample rates,
so that I can troubleshoot media compatibility.

Acceptance Criteria:
- expandable advanced panel
- accurate stream information
- copy diagnostics button

---

# 7. Information Architecture

## Primary Screens in Phase 3

### 1. Empty Workspace State

Default application state before a file is loaded.

### 2. Active File — Probing State

Single loaded file is being analyzed.

### 3. Active File — Ready State

Metadata available and file validated.

### 4. Active File — Error State

Unsupported/corrupt/problematic file.

### 5. Diagnostics Drawer

Advanced technical details.

---

## Single-File Workspace Model

The Phase 3 application behaves as a focused single-document workspace.

At any moment:

- exactly one file may be active
- one processing pipeline may exist
- one preview/export session may exist

Loading a new file replaces the current workspace after user confirmation.

This model intentionally optimizes for:

- clarity
- simplicity
- fast onboarding
- focused creator workflows

rather than media queue management.

---

# 8. Detailed UI Design

# 8.1 Main Window Layout

```
┌──────────────────────────────────────────────┐
│ Header                                       │
│ Speech Cleanup                               │
│ “All processing happens locally”             │
├──────────────────────────────────────────────┤
│                                              │
│                                              │
│              ACTIVE WORKSPACE                │
│                                              │
│     Drag a video here                        │
│                                              │
│                  or                          │
│                                              │
│           [ Browse file ]                    │
│                                              │
│     Supported: MP4, MOV, MKV, WebM           │
│                                              │
├──────────────────────────────────────────────┤
│ Current File                                 │
│                                              │
│ ┌──────────────────────────────────────────┐ │
│ │ thumbnail  filename.mp4                 │ │
│ │ 12m 14s • H.264 • AAC • Ready           │ │
│ │                                          │ │
│ │ [Details] [Replace File] [Clear Workspace]        │ │
│ └──────────────────────────────────────────┘ │
│                                              │
├──────────────────────────────────────────────┤
│ Footer                                       │
│ Local processing • No uploads                │
└──────────────────────────────────────────────┘
```

---

# 8.2 Empty State Design

## Objective

Communicate:

- what the app does
- what file types work
- privacy guarantee
- primary CTA

---

## Content

### Headline

> Clean speech from your videos.

### Subcopy

> Drag in a video to inspect and prepare it for enhancement.

### CTA

Primary button:

> Browse files

Secondary affordance:

> Drag & drop anywhere

### Supported Formats

Displayed inline:

- MP4
- MOV
- MKV
- WebM

### Privacy Message

Small muted text:

> Your files never leave your computer.

---

# 8.3 Active File Card Design

The active file is represented as a single persistent workspace card.

## Card Layout

```
┌────────────────────────────────────────────┐
│ [thumb]  my-video.mp4                     │
│          1920×1080 • 14m 12s              │
│          H.264 • AAC • Stereo • 48 kHz    │
│                                            │
│          ● Ready                           │
│                                            │
│ [Details] [Retry] [Remove]                │
└────────────────────────────────────────────┘
```

---

## Card States

### 1. Queued

Status:
- Gray dot
- “Waiting to analyze”

### 2. Probing

Status:
- animated spinner
- “Inspecting media…”

### 3. Ready

Status:
- green dot
- “Ready”

### 4. Unsupported

Status:
- yellow warning icon
- “Unsupported format”

### 5. Corrupt/Error

Status:
- red icon
- “Couldn’t read file”

---

# 8.4 Diagnostics Drawer

## Purpose

Expose advanced details without cluttering the main UX.

Opened via:

> [Details]

Drawer slides from right side.

---

## Drawer Layout

```
┌──────────────────────────────┐
│ File Details                 │
├──────────────────────────────┤
│ Filename                     │
│ interview-final-v2.mov       │
│                              │
│ Container                    │
│ QuickTime / MOV              │
│                              │
│ Duration                     │
│ 00:14:12.102                 │
│                              │
│ Video                        │
│ Codec: H.264                 │
│ Resolution: 1920×1080        │
│ FPS: 29.97                   │
│ Bitrate: 18 Mbps             │
│                              │
│ Audio                        │
│ Codec: AAC                   │
│ Channels: Stereo             │
│ Sample Rate: 48000 Hz        │
│ Bitrate: 192 kbps            │
│                              │
│ Compatibility                │
│ ✓ Ready for enhancement      │
│                              │
│ [Copy diagnostics]

Copy feedback requirement:

After successful copy action:

- button changes to checkmark state
- success toast appears

Example:

✓ Diagnostics copied

Toast duration:
- 2–3 seconds           │
└──────────────────────────────┘
```

---

# 8.5 Error UX

## Human-readable Error Mapping

Every technical error must map to an understandable user action.

The application should never expose internal codes such as:

- UNSUPPORTED_CODEC
- NO_AUDIO_STREAM
- FFPROBE_FAILURE

The UI layer converts internal errors into actionable messages.

| Internal Error | User Message |
|---|---|
| UNSUPPORTED_CODEC | We can't read this audio format. Please try an MP4 or MOV file. |
| NO_AUDIO_STREAM | This video appears to be silent. Speech cleanup requires an audio track. |
| UNSUPPORTED_CONTAINER | This file type isn't supported yet. Try MP4, MOV, MKV, or WebM. |
| FILE_NOT_FOUND | We couldn't locate this file. It may have been moved or deleted. |
| CORRUPT_MEDIA | We couldn't read this file. It may be damaged or incomplete. |
| ACCESS_DENIED | We don't have permission to access this file. |

---

## Unsupported Format

### Message

> This file format isn’t currently supported.

### Additional Guidance

> Recommended formats:
> MP4 (H.264 + AAC)

### CTA

- Remove file
- Learn more

---

## Corrupt File

### Message

> We couldn’t read this file.

### Guidance

> The file may be damaged or incomplete.

### CTA

- Retry
- Remove

---

## Missing Audio Stream

### Message

> No audio track detected.

### Guidance

> This video can’t be enhanced because it contains no audio.

---

# 9. Interaction Design

# 9.1 Drag-and-Drop Flow

## Single-File Behavior

If no file is loaded:
- dropped file becomes the active workspace

If a file is already loaded:
- user is prompted before replacement

Prompt example:

> Replace the current file?
>
> Your current preview/export progress will be lost.

Actions:
- Replace
- Cancel

Multiple-file drag/drop is not supported in the MVP.

## Safe Drop Interaction

If an active file already exists in the workspace and the user drags a new file into the window:

Visual behavior:
- background dimmed
- workspace card blurred
- highlighted drop target
- warning icon shown

Overlay copy:

> Replace current file?
>
> Dropping a new file will clear your current workspace and remove any preview or export progress.

Actions:
- Replace file
- Cancel

If multiple files are dropped simultaneously:
- only the first supported file is accepted
- user receives inline guidance:

> The current version supports one file at a time.

## Interaction Sequence

### Step 1 — Idle

Drop zone visible.

### Step 2 — Drag Hover

Visual changes:
- border glow
- elevated shadow
- background tint
- cursor feedback

Text changes:

> Drop files to inspect

---

### Step 3 — Drop

Immediate optimistic insertion into queue.

Rows appear instantly with:

- filename
- “Inspecting media…”

No blocking spinner overlay.

---

### Step 4 — Probe Complete

Metadata animates in.

Transition:
- fade-in metadata
- spinner resolves into status icon

---

# 9.2 Browse File Flow

### User Action

Click “Browse files”.

### Native Dialog

Filters:
- .mp4
- .mov
- .mkv
- .webm

Multi-select enabled.

---

# 9.3 Details Drawer Interaction

## Open

- click Details
- smooth 180–240ms slide animation

## Close

- Escape key
- click outside
- explicit X button

---

# 9.4 Retry Flow

If probing fails:

- Retry button visible
- re-runs ffprobe
- replaces previous error state

---

# 10. Accessibility Requirements

## Keyboard

All functionality accessible via keyboard.

Required:
- tab navigation
- enter/space activation
- escape closes drawer

---

## Screen Reader

Each file row must expose:

- filename
- duration
- status
- codec compatibility

Example:

> “interview.mp4, Ready, H.264 video, AAC audio, duration 14 minutes.”

---

## Color Accessibility

Never rely on color alone.

Every status also includes:
- icon
- text label

---

# 11. Technical PRD

# 11.1 Architecture Overview

```
React UI
   ↓
Tauri Command Layer (Rust)
   ↓
Go Sidecar
   ↓
ffprobe
```

---

# 11.2 Probe Flow

## Sequence Diagram

```
User drops file
    ↓
Frontend validates extension
    ↓
IPC call → media.probe(path)
    ↓
Go invokes ffprobe JSON output
    ↓
JSON parsed into typed structs
    ↓
Normalized media model returned
    ↓
UI updates row state
```

---

# 11.3 Canonical Media Schema

## Probe Response

```ts
interface MediaProbeResult {
  id: string
  path: string
  filename: string
  sizeBytes: number
  durationSeconds: number

  container: {
    format: string
    longName: string
  }

  video?: {
    codec: string
    width: number
    height: number
    fps: number
    bitrate?: number
    pixelFormat?: string
  }

  audio?: {
    codec: string
    channels: number
    sampleRate: number
    bitrate?: number
    layout?: string
  }

  compatibility: {
    supported: boolean
    issues: string[]
    warnings: string[]
  }
}
```

---

# 11.4 IPC Contract

## Command

```ts
media.probe(path)
```

---

## Success Response

```json
{
  "ok": true,
  "data": {
    "durationSeconds": 852.2
  }
}
```

---

## Failure Response

```json
{
  "ok": false,
  "error": {
    "code": "UNSUPPORTED_CODEC",
    "message": "Unsupported audio codec"
  }
}
```

---

# 11.5 Error Taxonomy

| Code | Meaning |
|---|---|
| FILE_NOT_FOUND | Path invalid |
| ACCESS_DENIED | File locked/no permission |
| UNSUPPORTED_CONTAINER | Unknown container |
| UNSUPPORTED_CODEC | Codec unsupported |
| NO_AUDIO_STREAM | Missing audio |
| CORRUPT_MEDIA | Probe parse failure |
| FFPROBE_FAILURE | Internal ffprobe error |
| UNKNOWN | Fallback |

---

# 11.6 FFmpeg Bundling Requirements

## Requirements

- Bundle platform-specific ffprobe binaries
- No system dependency required
- App functions offline
- Version pinned and reproducible

---

## Supported Platforms

| Platform | Arch |
|---|---|
| Windows | x64 |
| macOS | arm64 |
| macOS | x64 |

---

## FFmpeg Versioning

Pin exact version.

Example:

- FFmpeg 7.x

Must be documented in:
- README
- about screen
- licenses page

---

# 11.7 Performance Requirements

| Metric | Target |
|---|---|
| Small file probe | <1s |
| 1-hour file probe | <3s |
| UI acknowledgement | <100ms |
| Probe concurrency | 5 simultaneous |
| Memory overhead | <150MB |

---

# 11.8 Reliability Requirements

## Must Never

- crash app on malformed media
- freeze UI thread
- expose raw stderr
- orphan ffprobe processes

---

## Must Always

- timeout safely
- clean subprocesses
- return structured errors
- log failures locally

---

# 11.9 Logging Requirements

## Log Categories

### Probe Started

```json
{
  "event": "probe_started",
  "path": "file.mp4"
}
```

### Probe Completed

```json
{
  "event": "probe_completed",
  "duration_ms": 421
}
```

### Probe Failed

```json
{
  "event": "probe_failed",
  "code": "CORRUPT_MEDIA"
}
```

---

# 11.10 Single-File Workspace Constraints

## Supported

- one active file
- one active probe job
- one active processing pipeline
- replace/remove current file

---

## Not Supported in MVP

- multi-file queue
- batch export
- parallel processing
- simultaneous preview sessions

---

## Internal Architecture Note

Although the UX is single-file-only, backend processing should still use job abstractions internally to support:

- cancellation
- progress reporting
- retries
- future batch workflows

without major architectural rewrites.

---

# 12. Detailed State Machine

# 12.1 Workspace State Lifecycle

```
EMPTY
  ↓
FILE_LOADED
  ↓
PROBING
  ↓
READY
  ↓
REMOVED
  ↓
EMPTY
```

Error branch:

```
PROBING
  ↓
ERROR
  ↓
RETRYING
  ↓
READY or ERROR
```

---

# 13. Edge Cases

# 13.1 Very Large Files

Requirements:
- no full file reads
- metadata-only inspection
- streaming probe process

---

# 13.2 Unicode Paths

Must support:

- Japanese filenames
- emojis
- accents
- RTL languages

Example:

```
🎥 интервью финал.mov
```

---

# 13.3 Long Windows Paths

Support:

- >260 character paths

---

# 13.4 Multiple Audio Tracks

Behavior:
- select default track
- expose track count in diagnostics
- explicitly indicate selected track in Ready state

Ready state example:

> Audio: Track 1 selected (Microphone)

If multiple audio tracks exist:

- first/default track selected automatically
- user informed of selection
- all tracks listed in diagnostics drawer

Diagnostics example:

Audio Tracks

Track 1 — AAC Stereo — Microphone
Track 2 — AAC Stereo — Game Audio
Track 3 — AAC Stereo — System Audio

---

# 13.5 Missing Duration

Fallback:
- show “Unknown duration”
- still allow probe success if streams valid

---

# 14. Visual Design System

# 14.1 Design Language

The app should feel:

- professional
- calm
- creator-focused
- trustworthy
- technical without intimidation

---

# 14.2 Style Direction

## Inspired By

- Linear
- Riverside
- Descript
- Notion

---

# 14.3 Visual Characteristics

### Surfaces

- soft elevation
- rounded cards
- minimal borders

### Typography

- clean sans-serif
- high readability
- strong hierarchy

### Motion

- subtle
- functional
- no playful bounces

---

# 14.4 Suggested Tokens

## Radius

- cards: 16px
- buttons: 12px

## Spacing Scale

- 4 / 8 / 12 / 16 / 24 / 32

## Animation

- fast: 120ms
- standard: 200ms
- slow: 320ms

---

# 15. QA Plan

# 15.1 Manual QA Matrix

| Scenario | Expected |
|---|---|
| MP4 H264 AAC | Ready |
| MOV H264 AAC | Ready |
| MKV Opus | Ready |
| txt renamed mp4 | Error |
| missing file | Error |
| 50 simultaneous files | Stable |
| unicode filename | Stable |
| 4GB file | Stable |

---

# 15.2 Automated Tests

## Unit Tests

- ffprobe parser
- codec normalization
- error mapping
- IPC serialization

---

## Integration Tests

- probe fixture media
- timeout handling
- subprocess cleanup

---

## UI Tests

- drag/drop
- retry flow
- drawer interactions
- keyboard accessibility

---

# 16. Acceptance Criteria

Phase 3 is complete when:

## Functional

- app probes all supported formats
- metadata accurate
- unsupported files handled safely
- diagnostics UI complete

---

## UX

- no blocking interactions
- probe feels responsive
- clear error messaging
- accessibility baseline achieved

---

## Technical

- deterministic IPC schema
- ffprobe bundled correctly
- logs generated
- no crashes in malformed-media tests

---

# 17. Risks & Mitigations

| Risk | Mitigation |
|---|---|
| FFmpeg licensing issues | Document provenance + LGPL audit |
| Corrupt files crash parser | Strict parsing + fuzz tests |
| Slow probing on large files | Metadata-only probe strategy |
| Unicode path bugs | Cross-platform fixture suite |
| UI jank on many files | Async probe queue |

---

# 18. Future Compatibility

This phase intentionally lays foundations for:

- waveform rendering (P9)
- extraction/remux (P4)
- quality analysis (P11)
- batch queue (P12)
- presets (P10)

The canonical media schema introduced here should remain stable throughout the product lifecycle.

---

# 19. Recommended Engineering Breakdown

## Frontend Engineer

Owns:
- drop zone
- queue UI
- diagnostics drawer
- state machine
- accessibility

---

## Backend/Systems Engineer

Owns:
- ffprobe invocation
- parser
- normalization layer
- subprocess lifecycle
- error taxonomy

---

## QA Engineer

Owns:
- fixture validation
- malformed media testing
- unicode path testing
- long-duration tests

---

# 20. Final Deliverable Summary

At the end of Phase 3, the product should:

- feel like a real desktop media application
- reliably understand creator video files
- provide trustworthy compatibility feedback
- establish a stable ingest architecture
- be ready for extraction/remux in Phase 4

This is the foundational “media intelligence” layer of the entire application.

