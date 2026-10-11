package e2e

import (
	"context"
	"testing"

	"github.com/jackc/pgx/v5/pgxpool"
	"github.com/stretchr/testify/require"
	"github.com/testcontainers/testcontainers-go/modules/postgres"
	"go.uber.org/zap"

	"github.com/poro/auth/internal/config"
	"github.com/poro/auth/internal/database"
)

// startPostgres runs a migrated auth database for one test.
func startPostgres(t *testing.T) *pgxpool.Pool {
	t.Helper()
	ctx := context.Background()
	log := zap.NewNop()
	pg, err := postgres.Run(ctx, "postgres:16-alpine",
		postgres.WithDatabase("poro_auth"),
		postgres.WithUsername("poro"),
		postgres.WithPassword("poro_dev_password"),
		postgres.BasicWaitStrategies(),
	)
	require.NoError(t, err)
	t.Cleanup(func() { _ = pg.Terminate(context.Background()) })
	host, err := pg.Host(ctx)
	require.NoError(t, err)
	if host == "localhost" {
		host = "127.0.0.1"
	}
	port, err := pg.MappedPort(ctx, "5432/tcp")
	require.NoError(t, err)
	cfg := &config.Config{
		PostgresHost: host, PostgresPort: port.Port(), PostgresUser: "poro",
		PostgresPassword: "poro_dev_password", PostgresDB: "poro_auth", PostgresSSLMode: "disable",
	}
	require.NoError(t, database.RunMigrations(ctx, cfg, log))
	pool, err := database.NewPostgresPool(ctx, cfg, log)
	require.NoError(t, err)
	t.Cleanup(pool.Close)
	return pool
}
