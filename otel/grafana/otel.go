// Package grafana provides OpenTelemetry initialization for Grafana Cloud.
//
// It supports exporting traces (Tempo), logs (Loki), and metrics (Mimir) via
// OTLP/HTTP to the Grafana Cloud OTLP gateway, using Basic Auth.
//
// Typical usage in main():
//
//	traceShutdown, err := grafana.InitTracerProvider(ctx, "my-svc", "v1.0.0", cfg.Trace)
//	logProvider, logShutdown, err := grafana.InitLoggerProvider(ctx, "my-svc", "v1.0.0", cfg.Log)
//	metricShutdown, err := grafana.InitMeterProvider(ctx, "my-svc", "v1.0.0", cfg.Metric)
//	defer traceShutdown(ctx)
//	defer logShutdown(ctx)
//	defer metricShutdown(ctx)
package grafana

import (
	"context"
	"encoding/base64"
	"time"

	"go.opentelemetry.io/otel"
	"go.opentelemetry.io/otel/exporters/otlp/otlplog/otlploghttp"
	"go.opentelemetry.io/otel/exporters/otlp/otlpmetric/otlpmetrichttp"
	"go.opentelemetry.io/otel/exporters/otlp/otlptrace/otlptracehttp"
	"go.opentelemetry.io/otel/exporters/stdout/stdouttrace"
	"go.opentelemetry.io/otel/propagation"
	sdklog "go.opentelemetry.io/otel/sdk/log"
	sdkmetric "go.opentelemetry.io/otel/sdk/metric"
	"go.opentelemetry.io/otel/sdk/resource"
	sdktrace "go.opentelemetry.io/otel/sdk/trace"
	semconv "go.opentelemetry.io/otel/semconv/v1.26.0"
)

// grafanaAuthHeaders returns Basic-Auth headers for the Grafana Cloud OTLP gateway.
// Returns nil when instanceID or apiKey is empty.
func grafanaAuthHeaders(instanceID, apiKey string) map[string]string {
	if instanceID == "" || apiKey == "" {
		return nil
	}
	token := base64.StdEncoding.EncodeToString([]byte(instanceID + ":" + apiKey))
	return map[string]string{"Authorization": "Basic " + token}
}

// newResource builds an OTel resource tagged with the given service name and version.
func newResource(ctx context.Context, serviceName, serviceVersion string) (*resource.Resource, error) {
	return resource.New(ctx,
		resource.WithHost(),
		resource.WithProcessRuntimeDescription(),
		resource.WithTelemetrySDK(),
		resource.WithAttributes(
			semconv.ServiceName(serviceName),
			semconv.ServiceVersion(serviceVersion),
		),
	)
}

// InitTracerProvider sets up and registers the global OTel TracerProvider.
//
// When cfg.Endpoint is non-empty, spans are exported via OTLP/HTTP to
// Grafana Tempo; otherwise a pretty-printed stdout exporter is used.
//
// Returns a shutdown function that must be deferred in the caller.
func InitTracerProvider(ctx context.Context, serviceName, serviceVersion string, cfg TracerConfig) (func(context.Context) error, error) {
	otel.SetTextMapPropagator(propagation.NewCompositeTextMapPropagator(
		propagation.TraceContext{},
		propagation.Baggage{},
	))

	res, err := newResource(ctx, serviceName, serviceVersion)
	if err != nil {
		return nil, err
	}

	var exporter sdktrace.SpanExporter
	if cfg.Endpoint != "" {
		opts := []otlptracehttp.Option{
			otlptracehttp.WithEndpoint(cfg.Endpoint),
			otlptracehttp.WithURLPath("/otlp/v1/traces"),
		}
		if cfg.Insecure {
			opts = append(opts, otlptracehttp.WithInsecure())
		}
		if headers := grafanaAuthHeaders(cfg.InstanceID, cfg.APIKey); headers != nil {
			opts = append(opts, otlptracehttp.WithHeaders(headers))
		}
		exporter, err = otlptracehttp.New(ctx, opts...)
	} else {
		exporter, err = stdouttrace.New(stdouttrace.WithPrettyPrint())
	}
	if err != nil {
		return nil, err
	}

	sampleRate := cfg.SampleRate
	if sampleRate <= 0 {
		sampleRate = 1.0
	}
	sampler := sdktrace.AlwaysSample()
	if sampleRate < 1.0 {
		sampler = sdktrace.TraceIDRatioBased(sampleRate)
	}

	tp := sdktrace.NewTracerProvider(
		sdktrace.WithSampler(sampler),
		sdktrace.WithBatcher(exporter),
		sdktrace.WithResource(res),
	)
	otel.SetTracerProvider(tp)

	return tp.Shutdown, nil
}

// InitLoggerProvider creates an OTel LoggerProvider that exports logs via
// OTLP/HTTP to Grafana Loki.
//
// When cfg.Endpoint is empty the provider is still returned but no OTLP
// export is configured; callers can still use NewLogger with a nil provider.
//
// Returns the provider and a shutdown function.
func InitLoggerProvider(ctx context.Context, serviceName, serviceVersion string, cfg LoggerConfig) (*sdklog.LoggerProvider, func(context.Context) error, error) {
	res, err := newResource(ctx, serviceName, serviceVersion)
	if err != nil {
		return nil, nil, err
	}

	opts := []otlploghttp.Option{
		otlploghttp.WithEndpoint(cfg.Endpoint),
		otlploghttp.WithURLPath("/otlp/v1/logs"),
	}
	if cfg.Insecure {
		opts = append(opts, otlploghttp.WithInsecure())
	}
	if headers := grafanaAuthHeaders(cfg.InstanceID, cfg.APIKey); headers != nil {
		opts = append(opts, otlploghttp.WithHeaders(headers))
	}

	exporter, err := otlploghttp.New(ctx, opts...)
	if err != nil {
		return nil, nil, err
	}

	lp := sdklog.NewLoggerProvider(
		sdklog.WithProcessor(sdklog.NewBatchProcessor(exporter)),
		sdklog.WithResource(res),
	)
	return lp, lp.Shutdown, nil
}

// InitMeterProvider sets up and registers the global OTel MeterProvider.
//
// When cfg.Endpoint is non-empty, metrics are exported via OTLP/HTTP to
// Grafana Mimir. When empty, a no-op MeterProvider is registered (metrics
// calls are safe but produce no output).
//
// Returns a shutdown function that must be deferred in the caller.
func InitMeterProvider(ctx context.Context, serviceName, serviceVersion string, cfg MetricConfig) (func(context.Context) error, error) {
	res, err := newResource(ctx, serviceName, serviceVersion)
	if err != nil {
		return nil, err
	}

	interval := cfg.ReportInterval
	if interval <= 0 {
		interval = 60 * time.Second
	}

	if cfg.Endpoint == "" {
		// No endpoint → no-op: register an empty MeterProvider so calls don't panic.
		mp := sdkmetric.NewMeterProvider(sdkmetric.WithResource(res))
		otel.SetMeterProvider(mp)
		return mp.Shutdown, nil
	}

	opts := []otlpmetrichttp.Option{
		otlpmetrichttp.WithEndpoint(cfg.Endpoint),
		otlpmetrichttp.WithURLPath("/otlp/v1/metrics"),
	}
	if cfg.Insecure {
		opts = append(opts, otlpmetrichttp.WithInsecure())
	}
	if headers := grafanaAuthHeaders(cfg.InstanceID, cfg.APIKey); headers != nil {
		opts = append(opts, otlpmetrichttp.WithHeaders(headers))
	}

	exporter, err := otlpmetrichttp.New(ctx, opts...)
	if err != nil {
		return nil, err
	}

	mp := sdkmetric.NewMeterProvider(
		sdkmetric.WithReader(
			sdkmetric.NewPeriodicReader(exporter, sdkmetric.WithInterval(interval)),
		),
		sdkmetric.WithResource(res),
	)
	otel.SetMeterProvider(mp)

	return mp.Shutdown, nil
}
