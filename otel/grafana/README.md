# otel/grafana

[![Go Reference](https://pkg.go.dev/badge/github.com/towardsyou/kratos-contrib/otel/grafana.svg)](https://pkg.go.dev/github.com/towardsyou/kratos-contrib/otel/grafana)
[![Go Report Card](https://goreportcard.com/badge/github.com/towardsyou/kratos-contrib/otel/grafana)](https://goreportcard.com/report/github.com/towardsyou/kratos-contrib/otel/grafana)

OpenTelemetry plugin for [Kratos](https://github.com/go-kratos/kratos) that exports **traces**, **logs**, and **metrics** to [Grafana Cloud](https://grafana.com/products/cloud/) via OTLP/HTTP.

| Signal  | Grafana Product | OTLP Path              |
|---------|-----------------|------------------------|
| Traces  | Tempo           | `/otlp/v1/traces`      |
| Logs    | Loki            | `/otlp/v1/logs`        |
| Metrics | Mimir           | `/otlp/v1/metrics`     |

## Installation

```bash
go get github.com/towardsyou/kratos-contrib/otel/grafana
```

## Quick Start

```go
import (
    grafana "github.com/towardsyou/kratos-contrib/otel/grafana"
    "github.com/go-kratos/kratos/v2/log"
    "github.com/go-kratos/kratos/v2/middleware/tracing"
)

const (
    endpoint   = "otlp-gateway-prod-eu-west-2.grafana.net"
    instanceID = "<your-instance-id>"
    apiKey     = "<your-api-key>"
)

// Traces
traceShutdown, _ := grafana.InitTracerProvider(ctx, "my-svc", "v1.0.0", grafana.TracerConfig{
    Endpoint:   endpoint,
    InstanceID: instanceID,
    APIKey:     apiKey,
    SampleRate: 1.0,
})
defer traceShutdown(ctx)

// Logs
logProvider, logShutdown, _ := grafana.InitLoggerProvider(ctx, "my-svc", "v1.0.0", grafana.LoggerConfig{
    Endpoint:   endpoint,
    InstanceID: instanceID,
    APIKey:     apiKey,
})
defer logShutdown(ctx)

// Metrics
metricShutdown, _ := grafana.InitMeterProvider(ctx, "my-svc", "v1.0.0", grafana.MetricConfig{
    Endpoint:       endpoint,
    InstanceID:     instanceID,
    APIKey:         apiKey,
    ReportInterval: 30 * time.Second,
})
defer metricShutdown(ctx)

// Kratos logger with trace-log correlation
logger := log.With(
    grafana.NewLogger(log.NewStdLogger(os.Stdout), logProvider),
    "ts",      log.DefaultTimestamp,
    "caller",  log.DefaultCaller,
    grafana.OTelCtxKey, grafana.ContextValuer(),
    "trace.id", tracing.TraceID(),
    "span.id",  tracing.SpanID(),
)
```

## Using Traces and Metrics

```go
// Declare per-package
var tracer = grafana.Tracer("biz")
var meter  = grafana.Meter("biz")

// Create metric instruments
reqCounter, _ := meter.Int64Counter("http.server.requests")

func (u *UserUsecase) Register(ctx context.Context, req *RegisterReq) (err error) {
    ctx, finish := grafana.StartSpan(ctx, tracer, "UserUsecase.Register")
    defer func() { finish(err) }()

    reqCounter.Add(ctx, 1)
    // ...
}
```

## Local Development (stdout fallback)

When `TracerConfig.Endpoint` is empty, traces are printed to stdout in pretty format.  
When `LoggerConfig.Endpoint` is empty, logs go to stdout only.  
When `MetricConfig.Endpoint` is empty, a no-op MeterProvider is used (safe, no panic).

## Configuration Reference

### TracerConfig

| Field        | Type      | Default | Description                                   |
|--------------|-----------|---------|-----------------------------------------------|
| `Endpoint`   | `string`  | `""`    | OTLP gateway host. Empty → stdout exporter.   |
| `Insecure`   | `bool`    | `false` | Disable TLS (local collectors only).          |
| `SampleRate` | `float64` | `1.0`   | Sampling ratio [0, 1].                        |
| `InstanceID` | `string`  | `""`    | Grafana Cloud instance ID for Basic Auth.     |
| `APIKey`     | `string`  | `""`    | Grafana Cloud API key for Basic Auth.         |

### LoggerConfig

| Field        | Type     | Default | Description                                    |
|--------------|----------|---------|------------------------------------------------|
| `Endpoint`   | `string` | `""`    | OTLP gateway host. Empty → stdout only.        |
| `Insecure`   | `bool`   | `false` | Disable TLS.                                   |
| `InstanceID` | `string` | `""`    | Grafana Cloud instance ID.                     |
| `APIKey`     | `string` | `""`    | Grafana Cloud API key.                         |

### MetricConfig

| Field            | Type            | Default | Description                                  |
|------------------|-----------------|---------|----------------------------------------------|
| `Endpoint`       | `string`        | `""`    | OTLP gateway host. Empty → no-op provider.   |
| `Insecure`       | `bool`          | `false` | Disable TLS.                                 |
| `InstanceID`     | `string`        | `""`    | Grafana Cloud instance ID.                   |
| `APIKey`         | `string`        | `""`    | Grafana Cloud API key.                       |
| `ReportInterval` | `time.Duration` | `60s`   | How often metrics are pushed.                |
