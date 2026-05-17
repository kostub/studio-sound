// Package main is a deterministic test-only stand-in for the real ffprobe
// binary. The runner tests under sidecar/internal/media use it to control
// stdout / stderr / exit code / runtime via env vars.
//
// Env vars:
//
//	FAKE_FFPROBE_STDOUT       — written to stdout (raw bytes)
//	FAKE_FFPROBE_STDOUT_BYTES — number of zero bytes to dump (for oversize tests)
//	FAKE_FFPROBE_STDERR       — written to stderr
//	FAKE_FFPROBE_EXIT         — integer exit code (default 0)
//	FAKE_FFPROBE_SLEEP_MS     — sleep before exit (for deadline tests)
package main

import (
	"fmt"
	"os"
	"strconv"
	"time"
)

func main() {
	if s := os.Getenv("FAKE_FFPROBE_STDOUT"); s != "" {
		_, _ = fmt.Fprint(os.Stdout, s)
	}
	if n, _ := strconv.Atoi(os.Getenv("FAKE_FFPROBE_STDOUT_BYTES")); n > 0 {
		buf := make([]byte, 4096)
		for written := 0; written < n; {
			chunk := buf
			if rem := n - written; rem < len(chunk) {
				chunk = chunk[:rem]
			}
			k, err := os.Stdout.Write(chunk)
			if err != nil {
				break
			}
			written += k
		}
	}
	if s := os.Getenv("FAKE_FFPROBE_STDERR"); s != "" {
		_, _ = fmt.Fprint(os.Stderr, s)
	}
	if ms, _ := strconv.Atoi(os.Getenv("FAKE_FFPROBE_SLEEP_MS")); ms > 0 {
		time.Sleep(time.Duration(ms) * time.Millisecond)
	}
	exit, _ := strconv.Atoi(os.Getenv("FAKE_FFPROBE_EXIT"))
	os.Exit(exit)
}
