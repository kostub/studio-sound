package media

import (
	"crypto/rand"
	"fmt"
	"path/filepath"
	"strconv"
	"strings"

	"github.com/studio-sound/studio/sidecar/internal/ipc/generated"
)

// Normalize converts a parsed ffprobe output into the canonical MediaProbeResult.
// Audio is nil when no audio stream is present (JSON null when marshalled).
// Compatibility is zero-valued; Evaluate fills it in.
func Normalize(in *ffprobeOutput, path string, sizeBytes int) (*MediaProbeResult, error) {
	r := &MediaProbeResult{
		ID:        newUUID(),
		Path:      path,
		Filename:  filepath.Base(path),
		SizeBytes: sizeBytes,
		Container: Container{
			Format:   in.Format.FormatName,
			LongName: in.Format.FormatLongName,
		},
	}

	if dur, err := strconv.ParseFloat(strings.TrimSpace(in.Format.Duration), 64); err == nil && dur > 0 {
		r.DurationSeconds = &dur
	}

	var videoStream *ffprobeStream
	var audioStreams []ffprobeStream
	for i := range in.Streams {
		s := &in.Streams[i]
		switch s.CodecType {
		case "video":
			if videoStream == nil {
				videoStream = s
			}
		case "audio":
			audioStreams = append(audioStreams, *s)
		}
	}

	if videoStream != nil {
		r.Video = &VideoStream{
			Codec:       videoStream.CodecName,
			Width:       videoStream.Width,
			Height:      videoStream.Height,
			Fps:         parseFps(videoStream.RFrameRate),
			Bitrate:     optInt(videoStream.BitRate),
			PixelFormat: optString(videoStream.PixFmt),
		}
	}

	if len(audioStreams) > 0 {
		defIdx := defaultAudioIndex(audioStreams)
		chosen := audioStreams[defIdx]
		tracks := make([]AudioTrack, len(audioStreams))
		for i, s := range audioStreams {
			tracks[i] = generated.AudioTrack{
				Index:      s.Index,
				Codec:      s.CodecName,
				Channels:   s.Channels,
				SampleRate: parseInt(s.SampleRate),
				Bitrate:    optInt(s.BitRate),
				Layout:     optString(s.ChannelLayout),
				Title:      optString(s.Tags["title"]),
				Language:   optString(s.Tags["language"]),
				IsDefault:  s.Disposition["default"] == 1,
			}
		}
		r.Audio = &AudioStream{
			Codec:      chosen.CodecName,
			Channels:   chosen.Channels,
			SampleRate: parseInt(chosen.SampleRate),
			Bitrate:    optInt(chosen.BitRate),
			Layout:     optString(chosen.ChannelLayout),
			TrackIndex: chosen.Index,
			TrackCount: len(audioStreams),
			Tracks:     tracks,
		}
	}

	return r, nil
}

func defaultAudioIndex(streams []ffprobeStream) int {
	for i, s := range streams {
		if s.Disposition["default"] == 1 {
			return i
		}
	}
	lowest := 0
	for i, s := range streams {
		if s.Index < streams[lowest].Index {
			lowest = i
		}
	}
	return lowest
}

func parseFps(rfr string) float64 {
	parts := strings.SplitN(rfr, "/", 2)
	if len(parts) != 2 {
		return 0
	}
	num, err1 := strconv.ParseFloat(parts[0], 64)
	den, err2 := strconv.ParseFloat(parts[1], 64)
	if err1 != nil || err2 != nil || den == 0 {
		return 0
	}
	return num / den
}

func parseInt(s string) int {
	n, _ := strconv.Atoi(strings.TrimSpace(s))
	return n
}

func optInt(s string) *int {
	s = strings.TrimSpace(s)
	if s == "" {
		return nil
	}
	n, err := strconv.Atoi(s)
	if err != nil {
		return nil
	}
	return &n
}

func optString(s string) *string {
	if s == "" {
		return nil
	}
	return &s
}

func newUUID() string {
	var b [16]byte
	_, _ = rand.Read(b[:])
	b[6] = (b[6] & 0x0f) | 0x40 // version 4
	b[8] = (b[8] & 0x3f) | 0x80 // variant bits
	return fmt.Sprintf("%08x-%04x-%04x-%04x-%012x",
		b[0:4], b[4:6], b[6:8], b[8:10], b[10:16])
}
