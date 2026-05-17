package media

import "github.com/studio-sound/studio/sidecar/internal/ipc/generated"

// MediaProbeResult is the canonical wire-stable shape for a media probe.
// It mirrors generated.ProbeResult but types Audio as *generated.Audio
// so callers get compile-time safety (the generated field is interface{}).
type MediaProbeResult struct {
	ID              string                  `json:"id"`
	Path            string                  `json:"path"`
	Filename        string                  `json:"filename"`
	SizeBytes       int                     `json:"sizeBytes"`
	Container       generated.Container     `json:"container"`
	DurationSeconds *float64                `json:"durationSeconds,omitempty"`
	Video           *generated.Video        `json:"video,omitempty"`
	Audio           *generated.Audio        `json:"audio"`
	Compatibility   generated.Compatibility `json:"compatibility"`
}

// Type aliases so the rest of the package can use short names.
type AudioStream = generated.Audio
type AudioTrack = generated.AudioTrack
type VideoStream = generated.Video
type Container = generated.Container
type Compatibility = generated.Compatibility
