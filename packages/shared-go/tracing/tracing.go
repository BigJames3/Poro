// Package tracing configures OpenTelemetry tracing. Without an OTLP endpoint
// only W3C trace context propagation is enabled and no span is exported.
package tracing

import (
	"context"
	"fmt"
	"net/http"

	"github.com/gofiber/fiber/v2"
	"go.opentelemetry.io/otel"
	"go.opentelemetry.io/otel/attribute"
	"go.opentelemetry.io/otel/codes"
	"go.opentelemetry.io/otel/exporters/otlp/otlptrace/otlptracehttp"
	"go.opentelemetry.io/otel/propagation"
	"go.opentelemetry.io/otel/sdk/resource"
	sdktrace "go.opentelemetry.io/otel/sdk/trace"
	"go.opentelemetry.io/otel/trace"
)

const instrumentation = "github.com/poro/shared-go/tracing"

// Config selects the exporter.
type Config struct {
	Service  string
	Version  string
	Endpoint string // OTLP/HTTP URL, e.g. http://otel-collector:4318; empty disables export
}

// Setup installs the global tracer provider and propagator. The returned
// function flushes pending spans and must be called on shutdown.
func Setup(ctx context.Context, cfg Config) (func(context.Context) error, error) {
	otel.SetTextMapPropagator(propagation.NewCompositeTextMapPropagator(propagation.TraceContext{}, propagation.Baggage{}))
	if cfg.Endpoint == "" {
		return func(context.Context) error { return nil }, nil
	}
	exporter, err := otlptracehttp.New(ctx, otlptracehttp.WithEndpointURL(cfg.Endpoint))
	if err != nil {
		return nil, fmt.Errorf("otlp exporter: %w", err)
	}
	res, err := resource.New(ctx, resource.WithAttributes(
		attribute.String("service.name", cfg.Service),
		attribute.String("service.version", cfg.Version),
	))
	if err != nil {
		return nil, fmt.Errorf("otel resource: %w", err)
	}
	tp := sdktrace.NewTracerProvider(sdktrace.WithBatcher(exporter), sdktrace.WithResource(res))
	otel.SetTracerProvider(tp)
	return tp.Shutdown, nil
}

// Middleware starts a server span per request, continuing an incoming traceparent.
// It must run outside AccessLog so the span sees the resolved status.
func Middleware() fiber.Handler {
	tracer := otel.Tracer(instrumentation)
	return func(c *fiber.Ctx) error {
		carrier := propagation.HeaderCarrier(http.Header(c.GetReqHeaders()))
		ctx := otel.GetTextMapPropagator().Extract(c.UserContext(), carrier)
		ctx, span := tracer.Start(ctx, c.Method(), trace.WithSpanKind(trace.SpanKindServer))
		defer span.End()
		c.SetUserContext(ctx)

		err := c.Next()

		status := c.Response().StatusCode()
		span.SetName(c.Method() + " " + c.Route().Path)
		span.SetAttributes(
			attribute.String("http.request.method", c.Method()),
			attribute.String("http.route", c.Route().Path),
			attribute.Int("http.response.status_code", status),
		)
		if status >= fiber.StatusInternalServerError {
			span.SetStatus(codes.Error, http.StatusText(status))
		}
		return err
	}
}
