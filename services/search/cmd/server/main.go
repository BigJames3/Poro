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
	"github.com/poro/shared-go/httpx"
	"github.com/poro/shared-go/jwtauth"
	"github.com/poro/shared-go/kafka"
	"github.com/poro/shared-go/logging"
	"github.com/poro/shared-go/metrics"
	"github.com/poro/shared-go/revocation"
	"github.com/poro/shared-go/tracing"

	"github.com/poro/search/internal/config"
	"github.com/poro/search/internal/consumer"
	"github.com/poro/search/internal/database"
	"github.com/poro/search/internal/handler"
	"github.com/poro/search/internal/index"
	"github.com/poro/search/internal/middleware"
	"github.com/poro/search/internal/routes"
	"github.com/poro/search/internal/service"
)

const version = "1.0.0"

func main() {
	if err := run(); err != nil {
		fmt.Fprintf(os.Stderr, "search server: %v\n", err)
		os.Exit(1)
	}
}

func run() error {
	cfg, err := config.Load()
	if err != nil {
		return fmt.Errorf("load config: %w", err)
	}
	log, err := logging.New("search", cfg.LogLevel, cfg.IsDev())
	if err != nil {
		return fmt.Errorf("init logger: %w", err)
	}
	defer func() { _ = log.Sync() }()

	ctx, stop := signal.NotifyContext(context.Background(), syscall.SIGINT, syscall.SIGTERM)
	defer stop()

	shutdownTracing, err := tracing.Setup(ctx, tracing.Config{Service: "search", Version: version, Endpoint: cfg.OtelEndpoint})
	if err != nil {
		return fmt.Errorf("init tracing: %w", err)
	}
	defer func() { _ = shutdownTracing(context.Background()) }()
	m, err := metrics.New("search")
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
	redis, err := middleware.NewRedisStorage(cfg)
	if err != nil {
		return fmt.Errorf("init redis: %w", err)
	}
	defer func() { _ = redis.Close() }()

	indexer, err := kafka.NewConsumer(kafka.ConsumerConfig{
		Brokers: cfg.KafkaBrokers,
		Group:   consumer.Group,
		Topics:  consumer.Topics,
		Metrics: m.Registry(),
	}, consumer.NewIndexer(pool, log), log)
	if err != nil {
		return fmt.Errorf("init indexer: %w", err)
	}
	eraser, err := kafka.NewConsumer(kafka.ConsumerConfig{
		Brokers: cfg.KafkaBrokers,
		Group:   consumer.ErasureGroup,
		Topics:  consumer.ErasureTopics,
		Metrics: m.Registry(),
	}, consumer.NewEraser(pool, log), log)
	if err != nil {
		return fmt.Errorf("init account erasure: %w", err)
	}

	svc := service.New(index.NewPostgres(pool), cfg.PublicObjectURL)

	app := fiber.New(fiber.Config{
		AppName:                 "poro-search",
		DisableStartupMessage:   true,
		ReadTimeout:             10 * time.Second,
		WriteTimeout:            10 * time.Second,
		IdleTimeout:             30 * time.Second,
		BodyLimit:               4 * 1024,
		ErrorHandler:            httpx.ErrorHandler(log),
		ProxyHeader:             proxyHeader(cfg),
		EnableTrustedProxyCheck: len(cfg.TrustedProxies) > 0,
		EnableIPValidation:      true,
		TrustedProxies:          cfg.TrustedProxies,
	})
	app.Use(httpx.RequestID(), tracing.Middleware(), m.Middleware(), httpx.AccessLog(log), recover.New(), helmet.New())

	routes.Register(app, routes.Dependencies{
		Search: handler.New(svc),
		// Kafka is left out: search keeps serving the last indexed state.
		Health: health.New("search", version, map[string]health.Check{
			"postgres":   pool.Ping,
			"revocation": revocationCheck,
			"redis":      func(ctx context.Context) error { return redis.Conn().Ping(ctx).Err() },
		}),
		Metrics: m.Handler(),
		JWT:     jwt,
		Limiter: middleware.NewLimiter(redis),
	})

	var consumers sync.WaitGroup
	consumers.Add(2)
	go func() {
		defer consumers.Done()
		_ = indexer.Run(ctx)
	}()
	go func() {
		defer consumers.Done()
		_ = eraser.Run(ctx)
	}()
	indexerDone := make(chan struct{})
	go func() { consumers.Wait(); close(indexerDone) }()

	addr := ":" + cfg.AppPort
	errCh := make(chan error, 1)
	go func() { errCh <- app.Listen(addr) }()
	log.Info("search service started", zap.String("port", cfg.AppPort), zap.String("env", cfg.AppEnv), zap.String("version", version))

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
	case <-indexerDone:
	case <-shutdownCtx.Done():
	}
	log.Info("search service stopped")
	return listenErr
}

func proxyHeader(cfg *config.Config) string {
	if len(cfg.TrustedProxies) == 0 {
		return ""
	}
	return fiber.HeaderXForwardedFor
}
