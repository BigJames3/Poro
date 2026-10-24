package routes

import (
	"github.com/gofiber/fiber/v2"

	"github.com/poro/shared-go/health"
	"github.com/poro/shared-go/jwtauth"
	"github.com/poro/shared-go/ratelimit"

	"github.com/poro/video/internal/handler"
	"github.com/poro/video/internal/middleware"
)

// Dependencies are the handlers the router needs.
type Dependencies struct {
	Videos  *handler.Videos
	Health  *health.Handler
	Metrics fiber.Handler
	JWT     *jwtauth.Verifier
	Limiter *ratelimit.Limiter
	// PhotoPostsEnabled opens photo post creation and completion (ADR-0020).
	PhotoPostsEnabled bool
}

// Register mounts probes, metrics and /api/v1/videos.
func Register(app *fiber.App, deps Dependencies) {
	deps.Health.Register(app)
	app.Get("/metrics", deps.Metrics)

	auth := jwtauth.Middleware(deps.JWT)
	opt := jwtauth.OptionalMiddleware(deps.JWT)
	limit := middleware.GeneralLimit(deps.Limiter)

	// The limit follows the JWT middleware so signed-in callers are counted per
	// account, not per carrier NAT address.
	v := app.Group("/api/v1/videos")
	v.Get("/", auth, limit, deps.Videos.List)
	v.Post("/uploads", auth, limit, middleware.InitLimit(deps.Limiter), deps.Videos.Init)
	v.Post("/uploads/:id/complete", auth, limit, middleware.CompleteLimit(deps.Limiter), deps.Videos.Complete)
	// Photo posts (ADR-0020) count against the same init and complete limits.
	// Until PHOTO_POSTS_ENABLED they answer 404 photo_posts_disabled; aborting
	// an upload and reading stay open, so nothing already stored is stranded.
	photos := middleware.PhotoPosts(deps.PhotoPostsEnabled)
	v.Post("/photo-uploads", photos, auth, limit, middleware.InitLimit(deps.Limiter), deps.Videos.InitPhoto)
	v.Post("/photo-uploads/:id/complete", photos, auth, limit, middleware.CompleteLimit(deps.Limiter), deps.Videos.CompletePhoto)
	v.Delete("/uploads/:id", auth, limit, deps.Videos.Abort)
	v.Get("/:id", opt, limit, deps.Videos.Get)
	v.Delete("/:id", auth, limit, deps.Videos.Delete)
}
