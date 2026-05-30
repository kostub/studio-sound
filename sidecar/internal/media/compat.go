package media

import (
	"strconv"
	"strings"
)

type Verdict struct {
	Supported bool
	Issues    []string
	Warnings  []string
}

var supportedContainerTokens = []string{
	"mov", "mp4", "m4a", "3gp", "3g2", "mj2",
	"matroska", "webm",
}

var supportedAudioCodecs = map[string]bool{
	"aac": true, "opus": true,
	"pcm_s16le": true, "pcm_s24le": true, "pcm_f32le": true,
	"mp3": true, "vorbis": true, "flac": true,
}

func Evaluate(r *MediaProbeResult) Verdict {
	v := Verdict{Supported: true, Issues: []string{}, Warnings: []string{}}

	if !containerSupported(r.Container.Format) {
		v.Supported = false
		v.Issues = append(v.Issues, "Unsupported container: "+r.Container.Format)
	}
	if r.Audio == nil {
		v.Supported = false
		v.Issues = append(v.Issues, "No audio stream detected in the file")
	} else if !supportedAudioCodecs[strings.ToLower(r.Audio.Codec)] {
		v.Supported = false
		v.Issues = append(v.Issues, "Unsupported audio codec: "+r.Audio.Codec)
	}

	if r.DurationSeconds == nil {
		v.Warnings = append(v.Warnings, "File duration could not be determined")
	}
	if r.Audio != nil && r.Audio.TrackCount > 1 {
		v.Warnings = append(v.Warnings,
			"Multiple audio tracks detected (selected track #"+strconv.Itoa(r.Audio.TrackIndex)+")")
	}
	return v
}

// ffprobe reports format_name as a comma-separated list of format families
// (e.g. "mov,mp4,m4a,3gp,3g2,mj2"). Split and exact-match per token so we
// don't false-positive on substrings like "movie" containing "mov".
func containerSupported(formatName string) bool {
	for _, f := range strings.Split(strings.ToLower(formatName), ",") {
		f = strings.TrimSpace(f)
		for _, tok := range supportedContainerTokens {
			if f == tok {
				return true
			}
		}
	}
	return false
}
