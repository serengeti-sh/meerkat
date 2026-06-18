// Package logger provides structured logging using zerolog.
package logger

import (
	"context"
	"os"

	"github.com/rs/zerolog"
)

type contextKey struct{}

var loggerKey = &contextKey{}

// Config holds logger configuration.
type Config struct {
	Level  string // debug, info, warn, error
	Format string // json, console
}

// New creates a configured zerolog logger.
func New(cfg Config) zerolog.Logger {
	level := zerolog.InfoLevel
	if cfg.Level != "" {
		if l, err := zerolog.ParseLevel(cfg.Level); err == nil {
			level = l
		}
	}

	var log zerolog.Logger
	if cfg.Format == "console" {
		log = zerolog.New(zerolog.ConsoleWriter{Out: os.Stderr})
	} else {
		log = zerolog.New(os.Stderr)
	}

	return log.Level(level).With().Timestamp().Logger()
}

// WithContext injects logger into context.
func WithContext(ctx context.Context, log zerolog.Logger) context.Context {
	return context.WithValue(ctx, loggerKey, &log)
}

// Ctx extracts logger from context, falls back to a default logger.
func Ctx(ctx context.Context) *zerolog.Logger {
	if log, ok := ctx.Value(loggerKey).(*zerolog.Logger); ok {
		return log
	}
	l := zerolog.New(os.Stderr).With().Timestamp().Logger()
	return &l
}
