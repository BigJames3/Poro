package tracing

import (
	"context"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/gofiber/fiber/v2"
	"github.com/stretchr/testify/require"
	"go.opentelemetry.io/otel"
	"go.opentelemetry.io/otel/codes"
	sdktrace "go.opentelemetry.io/otel/sdk/trace"
	"go.opentelemetry.io/otel/sdk/trace/tracetest"
	"go.opentelemetry.io/otel/trace"
)

func TestSetupWithoutEndpointOnlyPropagates(t *testing.T) {
	shutdown, err := Setup(context.Background(), Config{Service: "user", Version: "1"})
	require.NoError(t, err)
	require.NoError(t, shutdown(context.Background()))
	require.Contains(t, otel.GetTextMapPropagator().Fields(), "traceparent")
}

func TestSetupWithEndpoint(t *testing.T) {
	shutdown, err := Setup(context.Background(), Config{Service: "user", Version: "1", Endpoint: "http://127.0.0.1:1/v1/traces"})
	require.NoError(t, err)
	ctx, cancel := context.WithCancel(context.Background())
	cancel()
	_ = shutdown(ctx)
}

func TestMiddlewareContinuesIncomingTrace(t *testing.T) {
	recorder := tracetest.NewSpanRecorder()
	otel.SetTracerProvider(sdktrace.NewTracerProvider(sdktrace.WithSpanProcessor(recorder)))
	_, err := Setup(context.Background(), Config{})
	require.NoError(t, err)

	var seen trace.SpanContext
	app := fiber.New()
	app.Use(Middleware())
	app.Get("/users/:id", func(c *fiber.Ctx) error {
		seen = trace.SpanContextFromContext(c.UserContext())
		return c.SendStatus(fiber.StatusBadGateway)
	})

	req := httptest.NewRequest(http.MethodGet, "/users/42", nil)
	req.Header.Set("traceparent", "00-4bf92f3577b34da6a3ce929d0e0e4736-00f067aa0ba902b7-01")
	resp, err := app.Test(req, -1)
	require.NoError(t, err)
	require.NoError(t, resp.Body.Close())

	require.Equal(t, "4bf92f3577b34da6a3ce929d0e0e4736", seen.TraceID().String(), "the handler joins the caller's trace")
	spans := recorder.Ended()
	require.Len(t, spans, 1)
	require.Equal(t, "GET /users/:id", spans[0].Name())
	require.Equal(t, codes.Error, spans[0].Status().Code)
	require.Equal(t, trace.SpanKindServer, spans[0].SpanKind())
}
