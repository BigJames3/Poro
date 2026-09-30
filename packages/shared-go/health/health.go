// Package health serves the liveness and readiness probes of a service.
package health

import (
	"context"
	"sort"
	"time"

	"github.com/gofiber/fiber/v2"
)

const checkTimeout = 2 * time.Second

// Check reports whether one dependency is usable.
type Check func(ctx context.Context) error

// Handler answers /health/live and /health/ready.
type Handler struct {
	service string
	version string
	checks  map[string]Check
}

// New builds a probe handler. Readiness fails when any check fails.
func New(service, version string, checks map[string]Check) *Handler {
	return &Handler{service: service, version: version, checks: checks}
}

// Register mounts /health/live, /health/ready and /health (alias of ready).
func (h *Handler) Register(app fiber.Router) {
	app.Get("/health", h.Ready)
	app.Get("/health/live", h.Live)
	app.Get("/health/ready", h.Ready)
}

// Live answers 200 while the process can serve HTTP. It checks no dependency,
// so a database outage does not make the orchestrator restart every pod.
func (h *Handler) Live(c *fiber.Ctx) error {
	return c.JSON(fiber.Map{"status": "ok", "service": h.service, "version": h.version})
}

// Ready answers 503 when a dependency is down, so traffic moves to healthy instances.
func (h *Handler) Ready(c *fiber.Ctx) error {
	names := make([]string, 0, len(h.checks))
	for name := range h.checks {
		names = append(names, name)
	}
	sort.Strings(names)

	results := fiber.Map{}
	status, code := "ok", fiber.StatusOK
	for _, name := range names {
		ctx, cancel := context.WithTimeout(c.UserContext(), checkTimeout)
		err := h.checks[name](ctx)
		cancel()
		if err != nil {
			results[name] = "down"
			status, code = "degraded", fiber.StatusServiceUnavailable
			continue
		}
		results[name] = "up"
	}
	return c.Status(code).JSON(fiber.Map{
		"status":  status,
		"service": h.service,
		"version": h.version,
		"checks":  results,
	})
}
