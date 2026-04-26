package jobs

import (
	"bytes"
	"log/slog"
)

func newInfoBufferLogger(buf *bytes.Buffer) *slog.Logger {
	return slog.New(slog.NewJSONHandler(buf, &slog.HandlerOptions{Level: slog.LevelInfo}))
}
