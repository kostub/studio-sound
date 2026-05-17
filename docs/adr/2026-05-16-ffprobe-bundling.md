# ADR: Bundling ffprobe (LGPL build)

**Status:** Accepted
**Date:** 2026-05-16

## Context

Phase 3 requires running `ffprobe` locally against creator-supplied media files. We do not want the user to install ffmpeg/ffprobe globally — the app must work offline and out of the box. We also need to keep our license posture defensible.

## Decision

Bundle a single `ffprobe` binary (no `ffmpeg`) from the BtbN/FFmpeg-Builds **LGPL** 7.x release line, per platform (Windows x64; macOS x64 and arm64 deferred until BtbN publishes macOS builds). Pin by SHA-256 in `third_party/ffprobe.lock.json`. Fetch + verify on `npm run setup` and in CI.

## Why LGPL (not GPL)

LGPL allows dynamic linking and bundling without forcing our application source under the GPL. Our usage is "execute as a separate process," which is even more permissive than dynamic linking.

## Why BtbN

BtbN/FFmpeg-Builds is the most actively maintained pre-built FFmpeg distribution and supplies clearly separated LGPL builds. Hashes pinned per platform mean a tampered upstream cannot silently change what we bundle.

## macOS status

BtbN does not currently publish macOS builds (`macos64`/`macosarm64` return HTTP 404). The `ffprobe.lock.json` has only `windows-amd64` for Phase 3. A follow-up will source macOS binaries from an alternative provider (e.g., evermeet.cx or a self-hosted build) and add them to the lock file.

## Why no codesigning in Phase 3

Phase 3 ships a development / unsigned Tauri build. macOS Gatekeeper quarantine is documented in `docs/troubleshooting.md` (`xattr -d com.apple.quarantine`). Production codesigning + notarisation belong to a later release-hardening phase.

## Consequences

- First-launch on macOS may flag ffprobe under Gatekeeper once macOS binaries are added. Documented workaround in `docs/troubleshooting.md`.
- Bundle grows by ~20 MB per platform for Windows (~60 MB total once macOS binaries are sourced).
- Lock-file bumps require manual SHA recomputation and a CI test pass.
- We must reproduce the LGPL text inside the bundle (`third_party/LICENSE.ffmpeg-lgpl.txt`) and reference it in README. An in-app license screen is deferred.
