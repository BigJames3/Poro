package main

import (
	"context"
	"fmt"
	"os"
	"os/signal"
	"syscall"

	"go.uber.org/zap"

	"github.com/poro/shared-go/events"
	"github.com/poro/shared-go/kafka"
	"github.com/poro/shared-go/logging"
	"github.com/poro/shared-go/tracing"

	"github.com/poro/video/internal/config"
	"github.com/poro/video/internal/database"
	"github.com/poro/video/internal/repository"
	"github.com/poro/video/internal/storage"
	"github.com/poro/video/internal/transcode"
	"github.com/poro/video/internal/worker"
)

const version = "1.0.0"

func main() {
	if err := run(); err != nil {
		fmt.Fprintf(os.Stderr, "media-worker: %v\n", err)
		os.Exit(1)
	}
}

func run() error {
	cfg, err := config.Load()
	if err != nil {
		return fmt.Errorf("load config: %w", err)
	}
	log, err := logging.New("media-worker", cfg.LogLevel, cfg.IsDev())
	if err != nil {
		return fmt.Errorf("init logger: %w", err)
	}
	defer func() { _ = log.Sync() }()

	ctx, stop := signal.NotifyContext(context.Background(), syscall.SIGINT, syscall.SIGTERM)
	defer stop()

	shutdownTracing, err := tracing.Setup(ctx, tracing.Config{Service: "media-worker", Version: version, Endpoint: cfg.OtelEndpoint})
	if err != nil {
		return fmt.Errorf("init tracing: %w", err)
	}
	defer func() { _ = shutdownTracing(context.Background()) }()

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
	h := worker.New(pool, repository.New(pool), store, transcode.NewFFmpeg(cfg.FFmpegBin, cfg.FFprobeBin), log)
	consumer, err := kafka.NewConsumer(kafka.ConsumerConfig{
		Brokers: cfg.KafkaBrokers,
		Group:   worker.Group,
		Topics:  []string{events.TypeVideoUploaded},
	}, h.Handle, log)
	if err != nil {
		return fmt.Errorf("init consumer: %w", err)
	}

	log.Info("media-worker started", zap.String("env", cfg.AppEnv), zap.String("version", version))
	if err := consumer.Run(ctx); err != nil {
		return err
	}
	log.Info("media-worker stopped")
	return nil
}
