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
`third_party/ffprobe.lock.json`, decompresses the per-platform `ffprobe`
checked into `third_party/ffprobe/` into `app/src-tauri/binaries/`, and
verifies its SHA-256 (no network). Platforms without a checked-in binary
fall back to downloading + verifying the upstream archive.
