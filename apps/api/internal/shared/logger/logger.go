package logger

import (
	"log/slog"
	"os"
)

// New returns a JSON slog logger. LOG_LEVEL=debug enables debug output;
// anything else logs info and above.
func New() *slog.Logger {
	level := slog.LevelInfo
	if os.Getenv("LOG_LEVEL") == "debug" {
		level = slog.LevelDebug
	}
	return slog.New(slog.NewJSONHandler(os.Stdout, &slog.HandlerOptions{Level: level}))
}
