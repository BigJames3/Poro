package main

import (
	"context"
	"fmt"
	"os"
	"os/signal"
	"syscall"
	"time"

	"github.com/gofiber/fiber/v2"
	"github.com/gofiber/fiber/v2/middleware/helmet"
	"github.com/gofiber/fiber/v2/middleware/recover"
	"go.uber.org/zap"

	"github.com/poro/auth/internal/config"
	"github.com/poro/auth/internal/database"
	"github.com/poro/auth/internal/handler"
	"github.com/poro/auth/internal/middleware"
	"github.com/poro/auth/internal/repository"
	"github.com/poro/auth/internal/routes"
	"github.com/poro/auth/internal/service"
)

const version = "1.1.0"

func main() {
	if err := run(); err != nil {
		fmt.Fprintf(os.Stderr, "auth server: %v\n", err)
		os.Exit(1)
	}
}

// run fails fast when a dependency is missing: the orchestrator restarts the
// process instead of serving requests that cannot succeed.
func run() error {
	cfg, err := config.Load()
	if err != nil {
		return fmt.Errorf("load config: %w", err)
	}
	log, err := newLogger(cfg)
	if err != nil {
		return fmt.Errorf("init logger: %w", err)
	}
	defer func() { _ = log.Sync() }()

	ctx := context.Background()
	pool, err := database.NewPostgresPool(ctx, cfg, log)
	if err != nil {
		return err
	}
	defer pool.Close()
	if err := database.RunMigrations(ctx, cfg, log); err != nil {
		return fmt.Errorf("run migrations: %w", err)
	}
	rdb, err := database.NewRedisClient(ctx, cfg, log)
	if err != nil {
		return err
	}
	defer func() { _ = rdb.Close() }()

	tokens, err := service.NewTokenService(cfg, rdb)
	if err != nil {
		return fmt.Errorf("init token service: %w", err)
	}
	throttle, err := service.NewRedisOTPThrottle(cfg, rdb)
	if err != nil {
		return fmt.Errorf("init otp throttle: %w", err)
	}
	sender, err := service.NewSMSSender(cfg, log)
	if err != nil {
		return fmt.Errorf("init sms sender: %w", err)
	}
	otp, err := service.NewOTPService(cfg, repository.NewOTPRepository(pool), throttle, sender, log)
	if err != nil {
		return fmt.Errorf("init otp service: %w", err)
	}
	authSvc, err := service.NewAuthService(
		cfg,
		repository.NewUserRepository(pool),
		repository.NewRefreshTokenRepository(pool),
		otp,
		tokens,
		log,
	)
	if err != nil {
		return fmt.Errorf("init auth service: %w", err)
	}
	limiter, err := middleware.NewRedisStorage(cfg)
	if err != nil {
		return fmt.Errorf("init rate limiter: %w", err)
	}

	app := newApp(cfg, log)
	routes.Register(app, routes.Dependencies{
		Auth:    handler.NewAuthHandler(authSvc),
		Health:  handler.NewHealthHandler(pool, rdb, version),
		JWKS:    handler.NewJWKSHandler(tokens),
		Tokens:  tokens,
		Limiter: limiter,
	})

	addr := ":" + cfg.AppPort
	errCh := make(chan error, 1)
	go func() { errCh <- app.Listen(addr) }()
	log.Info("auth service started", zap.String("port", cfg.AppPort), zap.String("env", cfg.AppEnv), zap.String("version", version))

	stop := make(chan os.Signal, 1)
	signal.Notify(stop, syscall.SIGINT, syscall.SIGTERM)
	select {
	case err := <-errCh:
		if err != nil {
			return fmt.Errorf("listen %s: %w", addr, err)
		}
		return nil
	case <-stop:
		shutdownCtx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
		defer cancel()
		if err := app.ShutdownWithContext(shutdownCtx); err != nil {
			return fmt.Errorf("shutdown: %w", err)
		}
		log.Info("auth service stopped")
		return nil
	}
}

func newApp(cfg *config.Config, log *zap.Logger) *fiber.App {
	app := fiber.New(fiber.Config{
		AppName:                 "poro-auth",
		DisableStartupMessage:   true,
		ReadTimeout:             10 * time.Second,
		WriteTimeout:            10 * time.Second,
		IdleTimeout:             30 * time.Second,
		BodyLimit:               64 * 1024,
		ErrorHandler:            middleware.ErrorHandler(log),
		ProxyHeader:             proxyHeader(cfg),
		EnableTrustedProxyCheck: len(cfg.TrustedProxies) > 0,
		EnableIPValidation:      true,
		TrustedProxies:          cfg.TrustedProxies,
	})
	app.Use(
		middleware.RequestID(),
		middleware.AccessLog(log),
		recover.New(),
		helmet.New(),
	)
	return app
}

// proxyHeader reads the client IP from X-Forwarded-For only behind trusted proxies.
// Without them the header is attacker-controlled and would defeat IP rate limits.
func proxyHeader(cfg *config.Config) string {
	if len(cfg.TrustedProxies) == 0 {
		return ""
	}
	return fiber.HeaderXForwardedFor
}

func newLogger(cfg *config.Config) (*zap.Logger, error) {
	zc := zap.NewProductionConfig()
	if cfg.IsDev() {
		zc = zap.NewDevelopmentConfig()
	}
	if err := zc.Level.UnmarshalText([]byte(cfg.LogLevel)); err != nil {
		return nil, fmt.Errorf("parse log level %q: %w", cfg.LogLevel, err)
	}
	log, err := zc.Build()
	if err != nil {
		return nil, fmt.Errorf("build logger: %w", err)
	}
	return log, nil
}
