package repository_test

import (
	"context"
	"errors"
	"net"
	"net/url"
	"os"
	"strings"
	"testing"

	"github.com/golang-migrate/migrate/v4"
	_ "github.com/golang-migrate/migrate/v4/database/pgx/v5"
	"github.com/golang-migrate/migrate/v4/source/iofs"
	"github.com/google/uuid"
	"github.com/jackc/pgx/v5"
	"github.com/stretchr/testify/require"
)

// TestMigrationsUpgradeLegacyDataAndRoundTrip runs the migrations on a scratch
// database: it seeds the single-role schema, checks the backfills, then applies
// every down migration and upgrades again.
func TestMigrationsUpgradeLegacyDataAndRoundTrip(t *testing.T) {
	ctx := context.Background()
	dbName := "poro_migrations_" + strings.ReplaceAll(uuid.Must(uuid.NewV7()).String(), "-", "")[20:]
	_, err := testPool.Exec(ctx, "CREATE DATABASE "+pgx.Identifier{dbName}.Sanitize())
	require.NoError(t, err)

	dbURL := func(scheme, query string) string {
		return (&url.URL{
			Scheme:   scheme,
			User:     url.UserPassword(testCfg.PostgresUser, testCfg.PostgresPassword),
			Host:     net.JoinHostPort(testCfg.PostgresHost, testCfg.PostgresPort),
			Path:     "/" + dbName,
			RawQuery: query,
		}).String()
	}
	source, err := iofs.New(os.DirFS("../../migrations"), ".")
	require.NoError(t, err)
	m, err := migrate.NewWithSourceInstance("iofs", source, dbURL("pgx5", "sslmode=disable&x-multi-statement=true"))
	require.NoError(t, err)
	t.Cleanup(func() { _, _ = m.Close() })

	conn, err := pgx.Connect(ctx, dbURL("postgres", "sslmode=disable"))
	require.NoError(t, err)
	t.Cleanup(func() { _ = conn.Close(ctx) })

	require.NoError(t, m.Migrate(3))
	creatorID := uuid.Must(uuid.NewV7())
	_, err = conn.Exec(ctx, `INSERT INTO users (id, phone, email, role, status) VALUES ($1, '+2250102030405', 'Legacy@Poro.Test', 'CREATOR', 'ACTIVE')`, creatorID)
	require.NoError(t, err)
	tokenID := uuid.Must(uuid.NewV7())
	_, err = conn.Exec(ctx, `INSERT INTO refresh_tokens (id, user_id, token_hash, expires_at) VALUES ($1, $2, 'legacy-hash', NOW() + INTERVAL '1 hour')`, tokenID, creatorID)
	require.NoError(t, err)

	require.NoError(t, m.Up())

	rows, err := conn.Query(ctx, `SELECT role FROM user_roles WHERE user_id = $1 ORDER BY role`, creatorID)
	require.NoError(t, err)
	roles, err := pgx.CollectRows(rows, pgx.RowTo[string])
	require.NoError(t, err)
	require.Equal(t, []string{"CREATOR", "PERSONAL"}, roles, "the legacy role is kept and PERSONAL is added")

	var email string
	require.NoError(t, conn.QueryRow(ctx, `SELECT email FROM users WHERE id = $1`, creatorID).Scan(&email))
	require.Equal(t, "legacy@poro.test", email)

	var familyID uuid.UUID
	require.NoError(t, conn.QueryRow(ctx, `SELECT family_id FROM refresh_tokens WHERE id = $1`, tokenID).Scan(&familyID))
	require.Equal(t, tokenID, familyID, "existing tokens become their own session")

	require.NoError(t, m.Migrate(3))
	var role string
	require.NoError(t, conn.QueryRow(ctx, `SELECT role FROM users WHERE id = $1`, creatorID).Scan(&role))
	require.Equal(t, "CREATOR", role, "the down migration restores the most privileged role")

	require.NoError(t, m.Down())
	_, _, err = m.Version()
	require.True(t, errors.Is(err, migrate.ErrNilVersion), "every down migration applied")
	require.NoError(t, m.Up())
}
