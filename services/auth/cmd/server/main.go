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

	"github.com/poro/shared-go/events"
	"github.com/poro/shared-go/health"
	"github.com/poro/shared-go/httpx"
	"github.com/poro/shared-go/kafka"
	"github.com/poro/shared-go/logging"
	"github.com/poro/shared-go/metrics"
	"github.com/poro/shared-go/tracing"

	"github.com/poro/auth/internal/config"
	"github.com/poro/auth/internal/consumer"
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
	log, err := logging.New("auth", cfg.LogLevel, cfg.IsDev())
	if err != nil {
		return fmt.Errorf("init logger: %w", err)
	}
	defer func() { _ = log.Sync() }()

	ctx, stop := signal.NotifyContext(context.Background(), syscall.SIGINT, syscall.SIGTERM)
	defer stop()

	shutdownTracing, err := tracing.Setup(ctx, tracing.Config{Service: "auth", Version: version, Endpoint: cfg.OtelEndpoint})
	if err != nil {
		return fmt.Errorf("init tracing: %w", err)
	}
	defer func() { _ = shutdownTracing(context.Background()) }()
	m, err := metrics.New("auth")
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

	creators, err := kafka.NewConsumer(kafka.ConsumerConfig{
		Brokers: cfg.KafkaBrokers,
		Group:   consumer.CreatorRolesGroup,
		Topics:  []string{events.TypeUserCreatorActivated},
	}, consumer.NewCreatorActivated(pool, log), log)
	if err != nil {
		return fmt.Errorf("init creator consumer: %w", err)
	}
	businesses, err := kafka.NewConsumer(kafka.ConsumerConfig{
		Brokers: cfg.KafkaBrokers,
		Group:   consumer.BusinessRolesGroup,
		Topics:  []string{events.TypeShopShopCreated},
	}, consumer.NewShopCreated(pool, log), log)
	if err != nil {
		return fmt.Errorf("init business consumer: %w", err)
	}

	app := newApp(cfg, log, m)
	routes.Register(app, routes.Dependencies{
		Auth: handler.NewAuthHandler(authSvc),
		// Kafka is left out: sign-in keeps working while the broker is down.
		Health: health.New("auth", version, map[string]health.Check{
			"postgres": pool.Ping,
			"redis":    func(ctx context.Context) error { return rdb.Ping(ctx).Err() },
		}),
		Metrics: m.Handler(),
		JWKS:    handler.NewJWKSHandler(tokens),
		Tokens:  tokens,
		Limiter: limiter,
	})

	var consumers sync.WaitGroup
	for _, c := range []*kafka.Consumer{creators, businesses} {
		consumers.Add(1)
		go func() {
			defer consumers.Done()
			_ = c.Run(ctx)
		}()
	}
	consumerDone := make(chan struct{})
	go func() {
		consumers.Wait()
		close(consumerDone)
	}()

	addr := ":" + cfg.AppPort
	errCh := make(chan error, 1)
	go func() { errCh <- app.Listen(addr) }()
	log.Info("auth service started", zap.String("port", cfg.AppPort), zap.String("env", cfg.AppEnv), zap.String("version", version))

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
	log.Info("auth service stopped")
	return listenErr
}

func newApp(cfg *config.Config, log *zap.Logger, m *metrics.Metrics) *fiber.App {
	app := fiber.New(fiber.Config{
		AppName:                 "poro-auth",
		DisableStartupMessage:   true,
		ReadTimeout:             10 * time.Second,
		WriteTimeout:            10 * time.Second,
		IdleTimeout:             30 * time.Second,
		BodyLimit:               64 * 1024,
		ErrorHandler:            httpx.ErrorHandler(log),
		ProxyHeader:             proxyHeader(cfg),
		EnableTrustedProxyCheck: len(cfg.TrustedProxies) > 0,
		EnableIPValidation:      true,
		TrustedProxies:          cfg.TrustedProxies,
	})
	app.Use(
		httpx.RequestID(),
		tracing.Middleware(),
		m.Middleware(),
		httpx.AccessLog(log),
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
