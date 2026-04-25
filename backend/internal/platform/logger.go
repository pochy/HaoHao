package platform

import (
	"io"
	"log/slog"
	"strings"
)

func NewLogger(level, format string, out io.Writer) *slog.Logger {
	var slogLevel slog.Level
	switch strings.ToLower(strings.TrimSpace(level)) {
	case "debug":
		slogLevel = slog.LevelDebug
	case "warn":
		slogLevel = slog.LevelWarn
	case "error":
		slogLevel = slog.LevelError
	default:
		slogLevel = slog.LevelInfo
	}

	options := &slog.HandlerOptions{Level: slogLevel}
	if strings.EqualFold(format, "text") {
		return slog.New(slog.NewTextHandler(out, options))
	}

	return slog.New(slog.NewJSONHandler(out, options))
}
