package platform

import (
	"context"
	"log/slog"

	"go.opentelemetry.io/otel"
	"go.opentelemetry.io/otel/attribute"
	"go.opentelemetry.io/otel/exporters/otlp/otlptrace/otlptracehttp"
	"go.opentelemetry.io/otel/propagation"
	"go.opentelemetry.io/otel/sdk/resource"
	sdktrace "go.opentelemetry.io/otel/sdk/trace"
)

type TracingConfig struct {
	Enabled     bool
	ServiceName string
	AppVersion  string
	Endpoint    string
	Insecure    bool
	SampleRatio float64
}

func InitTracing(ctx context.Context, cfg TracingConfig, logger *slog.Logger) (func(context.Context) error, error) {
	otel.SetTextMapPropagator(propagation.NewCompositeTextMapPropagator(
		propagation.TraceContext{},
		propagation.Baggage{},
	))

	if !cfg.Enabled {
		return func(context.Context) error { return nil }, nil
	}

	options := []otlptracehttp.Option{}
	if cfg.Endpoint != "" {
		options = append(options, otlptracehttp.WithEndpointURL(cfg.Endpoint))
	}
	if cfg.Insecure {
		options = append(options, otlptracehttp.WithInsecure())
	}

	exporter, err := otlptracehttp.New(ctx, options...)
	if err != nil {
		return nil, err
	}

	provider := sdktrace.NewTracerProvider(
		sdktrace.WithBatcher(exporter),
		sdktrace.WithSampler(sdktrace.TraceIDRatioBased(cfg.SampleRatio)),
		sdktrace.WithResource(resource.NewWithAttributes(
			"",
			attribute.String("service.name", cfg.ServiceName),
			attribute.String("service.version", cfg.AppVersion),
		)),
	)

	otel.SetTracerProvider(provider)
	if logger != nil {
		logger.Info("tracing enabled", "service", cfg.ServiceName, "endpoint", cfg.Endpoint, "sample_ratio", cfg.SampleRatio)
	}

	return provider.Shutdown, nil
}
