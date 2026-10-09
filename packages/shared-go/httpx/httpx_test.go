package httpx

import (
	"encoding/json"
	"errors"
	"io"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"github.com/gofiber/fiber/v2"
	"github.com/google/uuid"
	"github.com/stretchr/testify/require"
	"go.opentelemetry.io/otel/trace"
	"go.uber.org/zap"
	"go.uber.org/zap/zaptest/observer"
)

func TestRequestID(t *testing.T) {
	app := fiber.New()
	app.Use(RequestID())
	app.Get("/", func(c *fiber.Ctx) error { return c.SendString(RequestIDFrom(c)) })

	cases := []struct {
		name     string
		incoming string
		keep     bool
	}{
		{name: "kept", incoming: "abc-123_X.9", keep: true},
		{name: "generated when missing", incoming: ""},
		{name: "replaced when unsafe", incoming: "evil\"id<script>"},
		{name: "replaced when too long", incoming: strings.Repeat("a", 129)},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			req := httptest.NewRequest(http.MethodGet, "/", nil)
			if tc.incoming != "" {
				req.Header.Set(HeaderRequestID, tc.incoming)
			}
			resp, err := app.Test(req, -1)
			require.NoError(t, err)
			defer resp.Body.Close()
			got := resp.Header.Get(HeaderRequestID)
			body, _ := io.ReadAll(resp.Body)
			require.Equal(t, got, string(body), "handlers see the echoed ID")
			if tc.keep {
				require.Equal(t, tc.incoming, got)
				return
			}
			_, err = uuid.Parse(got)
			require.NoError(t, err)
		})
	}
}

func TestAccessLogAndErrorHandler(t *testing.T) {
	core, logs := observer.New(zap.DebugLevel)
	log := zap.New(core)
	app := fiber.New(fiber.Config{ErrorHandler: ErrorHandler(log)})
	app.Use(RequestID(), AccessLog(log))
	app.Get("/ok", func(c *fiber.Ctx) error { return WriteData(c, fiber.StatusCreated, fiber.Map{"id": 1}) })
	app.Get("/conflict", func(*fiber.Ctx) error { return NewAPIError(fiber.StatusConflict, "email_taken", "taken") })
	app.Get("/fiber", func(*fiber.Ctx) error { return fiber.NewError(fiber.StatusTooManyRequests, "slow down") })
	app.Get("/boom", func(*fiber.Ctx) error { return errors.New("db password=secret down") })
	app.Get("/health/live", func(c *fiber.Ctx) error { return c.SendStatus(fiber.StatusOK) })
	app.Get("/traced", func(c *fiber.Ctx) error {
		sc := trace.NewSpanContext(trace.SpanContextConfig{TraceID: trace.TraceID{1}, SpanID: trace.SpanID{1}})
		c.SetUserContext(trace.ContextWithSpanContext(c.UserContext(), sc))
		return c.SendStatus(fiber.StatusNoContent)
	})

	cases := []struct {
		path   string
		status int
		code   string
	}{
		{path: "/ok", status: 201},
		{path: "/conflict", status: 409, code: "email_taken"},
		{path: "/fiber", status: 429, code: "rate_limited"},
		{path: "/boom", status: 500, code: "internal_error"},
		{path: "/missing", status: 404, code: "not_found"},
	}
	for _, tc := range cases {
		resp, err := app.Test(httptest.NewRequest(http.MethodGet, tc.path, nil), -1)
		require.NoError(t, err)
		defer resp.Body.Close()
		require.Equal(t, tc.status, resp.StatusCode, tc.path)
		var body map[string]any
		require.NoError(t, json.NewDecoder(resp.Body).Decode(&body))
		require.NotEmpty(t, body["meta"].(map[string]any)["request_id"])
		if tc.code == "" {
			require.Nil(t, body["error"])
			require.Equal(t, float64(1), body["data"].(map[string]any)["id"])
			continue
		}
		require.Nil(t, body["data"])
		apiErr := body["error"].(map[string]any)
		require.Equal(t, tc.code, apiErr["code"], tc.path)
		require.NotContains(t, apiErr["message"], "secret", "internal details never reach the client")
	}
	live, err := app.Test(httptest.NewRequest(http.MethodGet, "/health/live", nil), -1)
	require.NoError(t, err)
	require.NoError(t, live.Body.Close())
	traced, err := app.Test(httptest.NewRequest(http.MethodGet, "/traced", nil), -1)
	require.NoError(t, err)
	require.NoError(t, traced.Body.Close())

	var statuses []int64
	for _, entry := range logs.FilterMessage("http request").All() {
		statuses = append(statuses, entry.ContextMap()["status"].(int64))
	}
	require.Equal(t, []int64{201, 409, 429, 500, 404, 200, 204}, statuses)
	require.Equal(t, zap.DebugLevel, logs.FilterMessage("http request").All()[5].Level, "probes are logged at debug")
	require.Equal(t, trace.TraceID{1}.String(), logs.FilterMessage("http request").All()[6].ContextMap()["trace_id"])
	require.Equal(t, 1, logs.FilterMessage("request failed").Len(), "only 5xx errors are logged with detail")
}

func TestAPIErrorCause(t *testing.T) {
	core, logs := observer.New(zap.DebugLevel)
	cause := errors.New("africastalking: 405 InsufficientBalance")
	base := NewAPIError(fiber.StatusServiceUnavailable, "sms_unavailable", "try again")
	app := fiber.New(fiber.Config{ErrorHandler: ErrorHandler(zap.New(core))})
	app.Get("/sms", func(*fiber.Ctx) error { return base.WithCause(cause) })

	resp, err := app.Test(httptest.NewRequest(http.MethodGet, "/sms", nil), -1)
	require.NoError(t, err)
	defer resp.Body.Close()
	require.Equal(t, http.StatusServiceUnavailable, resp.StatusCode)
	body, err := io.ReadAll(resp.Body)
	require.NoError(t, err)
	require.Contains(t, string(body), `"code":"sms_unavailable"`)
	require.NotContains(t, string(body), "InsufficientBalance")

	entries := logs.FilterMessage("request failed").All()
	require.Len(t, entries, 1)
	require.Contains(t, entries[0].ContextMap()["error"], "InsufficientBalance")
	require.Nil(t, base.Cause, "WithCause does not mutate the shared error")
	require.ErrorIs(t, base.WithCause(cause), cause)
	require.Equal(t, "sms_unavailable: try again", base.Error())
}

func TestCodeForStatus(t *testing.T) {
	for status, code := range map[int]string{
		400: "invalid_request", 401: "unauthorized", 403: "forbidden", 404: "not_found",
		405: "method_not_allowed", 409: "conflict", 413: "payload_too_large", 429: "rate_limited",
		503: "unavailable", 502: "internal_error", 418: "error",
	} {
		require.Equal(t, code, codeForStatus(status), status)
	}
}
