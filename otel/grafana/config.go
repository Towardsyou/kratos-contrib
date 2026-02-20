package grafana

import "time"

// TracerConfig configures the OpenTelemetry TracerProvider.
// Set Endpoint to a Grafana Cloud OTLP gateway to export traces to Grafana Tempo.
// Leave Endpoint empty to fall back to a stdout exporter (local development).
type TracerConfig struct {
	// Endpoint is the OTLP/HTTP gateway host (without scheme), e.g.
	// "otlp-gateway-prod-eu-west-2.grafana.net".
	// Empty → stdout pretty-print.
	Endpoint string

	// Insecure disables TLS. Use true only for local collectors.
	Insecure bool

	// SampleRate is the trace sampling ratio in [0, 1]. Defaults to 1.0.
	SampleRate float64

	// InstanceID and APIKey are used for Grafana Cloud OTLP Basic Auth.
	InstanceID string
	APIKey     string
}

// LoggerConfig configures the OpenTelemetry LoggerProvider.
// Exports logs to Grafana Loki via OTLP/HTTP.
type LoggerConfig struct {
	// Endpoint is the OTLP/HTTP gateway host (without scheme).
	// Empty → logs are written to stdout only.
	Endpoint string

	// Insecure disables TLS.
	Insecure bool

	// InstanceID and APIKey are used for Grafana Cloud OTLP Basic Auth.
	InstanceID string
	APIKey     string
}

// MetricConfig configures the OpenTelemetry MeterProvider.
// Exports metrics to Grafana Mimir via OTLP/HTTP.
type MetricConfig struct {
	// Endpoint is the OTLP/HTTP gateway host (without scheme).
	// Empty → metrics are not exported (no-op provider).
	Endpoint string

	// Insecure disables TLS.
	Insecure bool

	// InstanceID and APIKey are used for Grafana Cloud OTLP Basic Auth.
	InstanceID string
	APIKey     string

	// ReportInterval controls how often metrics are pushed. Defaults to 60s.
	ReportInterval time.Duration
}
