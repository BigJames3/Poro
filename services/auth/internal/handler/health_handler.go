package handler

import (
	"context"
	"time"

	"github.com/gofiber/fiber/v2"
	"github.com/jackc/pgx/v5/pgxpool"
	"github.com/redis/go-redis/v9"
)

const healthTimeout = 2 * time.Second

// HealthHandler serves the liveness and readiness probes.
// A nil pool or client counts as down.
type HealthHandler struct {
	pool    *pgxpool.Pool
	rdb     *redis.Client
	version string
}

// NewHealthHandler returns a health handler. version is the service version
// reported in the body.
func NewHealthHandler(pool *pgxpool.Pool, rdb *redis.Client, version string) *HealthHandler {
	return &HealthHandler{pool: pool, rdb: rdb, version: version}
}

// Live handles GET /health/live. It answers 200 while the process serves HTTP.
func (h *HealthHandler) Live(c *fiber.Ctx) error {
	return c.JSON(fiber.Map{"status": "ok", "service": "auth", "version": h.version})
}

// Ready handles GET /health/ready. It answers 503 when Postgres or Redis is down,
// so the orchestrator stops routing traffic to this instance.
func (h *HealthHandler) Ready(c *fiber.Ctx) error {
	postgres := h.postgresStatus(c.UserContext())
	redisStatus := h.redisStatus(c.UserContext())
	status, code := "ok", fiber.StatusOK
	if postgres != "up" || redisStatus != "up" {
		status, code = "degraded", fiber.StatusServiceUnavailable
	}
	return c.Status(code).JSON(fiber.Map{
		"status":  status,
		"service": "auth",
		"version": h.version,
		"checks": fiber.Map{
			"postgres": postgres,
			"redis":    redisStatus,
		},
	})
}

func (h *HealthHandler) postgresStatus(parent context.Context) string {
	if h.pool == nil {
		return "down"
	}
	ctx, cancel := context.WithTimeout(parent, healthTimeout)
	defer cancel()
	var one int
	if err := h.pool.QueryRow(ctx, "SELECT 1").Scan(&one); err != nil || one != 1 {
		return "down"
	}
	return "up"
}

func (h *HealthHandler) redisStatus(parent context.Context) string {
	if h.rdb == nil {
		return "down"
	}
	ctx, cancel := context.WithTimeout(parent, healthTimeout)
	defer cancel()
	if err := h.rdb.Ping(ctx).Err(); err != nil {
		return "down"
	}
	return "up"
}
