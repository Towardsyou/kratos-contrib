package grafana

import (
	"context"
	"fmt"
	"time"

	kratoslog "github.com/go-kratos/kratos/v2/log"
	otellog "go.opentelemetry.io/otel/log"
	sdklog "go.opentelemetry.io/otel/sdk/log"
)

// OTelCtxKey is the keyval key used to carry a context.Context through the
// Kratos logger pipeline. The OTel-aware logger extracts it for trace-log
// correlation and strips it from stdout output.
const OTelCtxKey = "_otel_ctx"

// ContextValuer returns a Kratos log.Valuer that captures the request context.
// Use it with log.With so every log line is automatically correlated with its
// active trace span.
//
//	logger = log.With(logger,
//	    grafana.OTelCtxKey, grafana.ContextValuer(),
//	    "trace.id",         tracing.TraceID(),
//	    "span.id",          tracing.SpanID(),
//	)
func ContextValuer() kratoslog.Valuer {
	return func(ctx context.Context) interface{} {
		return ctx
	}
}

// NewLogger wraps a Kratos logger so that every log call is:
//  1. Written to stdout (with OTelCtxKey stripped out).
//  2. Optionally forwarded to an OTel LoggerProvider via OTLP.
//
// Pass nil for logProvider to disable OTLP export (stdout-only mode).
// Obtain a logProvider with [InitLoggerProvider].
func NewLogger(stdout kratoslog.Logger, logProvider *sdklog.LoggerProvider) kratoslog.Logger {
	var otelLogger otellog.Logger
	if logProvider != nil {
		otelLogger = logProvider.Logger("kratos")
	}
	return &otelAwareLogger{
		stdout:  stdout,
		otelLog: otelLogger,
	}
}

type otelAwareLogger struct {
	stdout  kratoslog.Logger
	otelLog otellog.Logger
}

// Log implements [kratoslog.Logger].
func (l *otelAwareLogger) Log(level kratoslog.Level, keyvals ...interface{}) error {
	var ctx context.Context

	// Separate context from keyvals so it is never printed to stdout.
	filtered := make([]interface{}, 0, len(keyvals))
	for i := 0; i < len(keyvals); i += 2 {
		if i+1 >= len(keyvals) {
			filtered = append(filtered, keyvals[i])
			break
		}
		key := fmt.Sprint(keyvals[i])
		if key == OTelCtxKey {
			if c, ok := keyvals[i+1].(context.Context); ok {
				ctx = c
			}
			continue
		}
		filtered = append(filtered, keyvals[i], keyvals[i+1])
	}

	if err := l.stdout.Log(level, filtered...); err != nil {
		return err
	}

	if l.otelLog != nil {
		l.emitOTel(ctx, level, filtered)
	}
	return nil
}

func (l *otelAwareLogger) emitOTel(ctx context.Context, level kratoslog.Level, keyvals []interface{}) {
	if ctx == nil {
		ctx = context.Background()
	}

	var record otellog.Record
	record.SetTimestamp(time.Now())
	record.SetSeverity(toOTelSeverity(level))
	record.SetSeverityText(level.String())

	attrs := make([]otellog.KeyValue, 0, len(keyvals)/2)
	for i := 0; i < len(keyvals); i += 2 {
		if i+1 >= len(keyvals) {
			break
		}
		key := fmt.Sprint(keyvals[i])
		val := keyvals[i+1]
		if key == "msg" {
			record.SetBody(otellog.StringValue(fmt.Sprint(val)))
			continue
		}
		attrs = append(attrs, otellog.String(key, fmt.Sprint(val)))
	}

	record.AddAttributes(attrs...)
	l.otelLog.Emit(ctx, record)
}

func toOTelSeverity(level kratoslog.Level) otellog.Severity {
	switch level {
	case kratoslog.LevelDebug:
		return otellog.SeverityDebug
	case kratoslog.LevelInfo:
		return otellog.SeverityInfo
	case kratoslog.LevelWarn:
		return otellog.SeverityWarn
	case kratoslog.LevelError:
		return otellog.SeverityError
	case kratoslog.LevelFatal:
		return otellog.SeverityFatal
	default:
		return otellog.SeverityInfo
	}
}
