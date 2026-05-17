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
