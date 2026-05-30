package media

import (
	"encoding/json"
	"fmt"
)

type ffprobeOutput struct {
	Format  ffprobeFormat   `json:"format"`
	Streams []ffprobeStream `json:"streams"`
}

type ffprobeFormat struct {
	FormatName     string `json:"format_name"`
	FormatLongName string `json:"format_long_name"`
	Duration       string `json:"duration"`
	Size           string `json:"size"`
	BitRate        string `json:"bit_rate"`
}

type ffprobeStream struct {
	Index         int               `json:"index"`
	CodecType     string            `json:"codec_type"`
	CodecName     string            `json:"codec_name"`
	Width         int               `json:"width,omitempty"`
	Height        int               `json:"height,omitempty"`
	PixFmt        string            `json:"pix_fmt,omitempty"`
	RFrameRate    string            `json:"r_frame_rate,omitempty"`
	Channels      int               `json:"channels,omitempty"`
	SampleRate    string            `json:"sample_rate,omitempty"`
	ChannelLayout string            `json:"channel_layout,omitempty"`
	BitRate       string            `json:"bit_rate,omitempty"`
	Disposition   map[string]int    `json:"disposition,omitempty"`
	Tags          map[string]string `json:"tags,omitempty"`
}

func Parse(stdout []byte) (*ffprobeOutput, error) {
	var out ffprobeOutput
	if err := json.Unmarshal(stdout, &out); err != nil {
		return nil, fmt.Errorf("parse ffprobe json: %w", err)
	}
	return &out, nil
}
