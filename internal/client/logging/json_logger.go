package logging

import (
	"log/slog"
	"os"
)

func NewJSONLogger(logLevel *slog.LevelVar) *slog.Logger {
	return slog.New(
		slog.NewJSONHandler(
			os.Stdout, &slog.HandlerOptions{Level: logLevel},
		),
	)
}
