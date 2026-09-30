// Package routes registers the auth HTTP routes.
package routes

import (
	"github.com/gofiber/fiber/v2"

	"github.com/poro/auth/internal/handler"
	"github.com/poro/auth/internal/middleware"
	"github.com/poro/auth/internal/service"
)

// Dependencies are the handlers and middleware the router needs.
// A nil Limiter keeps rate-limit counters in process memory.
type Dependencies struct {
	Auth    *handler.AuthHandler
	Health  *handler.HealthHandler
	JWKS    *handler.JWKSHandler
	Tokens  service.TokenService
	Limiter fiber.Storage
}

// Register mounts the probes, the JWKS document and the /api/v1/auth routes.
func Register(app *fiber.App, deps Dependencies) {
	app.Get("/health", deps.Health.Ready)
	app.Get("/health/live", deps.Health.Live)
	app.Get("/health/ready", deps.Health.Ready)
	app.Get("/.well-known/jwks.json", deps.JWKS.JWKS)

	auth := app.Group("/api/v1/auth", middleware.GeneralLimit(deps.Limiter))
	auth.Post("/otp/request", middleware.OTPRequestLimit(deps.Limiter), deps.Auth.RequestOTP)
	auth.Post("/otp/verify", middleware.OTPVerifyLimit(deps.Limiter), deps.Auth.VerifyOTP)
	auth.Post("/email/register", middleware.LoginLimit(deps.Limiter), deps.Auth.RegisterEmail)
	auth.Post("/email/login", middleware.LoginLimit(deps.Limiter), deps.Auth.LoginEmail)
	auth.Post("/refresh", deps.Auth.Refresh)

	protected := auth.Group("", middleware.NewAuth(deps.Tokens))
	protected.Post("/logout", deps.Auth.Logout)
	protected.Get("/me", deps.Auth.Me)
}
