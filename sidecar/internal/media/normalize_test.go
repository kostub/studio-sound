package media

import (
	"testing"
)

func TestNormalize_PopulatesVideoAndAudio(t *testing.T) {
	out, err := Parse(readFixture(t, "h264_aac_stereo.json"))
	if err != nil {
		t.Fatal(err)
	}
	r := Normalize(out, "/tmp/x.mp4", 12345)
	if r.Filename != "x.mp4" {
		t.Errorf("filename = %q", r.Filename)
	}
	if r.SizeBytes != 12345 {
		t.Errorf("sizeBytes = %d", r.SizeBytes)
	}
	if r.Video == nil || r.Video.Codec != "h264" {
		t.Errorf("video = %+v", r.Video)
	}
	if r.Audio == nil || r.Audio.Codec != "aac" || r.Audio.Channels != 2 {
		t.Errorf("audio = %+v", r.Audio)
	}
	if r.DurationSeconds == nil || *r.DurationSeconds <= 0 {
		t.Errorf("durationSeconds = %v", r.DurationSeconds)
	}
}

func TestNormalize_AudioIsNilWhenNoAudioStream(t *testing.T) {
	out, err := Parse(readFixture(t, "no_audio.json"))
	if err != nil {
		t.Fatal(err)
	}
	r := Normalize(out, "/tmp/x.mp4", 1)
	if r.Audio != nil {
		t.Errorf("audio should be nil for no-audio file, got %+v", r.Audio)
	}
}

func TestNormalize_DefaultTrackSelectionByDispositionFlag(t *testing.T) {
	out, _ := Parse(readFixture(t, "aac_multitrack.json"))
	r := Normalize(out, "/tmp/x.mov", 1)
	if r.Audio == nil {
		t.Fatal("audio nil")
	}
	// index 1 is default-flagged in the fixture
	if r.Audio.TrackIndex != 1 {
		t.Errorf("trackIndex = %d, want 1", r.Audio.TrackIndex)
	}
	if r.Audio.TrackCount != 2 {
		t.Errorf("trackCount = %d, want 2", r.Audio.TrackCount)
	}
	if len(r.Audio.Tracks) != 2 {
		t.Errorf("tracks len = %d", len(r.Audio.Tracks))
	}
}

func TestNormalize_DurationOmittedWhenMissing(t *testing.T) {
	out, _ := Parse(readFixture(t, "missing_duration.json"))
	r := Normalize(out, "/tmp/x.mp4", 1)
	if r.DurationSeconds != nil {
		t.Errorf("durationSeconds should be nil, got %v", *r.DurationSeconds)
	}
}

func TestNormalize_FPSFromRFrameRate(t *testing.T) {
	out := &ffprobeOutput{
		Format:  ffprobeFormat{FormatName: "x"},
		Streams: []ffprobeStream{{CodecType: "video", CodecName: "h264", Width: 1920, Height: 1080, RFrameRate: "30000/1001"}},
	}
	r := Normalize(out, "/tmp/x.mp4", 1)
	if r.Video == nil {
		t.Fatal("video nil")
	}
	if r.Video.Fps < 29.96 || r.Video.Fps > 29.98 {
		t.Errorf("fps = %v, want ~29.97", r.Video.Fps)
	}
}
