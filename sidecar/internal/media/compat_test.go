package media

import (
	"strings"
	"testing"
)

func eval(t *testing.T, fixture string) (*MediaProbeResult, Verdict) {
	t.Helper()
	out, _ := Parse(readFixture(t, fixture))
	r, _ := Normalize(out, "/tmp/x", 1)
	v := Evaluate(r)
	return r, v
}

func TestEvaluate_H264AACIsSupported(t *testing.T) {
	_, v := eval(t, "h264_aac_stereo.json")
	if !v.Supported {
		t.Errorf("expected supported, got %+v", v)
	}
	if len(v.Issues) != 0 {
		t.Errorf("unexpected issues: %v", v.Issues)
	}
}

func TestEvaluate_VP9OpusIsSupported(t *testing.T) {
	_, v := eval(t, "vp9_opus.json")
	if !v.Supported {
		t.Errorf("expected supported, got %+v", v)
	}
}

func TestEvaluate_UnsupportedContainerSetsIssue(t *testing.T) {
	_, v := eval(t, "unsupported_container.json")
	if v.Supported {
		t.Error("expected unsupported")
	}
	var found bool
	for _, s := range v.Issues {
		if strings.Contains(s, "container") {
			found = true
		}
	}
	if !found {
		t.Errorf("no container issue in: %v", v.Issues)
	}
}

func TestEvaluate_UnsupportedCodecSetsIssue(t *testing.T) {
	_, v := eval(t, "unsupported_codec.json")
	if v.Supported {
		t.Error("expected unsupported")
	}
	var found bool
	for _, s := range v.Issues {
		if strings.Contains(s, "codec") {
			found = true
		}
	}
	if !found {
		t.Errorf("no codec issue in: %v", v.Issues)
	}
}

func TestEvaluate_NoAudioStreamSetsIssue(t *testing.T) {
	_, v := eval(t, "no_audio.json")
	if v.Supported {
		t.Error("expected unsupported")
	}
	var found bool
	for _, s := range v.Issues {
		if strings.Contains(s, "audio") {
			found = true
		}
	}
	if !found {
		t.Errorf("no audio issue in: %v", v.Issues)
	}
}

func TestEvaluate_MissingDurationIsWarningNotIssue(t *testing.T) {
	_, v := eval(t, "missing_duration.json")
	if !v.Supported {
		t.Errorf("missing duration must not block support")
	}
	var found bool
	for _, s := range v.Warnings {
		if strings.Contains(s, "duration") {
			found = true
		}
	}
	if !found {
		t.Errorf("missing duration should produce a warning: %v", v.Warnings)
	}
}

func TestEvaluate_MultitrackAddsInformationalWarning(t *testing.T) {
	_, v := eval(t, "aac_multitrack.json")
	if !v.Supported {
		t.Error("multitrack must remain supported")
	}
	var found bool
	for _, s := range v.Warnings {
		if strings.Contains(s, "track") {
			found = true
		}
	}
	if !found {
		t.Errorf("multitrack should produce a warning: %v", v.Warnings)
	}
}

func TestEvaluate_AllowListMatrix(t *testing.T) {
	cases := []struct {
		container string
		codec     string
	}{
		{"mov,mp4,m4a,3gp,3g2,mj2", "aac"},
		{"mov,mp4,m4a,3gp,3g2,mj2", "mp3"},
		{"matroska,webm", "opus"},
		{"matroska,webm", "vorbis"},
		{"matroska,webm", "flac"},
		{"mov,mp4,m4a,3gp,3g2,mj2", "pcm_s16le"},
		{"mov,mp4,m4a,3gp,3g2,mj2", "pcm_s24le"},
		{"mov,mp4,m4a,3gp,3g2,mj2", "pcm_f32le"},
	}
	for _, c := range cases {
		tracks := []AudioTrack{{Index: 0, Codec: c.codec, Channels: 2, SampleRate: 48000, IsDefault: true}}
		r := &MediaProbeResult{
			Container: Container{Format: c.container, LongName: c.container},
			Audio: &AudioStream{
				Codec: c.codec, Channels: 2, SampleRate: 48000,
				TrackIndex: 0, TrackCount: 1, Tracks: tracks,
			},
		}
		v := Evaluate(r)
		if !v.Supported {
			t.Errorf("%s/%s should be supported, got %+v", c.container, c.codec, v)
		}
	}
}
