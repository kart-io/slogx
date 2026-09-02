package slogx

import (
	"context"
	"io"
	"log/slog"
	"os"
	"time"
)

// This file is the calling surface. Init/New build the handler chain; the
// rest of a program should not need to import log/slog to write a line, build
// an attribute, or hand a logger to a component. Everything here is a thin
// alias or delegate over the default slog logger, so a *Logger returned by
// Init and the package-level functions always write to the same place.

// Type aliases, so a signature that takes a Handler or an Attr reads the same
// whether it was written against slog or slogx.
type (
	Attr           = slog.Attr
	Value          = slog.Value
	Level          = slog.Level
	Handler        = slog.Handler
	HandlerOptions = slog.HandlerOptions
	Record         = slog.Record
)

const (
	LevelDebug = slog.LevelDebug
	LevelInfo  = slog.LevelInfo
	LevelWarn  = slog.LevelWarn
	LevelError = slog.LevelError
)

// Wrap adopts an existing slog logger as a *Logger. It carries no shutdown
// hook; Shutdown on it is a no-op.
func Wrap(l *slog.Logger) *Logger {
	if l == nil {
		l = slog.Default()
	}
	return &Logger{Logger: l}
}

// FromHandler builds a *Logger over a handler, the way slog.New does.
func FromHandler(h Handler) *Logger { return Wrap(slog.New(h)) }

// Default returns the process-wide logger as a *Logger.
func Default() *Logger { return Wrap(slog.Default()) }

// SetDefault installs l as the process-wide logger for both slogx and slog.
func SetDefault(l *Logger) {
	if l == nil || l.Logger == nil {
		return
	}
	slog.SetDefault(l.Logger)
}

// With returns a *Logger that adds args to every record.
func With(args ...any) *Logger { return Wrap(slog.Default().With(args...)) }

// WithGroup returns a *Logger that nests every record under name.
func WithGroup(name string) *Logger { return Wrap(slog.Default().WithGroup(name)) }

// With and WithGroup on a *Logger keep the shutdown hook of the parent: a
// derived logger writes through the same handler chain.
func (l *Logger) With(args ...any) *Logger {
	return &Logger{Logger: l.Logger.With(args...), shutdownFunc: l.shutdownFunc}
}

func (l *Logger) WithGroup(name string) *Logger {
	return &Logger{Logger: l.Logger.WithGroup(name), shutdownFunc: l.shutdownFunc}
}

func NewJSONHandler(w io.Writer, opts *HandlerOptions) Handler { return slog.NewJSONHandler(w, opts) }
func NewTextHandler(w io.Writer, opts *HandlerOptions) Handler { return slog.NewTextHandler(w, opts) }

func Debug(msg string, args ...any) { slog.Default().Debug(msg, args...) }
func Info(msg string, args ...any)  { slog.Default().Info(msg, args...) }
func Warn(msg string, args ...any)  { slog.Default().Warn(msg, args...) }
func Error(msg string, args ...any) { slog.Default().Error(msg, args...) }

func DebugContext(ctx context.Context, msg string, args ...any) {
	slog.Default().DebugContext(ctx, msg, args...)
}

func InfoContext(ctx context.Context, msg string, args ...any) {
	slog.Default().InfoContext(ctx, msg, args...)
}

func WarnContext(ctx context.Context, msg string, args ...any) {
	slog.Default().WarnContext(ctx, msg, args...)
}

func ErrorContext(ctx context.Context, msg string, args ...any) {
	slog.Default().ErrorContext(ctx, msg, args...)
}

func Log(ctx context.Context, level Level, msg string, args ...any) {
	slog.Default().Log(ctx, level, msg, args...)
}

func LogAttrs(ctx context.Context, level Level, msg string, attrs ...Attr) {
	slog.Default().LogAttrs(ctx, level, msg, attrs...)
}

func String(key, value string) Attr                 { return slog.String(key, value) }
func Int(key string, value int) Attr                { return slog.Int(key, value) }
func Int64(key string, value int64) Attr            { return slog.Int64(key, value) }
func Uint64(key string, value uint64) Attr          { return slog.Uint64(key, value) }
func Float64(key string, value float64) Attr        { return slog.Float64(key, value) }
func Bool(key string, value bool) Attr              { return slog.Bool(key, value) }
func Time(key string, value time.Time) Attr         { return slog.Time(key, value) }
func Duration(key string, value time.Duration) Attr { return slog.Duration(key, value) }
func Any(key string, value any) Attr                { return slog.Any(key, value) }
func Group(key string, args ...any) Attr            { return slog.Group(key, args...) }

// Fatal logs at error level and exits with status 1. It is for command-line
// entry points; library code returns errors.
func Fatal(msg string, args ...any) {
	slog.Default().Error(msg, args...)
	os.Exit(1)
}
