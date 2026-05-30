package media

import (
	"context"
	"errors"
	"os"
	"testing"

	"github.com/studio-sound/studio/sidecar/internal/ipc"
)

func TestMapRunError_FileNotFound(t *testing.T) {
	e := MapRunError(os.ErrNotExist, "")
	if e.Code != ipc.CodeFileNotFound {
		t.Errorf("code = %s, want %s", e.Code, ipc.CodeFileNotFound)
	}
}

func TestMapRunError_AccessDenied(t *testing.T) {
	e := MapRunError(os.ErrPermission, "")
	if e.Code != ipc.CodeAccessDenied {
		t.Errorf("code = %s, want %s", e.Code, ipc.CodeAccessDenied)
	}
}

func TestMapRunError_DeadlineExceeded(t *testing.T) {
	e := MapRunError(context.DeadlineExceeded, "")
	if e.Code != ipc.CodeFFprobeFailure {
		t.Errorf("code = %s, want %s", e.Code, ipc.CodeFFprobeFailure)
	}
	if e.Message == "" {
		t.Error("message empty")
	}
}

func TestMapRunError_Canceled(t *testing.T) {
	e := MapRunError(context.Canceled, "")
	if e.Code != ipc.CodeFFprobeFailure {
		t.Errorf("code = %s, want %s", e.Code, ipc.CodeFFprobeFailure)
	}
	if e.Message != "probe was cancelled" {
		t.Errorf("message = %q, want %q", e.Message, "probe was cancelled")
	}
}

func TestMapRunError_CorruptMediaFromStderrMoov(t *testing.T) {
	e := MapRunError(errors.New("exit 1"), "moov atom not found")
	if e.Code != ipc.CodeCorruptMedia {
		t.Errorf("code = %s, want %s", e.Code, ipc.CodeCorruptMedia)
	}
}

func TestMapRunError_CorruptMediaFromStderrInvalid(t *testing.T) {
	e := MapRunError(errors.New("exit 1"), "Invalid data found when processing input")
	if e.Code != ipc.CodeCorruptMedia {
		t.Errorf("code = %s, want %s", e.Code, ipc.CodeCorruptMedia)
	}
}

func TestMapRunError_AccessDeniedFromStderr(t *testing.T) {
	e := MapRunError(errors.New("exit 1"), "Permission denied")
	if e.Code != ipc.CodeAccessDenied {
		t.Errorf("code = %s, want %s", e.Code, ipc.CodeAccessDenied)
	}
}

func TestMapRunError_DefaultIsFFprobeFailure(t *testing.T) {
	e := MapRunError(errors.New("oh no"), "weird unrelated stderr")
	if e.Code != ipc.CodeFFprobeFailure {
		t.Errorf("code = %s, want %s", e.Code, ipc.CodeFFprobeFailure)
	}
}

func TestMapParseError_AlwaysCorrupt(t *testing.T) {
	e := MapParseError(errors.New("bad json"))
	if e.Code != ipc.CodeCorruptMedia {
		t.Errorf("code = %s, want %s", e.Code, ipc.CodeCorruptMedia)
	}
}
