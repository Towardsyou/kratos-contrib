//go:build e2e

package e2e

import (
	"context"
	"io"
	"net/http"
	"net/http/httptest"
	"os"
	"strings"
	"sync/atomic"
	"testing"
	"time"

	kratoslog "github.com/go-kratos/kratos/v2/log"
	grafana "github.com/towardsyou/kratos-contrib/otel/grafana"
)

// TestOTelExport verifies that all three telemetry signals (traces, logs,
// metrics) are exported via OTLP/HTTP by pointing the providers at an
// in-process mock server — no docker required.
func TestOTelExport(t *testing.T) {
	var tracesHit, logsHit, metricsHit atomic.Bool

	// Mock OTLP HTTP receiver — records which signal endpoints were called.
	mux := http.NewServeMux()
	mux.HandleFunc("/otlp/v1/traces", func(w http.ResponseWriter, r *http.Request) {
		_, _ = io.ReadAll(r.Body)
		tracesHit.Store(true)
		// OTLP expects a valid protobuf response; return an empty JSON object
		// as the SDK accepts 200 with any body (it ignores the response).
		w.Header().Set("Content-Type", "application/json")
		w.WriteHeader(http.StatusOK)
		_, _ = w.Write([]byte("{}"))
	})
	mux.HandleFunc("/otlp/v1/logs", func(w http.ResponseWriter, r *http.Request) {
		_, _ = io.ReadAll(r.Body)
		logsHit.Store(true)
		w.Header().Set("Content-Type", "application/json")
		w.WriteHeader(http.StatusOK)
		_, _ = w.Write([]byte("{}"))
	})
	mux.HandleFunc("/otlp/v1/metrics", func(w http.ResponseWriter, r *http.Request) {
		_, _ = io.ReadAll(r.Body)
		metricsHit.Store(true)
		w.Header().Set("Content-Type", "application/json")
		w.WriteHeader(http.StatusOK)
		_, _ = w.Write([]byte("{}"))
	})

	mockSrv := httptest.NewServer(mux)
	defer mockSrv.Close()

	// The OTel OTLP HTTP exporter expects host:port (no scheme).
	otlpEndpoint := strings.TrimPrefix(mockSrv.URL, "http://")

	ctx := context.Background()
	const svcName = "e2e-otel-test"
	const svcVersion = "v0.0.1-e2e"

	// --- Traces ---
	traceShutdown, err := grafana.InitTracerProvider(ctx, svcName, svcVersion, grafana.TracerConfig{
		Endpoint:   otlpEndpoint,
		Insecure:   true,
		SampleRate: 1.0,
	})
	if err != nil {
		t.Fatalf("InitTracerProvider: %v", err)
	}
	defer func() { _ = traceShutdown(ctx) }()

	// --- Logs ---
	logProvider, logShutdown, err := grafana.InitLoggerProvider(ctx, svcName, svcVersion, grafana.LoggerConfig{
		Endpoint: otlpEndpoint,
		Insecure: true,
	})
	if err != nil {
		t.Fatalf("InitLoggerProvider: %v", err)
	}
	defer func() { _ = logShutdown(ctx) }()

	// --- Metrics ---
	metricShutdown, err := grafana.InitMeterProvider(ctx, svcName, svcVersion, grafana.MetricConfig{
		Endpoint:       otlpEndpoint,
		Insecure:       true,
		ReportInterval: 10 * time.Millisecond, // short interval to help flush
	})
	if err != nil {
		t.Fatalf("InitMeterProvider: %v", err)
	}
	defer func() { _ = metricShutdown(ctx) }()

	// --- Emit a trace span ---
	tracer := grafana.Tracer("e2e-tracer")
	spanCtx, finish := grafana.StartSpan(ctx, tracer, "e2e.test_operation")
	finish(nil)
	_ = spanCtx

	// --- Emit a log record ---
	stdLogger := kratoslog.NewStdLogger(os.Stdout)
	logger := grafana.NewLogger(stdLogger, logProvider)
	helper := kratoslog.NewHelper(logger)
	helper.Info("e2e test log message")

	// --- Emit a metric ---
	meter := grafana.Meter("e2e-meter")
	counter, err := meter.Int64Counter("e2e.test.requests")
	if err != nil {
		t.Fatalf("create counter: %v", err)
	}
	counter.Add(ctx, 1)

	// Flush all providers — Shutdown forces a final export of any buffered data.
	if err := traceShutdown(ctx); err != nil {
		t.Errorf("traceShutdown: %v", err)
	}
	if err := logShutdown(ctx); err != nil {
		t.Errorf("logShutdown: %v", err)
	}
	if err := metricShutdown(ctx); err != nil {
		t.Errorf("metricShutdown: %v", err)
	}

	if !tracesHit.Load() {
		t.Error("OTLP /otlp/v1/traces was never called — traces were not exported")
	}
	if !logsHit.Load() {
		t.Error("OTLP /otlp/v1/logs was never called — logs were not exported")
	}
	if !metricsHit.Load() {
		t.Error("OTLP /otlp/v1/metrics was never called — metrics were not exported")
	}
}
