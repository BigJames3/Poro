package testdb

import (
	"context"
	"fmt"
	"os"
	"testing"

	"github.com/jackc/pgx/v5/pgxpool"
	"github.com/testcontainers/testcontainers-go/modules/postgres"
	"go.uber.org/zap"

	"github.com/poro/feed/internal/config"
	"github.com/poro/feed/internal/database"
)

// Pool is the shared test database. Call Run from TestMain.
var Pool *pgxpool.Pool

// Available skips the test when Docker cannot start Postgres.
func Available(t *testing.T) {
	t.Helper()
	if Pool == nil {
		t.Skip("postgres testcontainer unavailable")
	}
}

// Run starts Postgres, applies migrations, and runs the package tests.
func Run(m *testing.M) int {
	ctx := context.Background()
	container, err := postgres.Run(ctx, "postgres:16-alpine",
		postgres.WithDatabase("poro_feed"),
		postgres.WithUsername("poro"),
		postgres.WithPassword("poro_dev_password"),
		postgres.BasicWaitStrategies(),
	)
	if err != nil {
		fmt.Fprintf(os.Stderr, "testdb unavailable: %v\n", err)
		return m.Run()
	}
	defer func() { _ = container.Terminate(context.Background()) }()

	host, err := container.Host(ctx)
	if err != nil {
		fmt.Fprintf(os.Stderr, "postgres host: %v\n", err)
		return 1
	}
	if host == "localhost" {
		host = "127.0.0.1"
	}
	port, err := container.MappedPort(ctx, "5432/tcp")
	if err != nil {
		fmt.Fprintf(os.Stderr, "postgres port: %v\n", err)
		return 1
	}

	cfg := &config.Config{ //nolint:gosec // throwaway testcontainer credentials
		PostgresHost:     host,
		PostgresPort:     port.Port(),
		PostgresUser:     "poro",
		PostgresPassword: "poro_dev_password",
		PostgresDB:       "poro_feed",
		PostgresSSLMode:  "disable",
	}
	log := zap.NewNop()
	if err := database.RunMigrations(ctx, cfg, log); err != nil {
		fmt.Fprintf(os.Stderr, "migrate: %v\n", err)
		return 1
	}
	Pool, err = database.NewPostgresPool(ctx, cfg, log)
	if err != nil {
		fmt.Fprintf(os.Stderr, "pool: %v\n", err)
		return 1
	}
	defer Pool.Close()
	return m.Run()
}
