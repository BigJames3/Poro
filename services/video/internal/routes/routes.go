package routes

import (
	"github.com/gofiber/fiber/v2"

	"github.com/poro/shared-go/health"
	"github.com/poro/shared-go/jwtauth"

	"github.com/poro/video/internal/handler"
	"github.com/poro/video/internal/middleware"
)

// Dependencies are the handlers the router needs.
type Dependencies struct {
	Videos  *handler.Videos
	Health  *health.Handler
	Metrics fiber.Handler
	JWT     *jwtauth.Verifier
	Limiter fiber.Storage
}

// Register mounts probes, metrics and /api/v1/videos.
func Register(app *fiber.App, deps Dependencies) {
	deps.Health.Register(app)
	app.Get("/metrics", deps.Metrics)

	auth := jwtauth.Middleware(deps.JWT)
	opt := jwtauth.OptionalMiddleware(deps.JWT)
	limit := middleware.GeneralLimit(deps.Limiter)

	v := app.Group("/api/v1/videos", limit)
	v.Get("/", auth, deps.Videos.List)
	v.Post("/uploads", auth, middleware.InitLimit(deps.Limiter), deps.Videos.Init)
	v.Post("/uploads/:id/complete", auth, middleware.CompleteLimit(deps.Limiter), deps.Videos.Complete)
	v.Delete("/uploads/:id", auth, deps.Videos.Abort)
	v.Get("/:id", opt, deps.Videos.Get)
	v.Delete("/:id", auth, deps.Videos.Delete)
}
