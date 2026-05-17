package media

import (
	"errors"
	"os"
	"path/filepath"
	"testing"
)

func TestResolveFFprobe_ReturnsPathWhenEnvSetAndFileExists(t *testing.T) {
	tmp := t.TempDir()
	fake := filepath.Join(tmp, "ffprobe")
	if err := os.WriteFile(fake, []byte("#!/bin/sh\n"), 0o755); err != nil {
		t.Fatal(err)
	}
	t.Setenv("STUDIO_FFPROBE_PATH", fake)
	got, err := ResolveFFprobe()
	if err != nil {
		t.Fatalf("unexpected err: %v", err)
	}
	if got != fake {
		t.Errorf("got %q, want %q", got, fake)
	}
}

func TestResolveFFprobe_ReturnsErrWhenEnvUnset(t *testing.T) {
	t.Setenv("STUDIO_FFPROBE_PATH", "")
	_, err := ResolveFFprobe()
	if !errors.Is(err, ErrFFprobeMissing) {
		t.Errorf("got %v, want ErrFFprobeMissing", err)
	}
}

func TestResolveFFprobe_ReturnsErrWhenFileMissing(t *testing.T) {
	t.Setenv("STUDIO_FFPROBE_PATH", "/definitely/not/a/real/path/ffprobe")
	_, err := ResolveFFprobe()
	if !errors.Is(err, ErrFFprobeMissing) {
		t.Errorf("got %v, want ErrFFprobeMissing", err)
	}
}

func TestResolveFFprobe_DoesNotConsultPATH(t *testing.T) {
	t.Setenv("STUDIO_FFPROBE_PATH", "")
	_, err := ResolveFFprobe()
	if !errors.Is(err, ErrFFprobeMissing) {
		t.Errorf("got %v, want ErrFFprobeMissing", err)
	}
}
