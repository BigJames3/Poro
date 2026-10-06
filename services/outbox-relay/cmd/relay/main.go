// Command relay publishes the outbox_events of one service database to Kafka.
package main

import (
	"context"
	"errors"
	"fmt"
	"os"
	"os/signal"
	"syscall"
	"time"

	"github.com/gofiber/fiber/v2"
	"github.com/jackc/pgx/v5/pgxpool"
	"go.uber.org/zap"

	"github.com/poro/outbox-relay/internal/config"
	"github.com/poro/shared-go/health"
	"github.com/poro/shared-go/kafka"
	"github.com/poro/shared-go/logging"
	"github.com/poro/shared-go/metrics"
	"github.com/poro/shared-go/outbox"
)

const version = "1.0.0"

func main() {
	if err := run(); err != nil {
		fmt.Fprintf(os.Stderr, "outbox relay: %v\n", err)
		os.Exit(1)
	}
}

func run() error {
	cfg, err := config.Load()
	if err != nil {
		return err
	}
	log, err := logging.New("outbox-relay", cfg.LogLevel, cfg.IsDev())
	if err != nil {
		return err
	}
	defer func() { _ = log.Sync() }()
	log = log.With(zap.String("source", cfg.Source))

	ctx, stop := signal.NotifyContext(context.Background(), syscall.SIGINT, syscall.SIGTERM)
	defer stop()

	pool, err := pgxpool.New(ctx, cfg.PostgresDSN())
	if err != nil {
		return fmt.Errorf("postgres pool: %w", err)
	}
	defer pool.Close()
	pingCtx, cancel := context.WithTimeout(ctx, 10*time.Second)
	err = pool.Ping(pingCtx)
	cancel()
	if err != nil {
		return fmt.Errorf("postgres ping: %w", err)
	}

	producer, err := kafka.NewProducer(cfg.KafkaBrokers)
	if err != nil {
		return err
	}
	defer producer.Close()

	m, err := metrics.New("outbox-relay-" + cfg.Source)
	if err != nil {
		return err
	}
	relay, err := outbox.NewRelay(pool, producer, log, outbox.RelayConfig{
		BatchSize:    cfg.BatchSize,
		PollInterval: cfg.PollInterval,
		Retention:    cfg.Retention,
	}, m.Registry())
	if err != nil {
		return err
	}

	app := fiber.New(fiber.Config{DisableStartupMessage: true})
	health.New("outbox-relay", version, map[string]health.Check{
		"postgres": pool.Ping,
		"kafka":    producer.Ping,
	}).Register(app)
	app.Get("/metrics", m.Handler())

	errCh := make(chan error, 2)
	go func() { errCh <- app.Listen(":" + cfg.HTTPPort) }()
	go func() { errCh <- relay.Run(ctx) }()
	log.Info("outbox relay started", zap.String("db", cfg.PostgresDB), zap.Strings("brokers", cfg.KafkaBrokers))

	select {
	case <-ctx.Done():
	case err := <-errCh:
		if err != nil && !errors.Is(err, context.Canceled) {
			return err
		}
	}
	shutdownCtx, cancelShutdown := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancelShutdown()
	_ = app.ShutdownWithContext(shutdownCtx)
	log.Info("outbox relay stopped")
	return nil
}
