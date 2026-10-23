// Package routes registers the feed HTTP routes.
package routes

import (
	"github.com/gofiber/fiber/v2"

	"github.com/poro/shared-go/health"
	"github.com/poro/shared-go/jwtauth"

	"github.com/poro/feed/internal/handler"
	"github.com/poro/feed/internal/middleware"
)

// Dependencies are the handlers and middleware the router needs.
type Dependencies struct {
	Feed    *handler.Feed
	Health  *health.Handler
	Metrics fiber.Handler
	JWT     *jwtauth.Verifier
	Limiter fiber.Storage
}

// Register mounts the probes (at the root and under /api/v1), the metrics and
// the authenticated /api/v1/feed routes. /metrics must not be exposed by the
// public gateway.
func Register(app *fiber.App, deps Dependencies) {
	deps.Health.Register(app)
	deps.Health.Register(app.Group("/api/v1"))
	app.Get("/metrics", deps.Metrics)

	feed := app.Group("/api/v1/feed")
	feed.Get("/for-you", jwtauth.Middleware(deps.JWT), middleware.GeneralLimit(deps.Limiter), deps.Feed.ForYou)
	feed.Get("/following", jwtauth.Middleware(deps.JWT), middleware.GeneralLimit(deps.Limiter), deps.Feed.Following)
	// Trending is public (ADR-0019). OptionalMiddleware still rejects a bad Bearer.
	// Middleware is per-route: a second Group on the same prefix would also run the JWT wall.
	feed.Get("/trending", jwtauth.OptionalMiddleware(deps.JWT), middleware.TrendingLimit(deps.Limiter), deps.Feed.Trending)
}
