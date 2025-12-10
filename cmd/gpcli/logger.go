package main

import (
	"bytes"
	"context"
	"fmt"
	"io"
	"log/slog"
	"os"
	"strings"
)

var logger *slog.Logger
var currentLogLevel slog.Level
var logFormat string

// humanHandler is a slog.Handler that outputs human-readable logs without timestamps
type humanHandler struct {
	out   io.Writer
	level slog.Level
}

func (h *humanHandler) Enabled(_ context.Context, level slog.Level) bool {
	return level >= h.level
}

func (h *humanHandler) Handle(_ context.Context, r slog.Record) error {
	var buf bytes.Buffer
	buf.WriteString(r.Message)
	r.Attrs(func(a slog.Attr) bool {
		buf.WriteString(" ")
		buf.WriteString(a.Key)
		buf.WriteString("=")
		buf.WriteString(fmt.Sprintf("%v", a.Value.Any()))
		return true
	})
	buf.WriteString("\n")
	_, err := h.out.Write(buf.Bytes())
	return err
}

func (h *humanHandler) WithAttrs(attrs []slog.Attr) slog.Handler { return h }
func (h *humanHandler) WithGroup(name string) slog.Handler       { return h }

// parseLogLevel converts a string log level to slog.Level
func parseLogLevel(level string) slog.Level {
	switch strings.ToLower(level) {
	case "debug":
		return slog.LevelDebug
	case "info":
		return slog.LevelInfo
	case "warn", "warning":
		return slog.LevelWarn
	case "error":
		return slog.LevelError
	default:
		return slog.LevelInfo
	}
}

// initLogger initializes the global logger with the specified level and format
func initLogger(level slog.Level) {
	opts := &slog.HandlerOptions{Level: level}
	var handler slog.Handler
	switch logFormat {
	case "slog":
		handler = slog.NewTextHandler(os.Stdout, opts)
	case "json":
		handler = slog.NewJSONHandler(os.Stdout, opts)
	default: // "human"
		handler = &humanHandler{out: os.Stdout, level: level}
	}
	logger = slog.New(handler)
	slog.SetDefault(logger)
}

// initQuietLogger initializes a logger that only shows errors
func initQuietLogger() {
	currentLogLevel = slog.LevelError
	initLogger(slog.LevelError)
}
