package slogx

import (
	"context"
	"fmt"
	"os"

	"go.opentelemetry.io/otel/attribute"
	"go.opentelemetry.io/otel/exporters/otlp/otlplog/otlploghttp"
	sdklog "go.opentelemetry.io/otel/sdk/log"
	"go.opentelemetry.io/otel/sdk/resource"
	semconv "go.opentelemetry.io/otel/semconv/v1.26.0"
)

func initOTelLoggerProvider(cfg Config) (*sdklog.LoggerProvider, func(context.Context) error, error) {
	ctx := context.Background()

	endpoint := cfg.OTelEndpoint
	if endpoint == "" {
		endpoint = "localhost:4318"
	}

	opts := []otlploghttp.Option{
		otlploghttp.WithEndpoint(endpoint),
	}
	if cfg.OTelInsecure {
		opts = append(opts, otlploghttp.WithInsecure())
	}

	exporter, err := otlploghttp.New(ctx, opts...)
	if err != nil {
		return nil, nil, fmt.Errorf("create otlp log exporter: %w", err)
	}

	serviceName := cfg.Service
	if serviceName == "" {
		serviceName = "app"
	}
	version := resolveVersion(cfg.Version)

	env := resolveEnv(cfg.Env)
	hostname, _ := os.Hostname()
	if hostname == "" {
		hostname = "unknown"
	}

	res, err := resource.New(ctx,
		resource.WithAttributes(
			semconv.ServiceNameKey.String(serviceName),
			semconv.ServiceVersionKey.String(version),
			semconv.HostNameKey.String(hostname),
			attribute.String("deployment.environment", env),
			attribute.String("env", env),
		),
	)
	if err != nil {
		return nil, nil, fmt.Errorf("create otel resource: %w", err)
	}

	processor := sdklog.NewBatchProcessor(exporter)
	provider := sdklog.NewLoggerProvider(
		sdklog.WithResource(res),
		sdklog.WithProcessor(processor),
	)

	return provider, provider.Shutdown, nil
}
