package health

import (
	"bytes"
	"errors"
	"testing"
)

type failingWriter struct{}

func (failingWriter) Write(_ []byte) (int, error) {
	return 0, errors.New("write failed")
}

func TestWriteHealthResponse(t *testing.T) {
	var stdout bytes.Buffer

	if err := Write(&stdout); err != nil {
		t.Fatalf("expected no error, got %v", err)
	}

	const want = `{"status":"ok","version":"0.0.1"}`
	if stdout.String() != want {
		t.Fatalf("unexpected health response\nwant: %s\n got: %s", want, stdout.String())
	}
}

func TestWriteReturnsWriterError(t *testing.T) {
	if err := Write(failingWriter{}); err == nil {
		t.Fatal("expected writer error")
	}
}
