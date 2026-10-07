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
	"github.com/poro/shared-go/tracing"

	"github.com/poro/feed/internal/config"
	"github.com/poro/feed/internal/consumer"
	"github.com/poro/feed/internal/database"
	"github.com/poro/feed/internal/handler"
	"github.com/poro/feed/internal/middleware"
	"github.com/poro/feed/internal/model"
	"github.com/poro/feed/internal/repository"
	"github.com/poro/feed/internal/routes"
	"github.com/poro/feed/internal/service"
	"github.com/poro/feed/internal/session"
	"github.com/poro/feed/internal/trending"
)

const version = "1.0.0"

func main() {
	if err := run(); err != nil {
		fmt.Fprintf(os.Stderr, "feed server: %v\n", err)
		os.Exit(1)
	}
}

func run() error {
	cfg, err := config.Load()
	if err != nil {
		return fmt.Errorf("load config: %w", err)
	}
	log, err := logging.New("feed", cfg.LogLevel, cfg.IsDev())
	if err != nil {
		return fmt.Errorf("init logger: %w", err)
	}
	defer func() { _ = log.Sync() }()

	ctx, stop := signal.NotifyContext(context.Background(), syscall.SIGINT, syscall.SIGTERM)
	defer stop()

	shutdownTracing, err := tracing.Setup(ctx, tracing.Config{Service: "feed", Version: version, Endpoint: cfg.OtelEndpoint})
	if err != nil {
		return fmt.Errorf("init tracing: %w", err)
	}
	defer func() { _ = shutdownTracing(context.Background()) }()
	m, err := metrics.New("feed")
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
	redis, err := middleware.NewRedisStorage(cfg)
	if err != nil {
		return fmt.Errorf("init redis: %w", err)
	}
	defer func() { _ = redis.Close() }()

	projector, err := kafka.NewConsumer(kafka.ConsumerConfig{
		Brokers: cfg.KafkaBrokers,
		Group:   consumer.Group,
		Topics:  consumer.Topics,
	}, consumer.NewProjector(pool, log), log)
	if err != nil {
		return fmt.Errorf("init projector: %w", err)
	}

	svc := service.New(pool,
		session.NewStore(redis.Conn(), model.SessionTTL),
		session.NewCache(redis.Conn(), model.FirstPageTTL),
		cfg.PublicObjectURL, log)

	app := fiber.New(fiber.Config{
		AppName:                 "poro-feed",
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
		Feed: handler.New(svc),
		// Kafka is left out: feeds keep serving the last projected state.
		Health: health.New("feed", version, map[string]health.Check{
			"postgres": pool.Ping,
			"redis":    func(ctx context.Context) error { return redis.Conn().Ping(ctx).Err() },
		}),
		Metrics: m.Handler(),
		JWT:     jwt,
		Limiter: redis,
	})

	var background sync.WaitGroup
	background.Add(2)
	go func() {
		defer background.Done()
		_ = projector.Run(ctx)
	}()
	go func() {
		defer background.Done()
		trending.Run(ctx, model.TrendingInterval, func(ctx context.Context) (int64, error) {
			return repository.RescoreAll(ctx, pool)
		}, log)
	}()
	backgroundDone := make(chan struct{})
	go func() { background.Wait(); close(backgroundDone) }()

	addr := ":" + cfg.AppPort
	errCh := make(chan error, 1)
	go func() { errCh <- app.Listen(addr) }()
	log.Info("feed service started", zap.String("port", cfg.AppPort), zap.String("env", cfg.AppEnv), zap.String("version", version))

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
	case <-backgroundDone:
	case <-shutdownCtx.Done():
	}
	log.Info("feed service stopped")
	return listenErr
}

func proxyHeader(cfg *config.Config) string {
	if len(cfg.TrustedProxies) == 0 {
		return ""
	}
	return fiber.HeaderXForwardedFor
}
