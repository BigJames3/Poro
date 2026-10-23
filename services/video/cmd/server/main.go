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

	"github.com/poro/video/internal/config"
	"github.com/poro/video/internal/database"
	"github.com/poro/video/internal/erasure"
	"github.com/poro/video/internal/handler"
	"github.com/poro/video/internal/middleware"
	"github.com/poro/video/internal/model"
	"github.com/poro/video/internal/moderation"
	"github.com/poro/video/internal/repository"
	"github.com/poro/video/internal/routes"
	"github.com/poro/video/internal/service"
	"github.com/poro/video/internal/storage"
)

const version = "1.0.0"

// hidingGroup hides the videos of accounts whose deletion is pending (shared-go/hiding).
const hidingGroup = "poro-video-account-hiding"

func main() {
	if err := run(); err != nil {
		fmt.Fprintf(os.Stderr, "video server: %v\n", err)
		os.Exit(1)
	}
}

func run() error {
	cfg, err := config.Load()
	if err != nil {
		return fmt.Errorf("load config: %w", err)
	}
	log, err := logging.New("video", cfg.LogLevel, cfg.IsDev())
	if err != nil {
		return fmt.Errorf("init logger: %w", err)
	}
	defer func() { _ = log.Sync() }()

	ctx, stop := signal.NotifyContext(context.Background(), syscall.SIGINT, syscall.SIGTERM)
	defer stop()

	shutdownTracing, err := tracing.Setup(ctx, tracing.Config{Service: "video", Version: version, Endpoint: cfg.OtelEndpoint})
	if err != nil {
		return fmt.Errorf("init tracing: %w", err)
	}
	defer func() { _ = shutdownTracing(context.Background()) }()
	m, err := metrics.New("video")
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

	store, err := storage.NewS3(ctx, cfg)
	if err != nil {
		return fmt.Errorf("init s3: %w", err)
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

	repo := repository.New(pool)
	svc := service.NewVideos(cfg, repo, store, log)
	moderator, err := kafka.NewConsumer(kafka.ConsumerConfig{
		Brokers: cfg.KafkaBrokers,
		Group:   moderation.Group,
		Topics:  moderation.Topics,
		Metrics: m.Registry(),
	}, moderation.New(pool, repo, store, log).Handle, log)
	if err != nil {
		return fmt.Errorf("init moderation consumer: %w", err)
	}
	eraser, err := kafka.NewConsumer(kafka.ConsumerConfig{
		Brokers: cfg.KafkaBrokers,
		Group:   erasure.Group,
		Topics:  erasure.Topics,
		Metrics: m.Registry(),
	}, erasure.New(pool, repo, store, log).Handle, log)
	if err != nil {
		return fmt.Errorf("init account erasure consumer: %w", err)
	}

	hider, err := kafka.NewConsumer(kafka.ConsumerConfig{
		Brokers: cfg.KafkaBrokers,
		Group:   hidingGroup,
		Topics:  hiding.Topics,
		Metrics: m.Registry(),
	}, hiding.Handler(pool), log)
	if err != nil {
		return fmt.Errorf("init account hiding consumer: %w", err)
	}

	app := fiber.New(fiber.Config{
		AppName:                 "poro-video",
		DisableStartupMessage:   true,
		ReadTimeout:             15 * time.Second,
		WriteTimeout:            15 * time.Second,
		IdleTimeout:             30 * time.Second,
		BodyLimit:               64 * 1024,
		ErrorHandler:            httpx.ErrorHandler(log),
		ProxyHeader:             proxyHeader(cfg),
		EnableTrustedProxyCheck: len(cfg.TrustedProxies) > 0,
		EnableIPValidation:      true,
		TrustedProxies:          cfg.TrustedProxies,
	})
	app.Use(httpx.RequestID(), tracing.Middleware(), m.Middleware(), httpx.AccessLog(log), recover.New(), helmet.New())

	routes.Register(app, routes.Dependencies{
		Videos: handler.NewVideos(svc),
		Health: health.New("video", version, map[string]health.Check{
			"postgres":   pool.Ping,
			"revocation": revocationCheck,
		}),
		Metrics: m.Handler(),
		JWT:     jwt,
		Limiter: middleware.NewLimiter(limiter),
	})

	consumerDone := make(chan struct{})
	go func() {
		defer close(consumerDone)
		var wg sync.WaitGroup
		for _, c := range []*kafka.Consumer{moderator, eraser, hider} {
			wg.Add(1)
			go func() {
				defer wg.Done()
				_ = c.Run(ctx)
			}()
		}
		wg.Add(1)
		go func() {
			defer wg.Done()
			sweepAbandonedUploads(ctx, svc, log)
		}()
		wg.Wait()
	}()

	addr := ":" + cfg.AppPort
	errCh := make(chan error, 1)
	go func() { errCh <- app.Listen(addr) }()
	log.Info("video service started", zap.String("port", cfg.AppPort), zap.String("env", cfg.AppEnv), zap.String("version", version))

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
	log.Info("video service stopped")
	return listenErr
}

// sweepAbandonedUploads discards, every UploadSweepInterval, the uploads left
// in `uploading` for longer than AbandonedUploadAge (PORO-008). Every replica
// may run it: each discard is conditional on the row still uploading.
func sweepAbandonedUploads(ctx context.Context, svc *service.Videos, log *zap.Logger) {
	ticker := time.NewTicker(model.UploadSweepInterval)
	defer ticker.Stop()
	for {
		cutoff := time.Now().Add(-model.AbandonedUploadAge)
		// A full batch means more may be waiting: keep going until a short one.
		for total := 0; ; {
			n, err := svc.SweepAbandonedUploads(ctx, cutoff, model.UploadSweepBatch)
			total += n
			if err != nil && ctx.Err() == nil {
				log.Error("sweep abandoned uploads", zap.Error(err))
			}
			if err != nil || n < model.UploadSweepBatch {
				if total > 0 {
					log.Info("abandoned uploads discarded", zap.Int("count", total))
				}
				break
			}
		}
		select {
		case <-ctx.Done():
			return
		case <-ticker.C:
		}
	}
}

func proxyHeader(cfg *config.Config) string {
	if len(cfg.TrustedProxies) == 0 {
		return ""
	}
	return fiber.HeaderXForwardedFor
}
