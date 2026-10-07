// Package routes registers the social HTTP routes.
package routes

import (
	"github.com/gofiber/fiber/v2"

	"github.com/poro/shared-go/health"
	"github.com/poro/shared-go/jwtauth"

	"github.com/poro/social/internal/handler"
	"github.com/poro/social/internal/middleware"
)

// Dependencies are the handlers and middleware the router needs.
type Dependencies struct {
	Social  *handler.Social
	Health  *health.Handler
	Metrics fiber.Handler
	JWT     *jwtauth.Verifier
	Limiter fiber.Storage
}

// Register mounts the probes (at the root and under /api/v1), the metrics and
// the /api/v1 social routes. /metrics must not be exposed by the public gateway.
func Register(app *fiber.App, deps Dependencies) {
	deps.Health.Register(app)
	deps.Health.Register(app.Group("/api/v1"))
	app.Get("/metrics", deps.Metrics)

	auth := jwtauth.Middleware(deps.JWT)
	opt := jwtauth.OptionalMiddleware(deps.JWT)
	general := middleware.GeneralLimit(deps.Limiter)
	like := middleware.LikeLimit(deps.Limiter)
	comment := middleware.CommentLimit(deps.Limiter)
	follow := middleware.FollowLimit(deps.Limiter)
	share := middleware.ShareLimit(deps.Limiter)
	h := deps.Social

	api := app.Group("/api/v1")

	api.Put("/videos/:videoId/like", auth, general, like, h.LikeVideo)
	api.Delete("/videos/:videoId/like", auth, general, like, h.UnlikeVideo)
	api.Get("/videos/:videoId/likes", opt, general, h.ListVideoLikes)
	api.Get("/videos/:videoId/stats", opt, general, h.VideoStats)
	api.Post("/videos/:videoId/comments", auth, general, comment, h.CreateComment)
	api.Get("/videos/:videoId/comments", opt, general, h.ListComments)
	api.Post("/videos/:videoId/shares", auth, general, share, h.Share)

	api.Get("/comments/:commentId/replies", opt, general, h.ListReplies)
	api.Patch("/comments/:commentId", auth, general, comment, h.EditComment)
	api.Delete("/comments/:commentId", auth, general, h.DeleteComment)
	api.Put("/comments/:commentId/like", auth, general, like, h.LikeComment)
	api.Delete("/comments/:commentId/like", auth, general, like, h.UnlikeComment)

	api.Get("/users/:userId/likes", opt, general, h.ListUserLikes)
	api.Get("/users/:userId/stats", opt, general, h.UserStats)
	api.Put("/users/:userId/follow", auth, general, follow, h.Follow)
	api.Delete("/users/:userId/follow", auth, general, follow, h.Unfollow)
	api.Get("/users/:userId/followers", opt, general, h.ListFollowers)
	api.Get("/users/:userId/following", opt, general, h.ListFollowing)
}
