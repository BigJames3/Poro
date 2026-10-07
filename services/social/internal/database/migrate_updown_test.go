package database_test

import (
	"context"
	"database/sql"
	"errors"
	"os"
	"path/filepath"
	"testing"

	"github.com/golang-migrate/migrate/v4"
	pgx5 "github.com/golang-migrate/migrate/v4/database/pgx/v5"
	"github.com/golang-migrate/migrate/v4/source/iofs"
	"github.com/stretchr/testify/require"

	"github.com/poro/social/internal/testdb"
)

func TestMain(m *testing.M) { os.Exit(testdb.Run(m)) }

var tables = []string{
	"videos_projection", "users_projection", "likes", "comments", "comment_likes", "follows",
	"shares", "video_counters", "user_counters", "outbox_events", "processed_events",
}

func tableCount(t *testing.T) int {
	t.Helper()
	var n int
	require.NoError(t, testdb.Pool.QueryRow(context.Background(),
		`SELECT count(*) FROM information_schema.tables WHERE table_schema = 'public' AND table_name = ANY($1)`,
		tables).Scan(&n))
	return n
}

// testdb applied the up migration; down must drop every table and up must
// recreate them on the same database.
func TestMigrationsDownAndUpAgain(t *testing.T) {
	testdb.Available(t)
	require.Equal(t, len(tables), tableCount(t))

	db, err := sql.Open("pgx", testdb.Pool.Config().ConnString())
	require.NoError(t, err)
	driver, err := pgx5.WithInstance(db, &pgx5.Config{MultiStatementEnabled: true})
	require.NoError(t, err)
	source, err := iofs.New(os.DirFS(filepath.Join("..", "..", "migrations")), ".")
	require.NoError(t, err)
	m, err := migrate.NewWithInstance("iofs", source, "pgx5", driver)
	require.NoError(t, err)
	t.Cleanup(func() { _, _ = m.Close() })

	require.NoError(t, m.Down())
	require.Zero(t, tableCount(t), "down drops every table")

	require.NoError(t, m.Up())
	require.Equal(t, len(tables), tableCount(t), "up recreates them")
	if err := m.Up(); !errors.Is(err, migrate.ErrNoChange) {
		require.NoError(t, err)
	}
}
