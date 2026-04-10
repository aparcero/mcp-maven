package observability

import (
	"context"
	"fmt"
	"log/slog"
	"os"
)

var (
	// defaultLogger is the global logger instance
	defaultLogger *slog.Logger
)

// Init initializes the global logger with the specified level.
func Init(level string) error {
	var lvl slog.Level
	switch level {
	case "debug":
		lvl = slog.LevelDebug
	case "info":
		lvl = slog.LevelInfo
	case "warn":
		lvl = slog.LevelWarn
	case "error":
		lvl = slog.LevelError
	default:
		return fmt.Errorf("invalid log level: %s", level)
	}

	opts := &slog.HandlerOptions{
		Level: lvl,
	}

	var handler slog.Handler
	// Use JSON for structured logging in production
	// Use text for development/readability
	if os.Getenv("LOG_FORMAT") == "json" {
		handler = slog.NewJSONHandler(os.Stderr, opts)
	} else {
		handler = slog.NewTextHandler(os.Stderr, opts)
	}

	defaultLogger = slog.New(handler)
	slog.SetDefault(defaultLogger)

	return nil
}

// L returns the default logger.
func L() *slog.Logger {
	if defaultLogger == nil {
		return slog.Default()
	}
	return defaultLogger
}

// With returns a logger with additional context.
func With(args ...any) *slog.Logger {
	return L().With(args...)
}

// Debug logs a debug message.
func Debug(msg string, args ...any) {
	L().Debug(msg, args...)
}

// Info logs an info message.
func Info(msg string, args ...any) {
	L().Info(msg, args...)
}

// Warn logs a warning message.
func Warn(msg string, args ...any) {
	L().Warn(msg, args...)
}

// Error logs an error message.
func Error(msg string, args ...any) {
	L().Error(msg, args...)
}

// Context returns a logger with context from the context.
func Context(ctx context.Context) *slog.Logger {
	return L()
}
