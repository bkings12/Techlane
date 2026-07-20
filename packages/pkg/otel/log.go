package otel

import (
	"log/slog"
	"os"
)

// SetupJSONLogger configures structured JSON logging (OpenTelemetry wiring can wrap this later).
func SetupJSONLogger() *slog.Logger {
	handler := slog.NewJSONHandler(os.Stdout, &slog.HandlerOptions{Level: slog.LevelInfo})
	logger := slog.New(handler)
	slog.SetDefault(logger)
	return logger
}
