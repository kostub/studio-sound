package ipc

import "testing"

func TestMediaProbeCodeConstants(t *testing.T) {
	cases := map[string]string{
		CodeFileNotFound:   "FILE_NOT_FOUND",
		CodeAccessDenied:   "ACCESS_DENIED",
		CodeCorruptMedia:   "CORRUPT_MEDIA",
		CodeFFprobeFailure: "FFPROBE_FAILURE",
		CodeFFprobeMissing: "FFPROBE_MISSING",
	}
	for got, want := range cases {
		if got != want {
			t.Errorf("constant value = %q, want %q", got, want)
		}
	}
}
