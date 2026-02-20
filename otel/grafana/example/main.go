// Example shows how to wire up the grafana plugin with a Kratos application.
package main

import (
	"context"
	"os"
	"time"

	grafana "github.com/towardsyou/kratos-contrib/otel/grafana"

	"github.com/go-kratos/kratos/v2/log"
	"github.com/go-kratos/kratos/v2/middleware/tracing"
)

func main() {
	ctx := context.Background()

	const (
		svcName    = "my-service"
		svcVersion = "v1.0.0"
		endpoint   = "otlp-gateway-prod-eu-west-2.grafana.net"
		instanceID = "<grafana-instance-id>"
		apiKey     = "<grafana-api-key>"
	)

	// --- Traces → Grafana Tempo ---
	traceShutdown, err := grafana.InitTracerProvider(ctx, svcName, svcVersion, grafana.TracerConfig{
		Endpoint:   endpoint,
		InstanceID: instanceID,
		APIKey:     apiKey,
		SampleRate: 1.0,
	})
	if err != nil {
		panic(err)
	}
	defer traceShutdown(ctx)

	// --- Logs → Grafana Loki ---
	logProvider, logShutdown, err := grafana.InitLoggerProvider(ctx, svcName, svcVersion, grafana.LoggerConfig{
		Endpoint:   endpoint,
		InstanceID: instanceID,
		APIKey:     apiKey,
	})
	if err != nil {
		panic(err)
	}
	defer logShutdown(ctx)

	// --- Metrics → Grafana Mimir ---
	metricShutdown, err := grafana.InitMeterProvider(ctx, svcName, svcVersion, grafana.MetricConfig{
		Endpoint:       endpoint,
		InstanceID:     instanceID,
		APIKey:         apiKey,
		ReportInterval: 30 * time.Second,
	})
	if err != nil {
		panic(err)
	}
	defer metricShutdown(ctx)

	// --- Kratos logger with OTel trace-log correlation ---
	stdLogger := log.NewStdLogger(os.Stdout)
	logger := log.With(
		grafana.NewLogger(stdLogger, logProvider),
		"ts", log.DefaultTimestamp,
		"caller", log.DefaultCaller,
		"service.name", svcName,
		"service.version", svcVersion,
		grafana.OTelCtxKey, grafana.ContextValuer(),
		"trace.id", tracing.TraceID(),
		"span.id", tracing.SpanID(),
	)

	helper := log.NewHelper(logger)

	// --- Trace + metric usage ---
	tracer := grafana.Tracer("example")
	meter := grafana.Meter("example")

	reqCounter, _ := meter.Int64Counter("example.requests")

	ctx2, finish := grafana.StartSpan(ctx, tracer, "example.operation")
	defer func() { finish(nil) }()

	reqCounter.Add(ctx2, 1)
	helper.WithContext(ctx2).Info("hello from grafana plugin")
}
