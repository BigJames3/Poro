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

	"github.com/poro/shared-go/health"
	"github.com/poro/shared-go/httpx"
	"github.com/poro/shared-go/jwtauth"
	"github.com/poro/shared-go/kafka"
	"github.com/poro/shared-go/logging"
	"github.com/poro/shared-go/metrics"
	"github.com/poro/shared-go/tracing"

	"github.com/poro/social/internal/config"
	"github.com/poro/social/internal/consumer"
	"github.com/poro/social/internal/database"
	"github.com/poro/social/internal/handler"
	"github.com/poro/social/internal/middleware"
	"github.com/poro/social/internal/routes"
	"github.com/poro/social/internal/service"
)

const version = "1.0.0"

func main() {
	if err := run(); err != nil {
		fmt.Fprintf(os.Stderr, "social server: %v\n", err)
		os.Exit(1)
	}
}

func run() error {
	cfg, err := config.Load()
	if err != nil {
		return fmt.Errorf("load config: %w", err)
	}
	log, err := logging.New("social", cfg.LogLevel, cfg.IsDev())
	if err != nil {
		return fmt.Errorf("init logger: %w", err)
	}
	defer func() { _ = log.Sync() }()

	ctx, stop := signal.NotifyContext(context.Background(), syscall.SIGINT, syscall.SIGTERM)
	defer stop()

	shutdownTracing, err := tracing.Setup(ctx, tracing.Config{Service: "social", Version: version, Endpoint: cfg.OtelEndpoint})
	if err != nil {
		return fmt.Errorf("init tracing: %w", err)
	}
	defer func() { _ = shutdownTracing(context.Background()) }()
	m, err := metrics.New("social")
	if err != nil {
		return fmt.Errorf("init metrics: %w", err)
	}

	pool, err := database.NewPostgresPool(ctx, cfg, log)
	if err != nil {
		return err
	}
	defer pool.Close()
	if err := database.RunMigrations(ctx, cfg, log); err != nil {
		return fmt.Errorf("run migrations: %w", err)
	}

	jwt, err := jwtauth.NewVerifier(jwtauth.Config{JWKSURL: cfg.JWKSURL})
	if err != nil {
		return fmt.Errorf("init jwks: %w", err)
	}
	limiter, err := middleware.NewRedisStorage(cfg)
	if err != nil {
		return fmt.Errorf("init rate limiter: %w", err)
	}
	defer func() { _ = limiter.Close() }()

	projections, err := kafka.NewConsumer(kafka.ConsumerConfig{
		Brokers: cfg.KafkaBrokers,
		Group:   consumer.Group,
		Topics:  consumer.Topics,
	}, consumer.NewProjections(pool, log), log)
	if err != nil {
		return fmt.Errorf("init projections consumer: %w", err)
	}

	app := fiber.New(fiber.Config{
		AppName:                 "poro-social",
		DisableStartupMessage:   true,
		ReadTimeout:             10 * time.Second,
		WriteTimeout:            10 * time.Second,
		IdleTimeout:             30 * time.Second,
		BodyLimit:               16 * 1024,
		ErrorHandler:            httpx.ErrorHandler(log),
		ProxyHeader:             proxyHeader(cfg),
		EnableTrustedProxyCheck: len(cfg.TrustedProxies) > 0,
		EnableIPValidation:      true,
		TrustedProxies:          cfg.TrustedProxies,
	})
	app.Use(httpx.RequestID(), tracing.Middleware(), m.Middleware(), httpx.AccessLog(log), recover.New(), helmet.New())

	routes.Register(app, routes.Dependencies{
		Social: handler.New(service.New(pool, log)),
		// Kafka is left out: likes and comments keep working while the broker is
		// down, their events wait in the outbox.
		Health: health.New("social", version, map[string]health.Check{
			"postgres": pool.Ping,
			"redis":    func(ctx context.Context) error { return limiter.Conn().Ping(ctx).Err() },
		}),
		Metrics: m.Handler(),
		JWT:     jwt,
		Limiter: limiter,
	})

	consumerDone := make(chan struct{})
	go func() {
		defer close(consumerDone)
		_ = projections.Run(ctx)
	}()

	addr := ":" + cfg.AppPort
	errCh := make(chan error, 1)
	go func() { errCh <- app.Listen(addr) }()
	log.Info("social service started", zap.String("port", cfg.AppPort), zap.String("env", cfg.AppEnv), zap.String("version", version))

	var listenErr error
	select {
	case err := <-errCh:
		if err != nil {
			listenErr = fmt.Errorf("listen %s: %w", addr, err)
		}
		stop()
	case <-ctx.Done():
	}
	shutdownCtx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()
	if err := app.ShutdownWithContext(shutdownCtx); err != nil && listenErr == nil {
		listenErr = fmt.Errorf("shutdown: %w", err)
	}
	select {
	case <-consumerDone:
	case <-shutdownCtx.Done():
	}
	log.Info("social service stopped")
	return listenErr
}

func proxyHeader(cfg *config.Config) string {
	if len(cfg.TrustedProxies) == 0 {
		return ""
	}
	return fiber.HeaderXForwardedFor
}
