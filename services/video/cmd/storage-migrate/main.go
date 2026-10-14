// Command storage-migrate moves the video sources stored before the private
// source bucket existed out of the public media and quarantine buckets. It is
// idempotent: Compose runs it once before the video API and the media-worker
// start, and a run with nothing left to move does nothing.
package main

import (
	"context"
	"fmt"
	"os"
	"os/signal"
	"syscall"

	"go.uber.org/zap"

	"github.com/poro/shared-go/logging"

	"github.com/poro/video/internal/config"
	"github.com/poro/video/internal/storage"
)

func main() {
	if err := run(); err != nil {
		fmt.Fprintf(os.Stderr, "storage-migrate: %v\n", err)
		os.Exit(1)
	}
}

func run() error {
	cfg, err := config.Load()
	if err != nil {
		return fmt.Errorf("load config: %w", err)
	}
	log, err := logging.New("video-storage-migrate", cfg.LogLevel, cfg.IsDev())
	if err != nil {
		return fmt.Errorf("init logger: %w", err)
	}
	defer func() { _ = log.Sync() }()

	ctx, stop := signal.NotifyContext(context.Background(), syscall.SIGINT, syscall.SIGTERM)
	defer stop()

	store, err := storage.NewS3(ctx, cfg)
	if err != nil {
		return fmt.Errorf("init s3: %w", err)
	}
	moved, err := store.MoveLegacySources(ctx)
	if err != nil {
		return fmt.Errorf("move legacy sources after %d objects: %w", moved, err)
	}
	log.Info("legacy video sources moved to the private bucket",
		zap.Int("count", moved), zap.String("bucket", cfg.S3SourceBucket))
	return nil
}
