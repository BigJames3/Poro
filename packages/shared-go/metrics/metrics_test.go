package metrics

import (
	"io"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/gofiber/fiber/v2"
	"github.com/stretchr/testify/require"
)

func TestMetricsRecordRouteTemplates(t *testing.T) {
	m, err := New("user")
	require.NoError(t, err)
	app := fiber.New()
	app.Use(m.Middleware())
	app.Get("/metrics", m.Handler())
	app.Get("/users/:username", func(c *fiber.Ctx) error { return c.SendStatus(fiber.StatusOK) })

	for _, name := range []string{"ada", "grace", "linus"} {
		resp, err := app.Test(httptest.NewRequest(http.MethodGet, "/users/"+name, nil), -1)
		require.NoError(t, err)
		require.NoError(t, resp.Body.Close())
	}

	resp, err := app.Test(httptest.NewRequest(http.MethodGet, "/metrics", nil), -1)
	require.NoError(t, err)
	defer resp.Body.Close()
	require.Equal(t, http.StatusOK, resp.StatusCode)
	body, err := io.ReadAll(resp.Body)
	require.NoError(t, err)
	out := string(body)
	require.Contains(t, out, `http_requests_total{method="GET",route="/users/:username",service="user",status="200"} 3`)
	require.NotContains(t, out, "/users/ada", "raw paths never become labels")
	require.Contains(t, out, "http_request_duration_seconds_bucket")
	require.Contains(t, out, "go_goroutines")
	require.NotNil(t, m.Registry())
}
