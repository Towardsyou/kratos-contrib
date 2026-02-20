package grafana

import (
	"context"

	"go.opentelemetry.io/otel"
	"go.opentelemetry.io/otel/codes"
	"go.opentelemetry.io/otel/trace"
)

// Tracer returns a named OTel Tracer from the global TracerProvider.
//
// Typical usage — declare once per file or struct:
//
//	var tracer = grafana.Tracer("biz")
func Tracer(name string) trace.Tracer {
	return otel.Tracer(name)
}

// StartSpan starts a child span and returns a finish function.
// The finish function records the error (if non-nil) and ends the span.
//
// Usage:
//
//	ctx, finish := grafana.StartSpan(ctx, tracer, "UserUsecase.Register")
//	defer func() { finish(err) }()
func StartSpan(ctx context.Context, tracer trace.Tracer, name string, opts ...trace.SpanStartOption) (context.Context, func(error)) {
	ctx, span := tracer.Start(ctx, name, opts...)
	return ctx, func(err error) {
		if err != nil {
			span.RecordError(err)
			span.SetStatus(codes.Error, err.Error())
		}
		span.End()
	}
}
