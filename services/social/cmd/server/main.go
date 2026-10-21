package main

import (
	"context"
	"fmt"
	"os"
	"os/signal"
	"sync"
	"syscall"
	"time"

	"github.com/gofiber/fiber/v2"
	"github.com/gofiber/fiber/v2/middleware/helmet"
	"github.com/gofiber/fiber/v2/middleware/recover"
	"go.uber.org/zap"

	"github.com/poro/shared-go/health"
	"github.com/poro/shared-go/hiding"
	"github.com/poro/shared-go/httpx"
	"github.com/poro/shared-go/jwtauth"
	"github.com/poro/shared-go/kafka"
	"github.com/poro/shared-go/logging"
	"github.com/poro/shared-go/metrics"
	"github.com/poro/shared-go/revocation"
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

	// Revoked sessions are refused (ADR-0016). Only a dev service started on
	// its own may run without the registry.
	var revocations jwtauth.Revocations
	revocationCheck := func(context.Context) error { return nil }
	if cfg.RevocationRedisURL != "" {
		registry, err := revocation.Open(cfg.RevocationRedisURL)
		if err != nil {
			return err
		}
		defer func() { _ = registry.Close() }()
		revocations, revocationCheck = registry, registry.Ping
	} else {
		log.Warn("REVOCATION_REDIS_URL is not set: revoked sessions stay accepted until their tokens expire")
	}
	jwt, err := jwtauth.NewVerifier(jwtauth.Config{JWKSURL: cfg.JWKSURL, Revocations: revocations})
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
		Metrics: m.Registry(),
	}, consumer.NewProjections(pool, log), log)
	if err != nil {
		return fmt.Errorf("init projections consumer: %w", err)
	}
	erasure, err := kafka.NewConsumer(kafka.ConsumerConfig{
		Brokers: cfg.KafkaBrokers,
		Group:   consumer.ErasureGroup,
		Topics:  consumer.ErasureTopics,
		Metrics: m.Registry(),
	}, consumer.NewErasure(pool, log), log)
	if err != nil {
		return fmt.Errorf("init account erasure consumer: %w", err)
	}
	hider, err := kafka.NewConsumer(kafka.ConsumerConfig{
		Brokers: cfg.KafkaBrokers,
		Group:   consumer.HidingGroup,
		Topics:  hiding.Topics,
		Metrics: m.Registry(),
	}, hiding.Handler(pool), log)
	if err != nil {
		return fmt.Errorf("init account hiding consumer: %w", err)
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
			"postgres":   pool.Ping,
			"revocation": revocationCheck,
			"redis":      func(ctx context.Context) error { return limiter.Conn().Ping(ctx).Err() },
		}),
		Metrics: m.Handler(),
		JWT:     jwt,
		Limiter: limiter,
	})

	consumerDone := make(chan struct{})
	go func() {
		defer close(consumerDone)
		var wg sync.WaitGroup
		for _, c := range []*kafka.Consumer{projections, erasure, hider} {
			wg.Add(1)
			go func() {
				defer wg.Done()
				_ = c.Run(ctx)
			}()
		}
		wg.Wait()
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
