package health

import (
	"context"
	"encoding/json"
	"errors"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/gofiber/fiber/v2"
	"github.com/stretchr/testify/require"
)

func TestProbes(t *testing.T) {
	var redisErr error
	app := fiber.New()
	New("user", "1.0.0", map[string]Check{
		"postgres": func(context.Context) error { return nil },
		"redis":    func(context.Context) error { return redisErr },
	}).Register(app)

	get := func(path string) (int, map[string]any) {
		resp, err := app.Test(httptest.NewRequest(http.MethodGet, path, nil), -1)
		require.NoError(t, err)
		defer resp.Body.Close()
		var body map[string]any
		require.NoError(t, json.NewDecoder(resp.Body).Decode(&body))
		return resp.StatusCode, body
	}

	status, body := get("/health/ready")
	require.Equal(t, http.StatusOK, status)
	require.Equal(t, map[string]any{"postgres": "up", "redis": "up"}, body["checks"])

	redisErr = errors.New("down")
	status, body = get("/health/ready")
	require.Equal(t, http.StatusServiceUnavailable, status)
	require.Equal(t, "degraded", body["status"])
	require.Equal(t, "down", body["checks"].(map[string]any)["redis"])

	status, _ = get("/health")
	require.Equal(t, http.StatusServiceUnavailable, status, "/health is the readiness probe")

	status, body = get("/health/live")
	require.Equal(t, http.StatusOK, status, "liveness ignores dependencies")
	require.Equal(t, "user", body["service"])
}
