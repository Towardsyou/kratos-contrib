package grafana

import (
	"go.opentelemetry.io/otel"
	"go.opentelemetry.io/otel/metric"
)

// Meter returns a named OTel Meter from the global MeterProvider.
//
// Typical usage — declare once per package:
//
//	var meter = grafana.Meter("myapp/http")
//
// Then create instruments:
//
//	reqCounter, _ := meter.Int64Counter("http.server.requests")
//	reqDuration, _ := meter.Float64Histogram("http.server.duration")
func Meter(name string) metric.Meter {
	return otel.Meter(name)
}
