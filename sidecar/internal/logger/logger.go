package logger

import (
	"io"
	"log/slog"
	"os"

	lumberjack "gopkg.in/natefinch/lumberjack.v2"
)

// New returns a JSON slog.Logger. If logPath is empty, logs are written to
// stderr. Otherwise, logs are written to a lumberjack rotating file at logPath
// (10 MiB max size, 3 backups, compressed).
func New(logPath string) *slog.Logger {
	var w io.Writer
	if logPath == "" {
		w = os.Stderr
	} else {
		w = &lumberjack.Logger{
			Filename:   logPath,
			MaxSize:    10, // megabytes
			MaxBackups: 3,
			Compress:   true,
		}
	}
	return NewWithWriter(w)
}

// NewWithWriter returns a JSON slog.Logger writing to w. Useful for testing.
func NewWithWriter(w io.Writer) *slog.Logger {
	return slog.New(slog.NewJSONHandler(w, nil))
}
