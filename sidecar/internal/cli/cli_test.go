package cli

import (
	"bytes"
	"errors"
	"strings"
	"testing"
)

type failingWriter struct{}

func (failingWriter) Write(_ []byte) (int, error) {
	return 0, errors.New("write failed")
}

func TestRunHealth(t *testing.T) {
	var stdout bytes.Buffer
	var stderr bytes.Buffer

	code := Run([]string{"health"}, &bytes.Buffer{}, &stdout, &stderr)

	if code != 0 {
		t.Fatalf("expected exit code 0, got %d", code)
	}

	const want = `{"status":"ok","version":"0.0.1"}`
	if stdout.String() != want {
		t.Fatalf("unexpected stdout\nwant: %s\n got: %s", want, stdout.String())
	}

	if stderr.Len() != 0 {
		t.Fatalf("expected empty stderr, got %q", stderr.String())
	}
}

func TestRunUnknownCommand(t *testing.T) {
	var stdout bytes.Buffer
	var stderr bytes.Buffer

	code := Run([]string{"nope"}, &bytes.Buffer{}, &stdout, &stderr)

	if code != 2 {
		t.Fatalf("expected exit code 2, got %d", code)
	}

	if stdout.Len() != 0 {
		t.Fatalf("expected empty stdout, got %q", stdout.String())
	}

	if !strings.Contains(stderr.String(), "unknown command: nope") {
		t.Fatalf("expected useful stderr, got %q", stderr.String())
	}
}

func TestRunMissingCommand(t *testing.T) {
	var stdout bytes.Buffer
	var stderr bytes.Buffer

	code := Run(nil, &bytes.Buffer{}, &stdout, &stderr)

	if code != 2 {
		t.Fatalf("expected exit code 2, got %d", code)
	}

	if stdout.Len() != 0 {
		t.Fatalf("expected empty stdout, got %q", stdout.String())
	}

	if !strings.Contains(stderr.String(), "missing command") {
		t.Fatalf("expected useful stderr, got %q", stderr.String())
	}
}

func TestRunHealthWriteFailure(t *testing.T) {
	var stderr bytes.Buffer

	code := Run([]string{"health"}, &bytes.Buffer{}, failingWriter{}, &stderr)

	if code != 1 {
		t.Fatalf("expected exit code 1, got %d", code)
	}

	if !strings.Contains(stderr.String(), "failed to encode health response") {
		t.Fatalf("expected encode failure stderr, got %q", stderr.String())
	}
}
