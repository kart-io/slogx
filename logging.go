package slogx

import (
	"context"
	"fmt"
	"io"
	"log/slog"
	"os"
	"strings"
	"time"

	"go.opentelemetry.io/contrib/bridges/otelslog"
)

type Config struct {
	Level        string
	Format       string
	AddSource    bool
	Service      string
	Version      string
	Env          string
	OTelEnabled  bool
	OTelEndpoint string
	OTelInsecure bool
}

type Logger struct {
	*slog.Logger
	shutdownFunc func(context.Context) error
}

func (l *Logger) Shutdown(ctx context.Context) error {
	if l.shutdownFunc != nil {
		return l.shutdownFunc(ctx)
	}
	return nil
}

// Init creates a new Logger, sets it as the default slog logger, and returns a cleanup func.
func Init(out io.Writer, cfg Config) (*Logger, func(), error) {
	if out == nil {
		out = os.Stdout
	}
	logger, err := New(out, cfg)
	if err != nil {
		return nil, func() {}, err
	}

	slog.SetDefault(logger.Logger)

	cleanup := func() {
		ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
		defer cancel()
		if err := logger.Shutdown(ctx); err != nil {
			_, _ = fmt.Fprintf(os.Stderr, "logging shutdown failed: %v\n", err)
		}
	}

	return logger, cleanup, nil
}

// New creates a new Logger instance based on the configuration.
func New(out io.Writer, cfg Config) (*Logger, error) {
	level, err := parseLevel(cfg.Level)
	if err != nil {
		return nil, err
	}
	opts := &slog.HandlerOptions{Level: level, AddSource: true, ReplaceAttr: redactAttr}
	var consoleHandler slog.Handler
	switch strings.ToLower(strings.TrimSpace(cfg.Format)) {
	case "", "json":
		consoleHandler = slog.NewJSONHandler(out, opts)
	case "text":
		consoleHandler = slog.NewTextHandler(out, opts)
	default:
		return nil, fmt.Errorf("unsupported log format %q", cfg.Format)
	}

	handlers := []slog.Handler{consoleHandler}
	var shutdownFn func(context.Context) error

	if cfg.OTelEnabled {
		provider, cleanup, err := initOTelLoggerProvider(cfg)
		if err != nil {
			return nil, fmt.Errorf("failed to init otel logger provider: %w", err)
		}
		shutdownFn = cleanup

		serviceName := cfg.Service
		if serviceName == "" {
			serviceName = "app"
		}
		version := resolveVersion(cfg.Version)

		otelHandler := otelslog.NewHandler(
			serviceName,
			otelslog.WithVersion(version),
			otelslog.WithLoggerProvider(provider),
		)
		handlers = append(handlers, otelHandler)
	}

	var finalHandler slog.Handler
	if len(handlers) == 1 {
		finalHandler = handlers[0]
	} else {
		finalHandler = NewMultiHandler(handlers...)
	}
	finalHandler = newTraceHandler(finalHandler)
	finalHandler = newCallerHandler(finalHandler, cfg.AddSource)

	slogger := slog.New(finalHandler)
	if cfg.Service != "" {
		slogger = slogger.With("service", cfg.Service)
	}
	if env := resolveEnv(cfg.Env); env != "" {
		slogger = slogger.With("env", env)
	}

	return &Logger{
		Logger:       slogger,
		shutdownFunc: shutdownFn,
	}, nil
}

func parseLevel(value string) (slog.Level, error) {
	switch strings.ToLower(strings.TrimSpace(value)) {
	case "", "info":
		return slog.LevelInfo, nil
	case "debug":
		return slog.LevelDebug, nil
	case "warn", "warning":
		return slog.LevelWarn, nil
	case "error":
		return slog.LevelError, nil
	default:
		return 0, fmt.Errorf("unsupported log level %q", value)
	}
}
