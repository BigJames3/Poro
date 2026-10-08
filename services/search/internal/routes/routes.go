// Package routes registers the search HTTP routes.
package routes

import (
	"github.com/gofiber/fiber/v2"

	"github.com/poro/shared-go/health"
	"github.com/poro/shared-go/jwtauth"

	"github.com/poro/search/internal/handler"
	"github.com/poro/search/internal/middleware"
)

// Dependencies are the handlers and middleware the router needs.
type Dependencies struct {
	Search  *handler.Search
	Health  *health.Handler
	Metrics fiber.Handler
	JWT     *jwtauth.Verifier
	Limiter fiber.Storage
}

// Register mounts the probes (at the root and under /api/v1), the metrics and
// /api/v1/search, where a token is optional and only keys the rate limit.
// /metrics must not be exposed by the public gateway.
func Register(app *fiber.App, deps Dependencies) {
	deps.Health.Register(app)
	deps.Health.Register(app.Group("/api/v1"))
	app.Get("/metrics", deps.Metrics)

	search := app.Group("/api/v1/search", jwtauth.OptionalMiddleware(deps.JWT), middleware.GeneralLimit(deps.Limiter))
	search.Get("/", deps.Search.Search)
	search.Get("/suggest", deps.Search.Suggest)
}
