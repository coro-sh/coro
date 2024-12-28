package log

import (
	"context"
	"io"
	"log/slog"
	"os"

	"github.com/lmittmann/tint"
)

// LoggerOption configures a Logger.
type LoggerOption func(opts *loggerOptions)

// WithLevel sets the logging level for the Logger.
func WithLevel(level slog.Level) LoggerOption {
	return func(opts *loggerOptions) {
		opts.level = level
	}
}

// WithDevelopment configures the Logger for development mode with human-readable
// output.
func WithDevelopment() LoggerOption {
	return func(opts *loggerOptions) {
		opts.handlerFunc = func(w io.Writer, opts *slog.HandlerOptions) slog.Handler {
			return tint.NewHandler(w, &tint.Options{Level: opts.Level})
		}
	}
}

// WithNop configures a no-operation Logger that discards all log messages.
func WithNop() LoggerOption {
	return func(opts *loggerOptions) {
		opts.handlerFunc = func(_ io.Writer, hopts *slog.HandlerOptions) slog.Handler {
			return slog.NewJSONHandler(io.Discard, hopts)
		}
	}
}

type loggerOptions struct {
	level       slog.Level
	handlerFunc func(w io.Writer, opts *slog.HandlerOptions) slog.Handler
}

// Logger defines the interface for structured logging.
type Logger interface {
	Log(ctx context.Context, level slog.Level, msg string, args ...any)
	Info(msg string, args ...any)
	Debug(msg string, args ...any)
	Warn(msg string, args ...any)
	Error(msg string, args ...any)
	With(args ...any) Logger
}

// NewLogger creates a new Logger instance with the specified options.
func NewLogger(opts ...LoggerOption) Logger {
	options := loggerOptions{
		level: slog.LevelInfo,
		handlerFunc: func(w io.Writer, opts *slog.HandlerOptions) slog.Handler {
			return slog.NewJSONHandler(w, opts)
		},
	}

	for _, opt := range opts {
		opt(&options)
	}

	return &logger{
		Logger: slog.New(options.handlerFunc(os.Stdout, &slog.HandlerOptions{
			Level: options.level,
		})),
	}
}

type logger struct {
	*slog.Logger
}

// With returns a new Logger with the specified arguments.
func (l *logger) With(args ...any) Logger {
	return &logger{l.Logger.With(args...)}
}

func ParseLevel(level string) (slog.Level, bool) {
	switch level {
	case "debug":
		return slog.LevelDebug, true
	case "info":
		return slog.LevelInfo, true
	case "warn":
		return slog.LevelWarn, true
	case "error":
		return slog.LevelError, true
	}
	return -1, false
}
