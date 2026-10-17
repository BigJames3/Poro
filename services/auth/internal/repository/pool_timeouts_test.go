package repository_test

import (
	"context"
	"errors"
	"testing"
	"time"

	"github.com/jackc/pgx/v5/pgconn"
	"github.com/stretchr/testify/require"

	"github.com/poro/auth/internal/database"
)

func TestPoolAppliesServerTimeouts(t *testing.T) {
	ctx := context.Background()
	for setting, want := range map[string]time.Duration{
		"statement_timeout":                   database.StatementTimeout,
		"lock_timeout":                        database.LockTimeout,
		"idle_in_transaction_session_timeout": database.IdleInTxTimeout,
	} {
		var got string
		require.NoError(t, testPool.QueryRow(ctx, "SHOW "+setting).Scan(&got))
		require.Equal(t, want.String(), normalizePGDuration(t, got), setting)
	}
}

// A lock held by another transaction makes the query fail after lock_timeout
// instead of blocking the request.
func TestPoolFailsFastOnHeldLock(t *testing.T) {
	ctx := context.Background()
	_, err := testPool.Exec(ctx, `CREATE TABLE IF NOT EXISTS lock_probe (id int PRIMARY KEY)`)
	require.NoError(t, err)
	_, err = testPool.Exec(ctx, `INSERT INTO lock_probe VALUES (1) ON CONFLICT DO NOTHING`)
	require.NoError(t, err)

	holder, err := testPool.Begin(ctx)
	require.NoError(t, err)
	defer func() { _ = holder.Rollback(ctx) }()
	_, err = holder.Exec(ctx, `SELECT 1 FROM lock_probe WHERE id = 1 FOR UPDATE`)
	require.NoError(t, err)

	start := time.Now()
	_, err = testPool.Exec(ctx, `UPDATE lock_probe SET id = 1 WHERE id = 1`)
	var pgErr *pgconn.PgError
	require.True(t, errors.As(err, &pgErr), "expected a Postgres error, got %v", err)
	require.Equal(t, "55P03", pgErr.Code, "lock_not_available")
	require.Less(t, time.Since(start), database.LockTimeout+2*time.Second)
}

func normalizePGDuration(t *testing.T, v string) string {
	t.Helper()
	d, err := time.ParseDuration(v)
	if err != nil {
		// Postgres prints "5s", "2s", "30s" for these values; anything else fails the test.
		t.Fatalf("unexpected duration format %q", v)
	}
	return d.String()
}
