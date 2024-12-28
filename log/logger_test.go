package log

import (
	"bytes"
	"encoding/json"
	"io"
	"log/slog"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestNewLogger(t *testing.T) {
	tests := []struct {
		name string
		opts []LoggerOption
	}{
		{
			name: "default logger",
			opts: []LoggerOption{},
		},
		{
			name: "development logger",
			opts: []LoggerOption{WithDevelopment()},
		},
		{
			name: "nop logger",
			opts: []LoggerOption{WithNop()},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			l := NewLogger()
			require.NotNil(t, l)
		})
	}
}

func TestLogger(t *testing.T) {
	var buf bytes.Buffer
	l := NewLogger(
		WithLevel(slog.LevelDebug),
		func(opts *loggerOptions) {
			opts.handlerFunc = func(_ io.Writer, opts *slog.HandlerOptions) slog.Handler {
				return slog.NewJSONHandler(&buf, opts)
			}
		},
	)

	wantMsg, wantKey1, wantVal1, wantKey2, wantVal2 := "lorem ipsum", "key1", "val1", "key2", "val2"
	l = l.With(wantKey1, wantVal1)

	tests := []struct {
		name      string
		wantLevel slog.Level
		logFunc   func(msg string, args ...any)
	}{
		{
			name:      "info level",
			wantLevel: slog.LevelInfo,
			logFunc:   l.Info,
		},
		{
			name:      "debug level",
			wantLevel: slog.LevelDebug,
			logFunc:   l.Debug,
		},
		{
			name:      "warn level",
			wantLevel: slog.LevelWarn,
			logFunc:   l.Warn,
		},
		{
			name:      "error level",
			wantLevel: slog.LevelError,
			logFunc:   l.Error,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			buf.Reset()
			tt.logFunc(wantMsg, wantKey2, wantVal2)

			var gotLog testLog
			err := json.Unmarshal(buf.Bytes(), &gotLog)
			require.NoError(t, err)

			assert.Equal(t, tt.wantLevel.String(), gotLog.Level)
			assert.Equal(t, wantMsg, gotLog.Msg)
			assert.Equal(t, wantVal1, gotLog.Key1)
			assert.Equal(t, wantVal2, gotLog.Key2)
			_, err = time.Parse(time.RFC3339, gotLog.Time)
			assert.NoError(t, err)
		})
	}
}

func TestWithNop(t *testing.T) {
	l := NewLogger(WithNop())

	require.NotNil(t, l)

	// Attempt to log
	l.Info("this should not log")
	l.Debug("this should also not log")
	l.Error("errors should not appear")

	// No assertions required as NOP logger discards logs
}

func TestParseLevel(t *testing.T) {
	tests := []struct {
		name      string
		input     string
		wantLevel slog.Level
		wantOK    bool
	}{
		{
			name:      "debug level",
			input:     "debug",
			wantLevel: slog.LevelDebug,
			wantOK:    true,
		},
		{
			name:      "info level",
			input:     "info",
			wantLevel: slog.LevelInfo,
			wantOK:    true,
		},
		{
			name:      "warn level",
			input:     "warn",
			wantLevel: slog.LevelWarn,
			wantOK:    true,
		},
		{
			name:      "error level",
			input:     "error",
			wantLevel: slog.LevelError,
			wantOK:    true,
		},
		{
			name:      "invalid level",
			input:     "invalid",
			wantLevel: -1,
			wantOK:    false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			level, ok := ParseLevel(tt.input)
			assert.Equal(t, tt.wantLevel, level)
			assert.Equal(t, tt.wantOK, ok)
		})
	}
}

type testLog struct {
	Time  string `json:"time"`
	Level string `json:"level"`
	Msg   string `json:"msg"`
	Key1  string `json:"key1"`
	Key2  string `json:"key2"`
}
