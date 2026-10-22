// Package routes registers the auth HTTP routes.
package routes

import (
	"github.com/gofiber/fiber/v2"

	"github.com/poro/shared-go/health"

	"github.com/poro/auth/internal/handler"
	"github.com/poro/auth/internal/middleware"
	"github.com/poro/auth/internal/service"
)

// Dependencies are the handlers and middleware the router needs.
// The Limiter fails closed: with Redis down the limited routes answer 503.
type Dependencies struct {
	Auth     *handler.AuthHandler
	Account  *handler.AccountHandler
	Email    *handler.EmailHandler
	Health   *health.Handler
	Metrics  fiber.Handler
	JWKS     *handler.JWKSHandler
	Tokens   service.TokenService
	Accounts middleware.AccountLookup
	Limiter  *middleware.Limiter
}

// Register mounts the probes, metrics, the JWKS document and the /api/v1/auth routes.
// /metrics must not be exposed by the public gateway.
func Register(app *fiber.App, deps Dependencies) {
	deps.Health.Register(app)
	app.Get("/metrics", deps.Metrics)
	app.Get("/.well-known/jwks.json", deps.JWKS.JWKS)

	// Public routes are counted per IP. Signed-in routes are counted per account
	// once the token is checked: behind carrier-grade NAT thousands of phones
	// share one IP, and the gateway already caps floods per IP.
	general := middleware.GeneralLimit(deps.Limiter)
	auth := app.Group("/api/v1/auth")
	auth.Post("/otp/request", general, middleware.OTPRequestLimit(deps.Limiter), deps.Auth.RequestOTP)
	auth.Post("/otp/verify", general, middleware.OTPVerifyLimit(deps.Limiter), deps.Auth.VerifyOTP)
	auth.Post("/email/register", general, middleware.LoginLimit(deps.Limiter), deps.Auth.RegisterEmail)
	auth.Post("/email/login", general, middleware.LoginLimit(deps.Limiter), deps.Auth.LoginEmail)
	auth.Post("/refresh", middleware.RefreshLimit(deps.Limiter), deps.Auth.Refresh)
	// Both send or check a code for an address: limited like a sign-in.
	auth.Post("/email/password/forgot", general, middleware.LoginLimit(deps.Limiter), deps.Email.ForgotPassword)
	auth.Post("/email/password/reset", general, middleware.LoginLimit(deps.Limiter), deps.Email.ResetPassword)

	protected := auth.Group("", middleware.NewAuth(deps.Tokens, deps.Accounts), middleware.AccountLimit(deps.Limiter))
	protected.Post("/logout", deps.Auth.Logout)
	protected.Get("/me", deps.Auth.Me)
	protected.Get("/sessions", deps.Account.Sessions)
	protected.Post("/sessions/revoke-others", deps.Account.RevokeOtherSessions)
	protected.Delete("/sessions/:id", deps.Account.RevokeSession)
	// The deletion checks a password or an OTP code: limited like a sign-in.
	protected.Post("/me/delete", middleware.AccountLoginLimit(deps.Limiter), deps.Account.DeleteAccount)
	protected.Post("/me/delete/cancel", deps.Account.CancelDeletion)
	protected.Post("/email/verification/request", middleware.AccountLoginLimit(deps.Limiter), deps.Email.RequestVerification)
	protected.Post("/email/verification/confirm", middleware.AccountLoginLimit(deps.Limiter), deps.Email.ConfirmVerification)
	// ADMIN or MODERATOR, checked by the service: blocks or restores an account everywhere (ADR-0019).
	protected.Post("/admin/users/:id/suspend", deps.Account.SuspendAccount)
	protected.Post("/admin/users/:id/reinstate", deps.Account.ReinstateAccount)
}
