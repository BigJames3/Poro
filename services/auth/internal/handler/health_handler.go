// Package handler exposes the auth HTTP endpoints.
package handler

import (
	"context"
	"time"

	"github.com/gofiber/fiber/v2"
	"github.com/jackc/pgx/v5/pgxpool"
	"github.com/redis/go-redis/v9"
)

const healthTimeout = 2 * time.Second

// HealthHandler reports whether Postgres and Redis answer.
// A nil pool or client counts as down. The handler always responds HTTP 200.
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

// Health handles GET /health.
func (h *HealthHandler) Health(c *fiber.Ctx) error {
	postgres := h.postgresStatus(c.UserContext())
	redisStatus := h.redisStatus(c.UserContext())
	status := "ok"
	if postgres != "up" || redisStatus != "up" {
		status = "degraded"
	}
	return c.Status(fiber.StatusOK).JSON(fiber.Map{
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
