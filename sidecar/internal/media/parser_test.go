package media

import (
	"os"
	"path/filepath"
	"testing"
)

func readFixture(t *testing.T, name string) []byte {
	t.Helper()
	b, err := os.ReadFile(filepath.Join("testdata", name))
	if err != nil {
		t.Fatal(err)
	}
	return b
}

func TestParse_H264AAC(t *testing.T) {
	out, err := Parse(readFixture(t, "h264_aac_stereo.json"))
	if err != nil {
		t.Fatal(err)
	}
	if out.Format.FormatName == "" {
		t.Error("missing format_name")
	}
	if len(out.Streams) < 2 {
		t.Errorf("got %d streams, want >=2", len(out.Streams))
	}
}

func TestParse_AcceptsUnknownFields(t *testing.T) {
	_, err := Parse([]byte(`{"format":{"format_name":"x","new_field":"x"},"streams":[]}`))
	if err != nil {
		t.Errorf("unexpected err on unknown field: %v", err)
	}
}

func TestParse_ReturnsErrOnInvalidJSON(t *testing.T) {
	_, err := Parse([]byte(`not json`))
	if err == nil {
		t.Error("expected err on invalid JSON")
	}
}
