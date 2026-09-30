// Package testinfra starts the containers shared by integration tests.
package testinfra

import (
	"context"
	"fmt"
	"strings"
	"testing"

	"github.com/jackc/pgx/v5/pgxpool"
	"github.com/stretchr/testify/require"
	"github.com/testcontainers/testcontainers-go/modules/postgres"
	"github.com/testcontainers/testcontainers-go/modules/redpanda"
	"github.com/twmb/franz-go/pkg/kadm"
	"github.com/twmb/franz-go/pkg/kgo"

	"github.com/poro/shared-go/events"
	"github.com/poro/shared-go/inbox"
	"github.com/poro/shared-go/outbox"
)

// Postgres starts a database with the outbox and inbox tables.
func Postgres(t *testing.T) *pgxpool.Pool {
	t.Helper()
	ctx := context.Background()
	c, err := postgres.Run(ctx, "postgres:16-alpine",
		postgres.WithDatabase("poro_test"),
		postgres.WithUsername("poro"),
		postgres.WithPassword("poro"),
		postgres.BasicWaitStrategies(),
	)
	require.NoError(t, err)
	t.Cleanup(func() { _ = c.Terminate(context.Background()) })

	dsn, err := c.ConnectionString(ctx, "sslmode=disable")
	require.NoError(t, err)
	dsn = strings.Replace(dsn, "localhost", "127.0.0.1", 1)
	pool, err := pgxpool.New(ctx, dsn)
	require.NoError(t, err)
	t.Cleanup(pool.Close)

	_, err = pool.Exec(ctx, outbox.Schema)
	require.NoError(t, err)
	_, err = pool.Exec(ctx, inbox.Schema)
	require.NoError(t, err)
	return pool
}

// Redpanda starts a single broker and returns its seed address.
func Redpanda(t *testing.T) string {
	t.Helper()
	ctx := context.Background()
	c, err := redpanda.Run(ctx, "redpandadata/redpanda:v24.2.7")
	require.NoError(t, err)
	t.Cleanup(func() { _ = c.Terminate(context.Background()) })
	broker, err := c.KafkaSeedBroker(ctx)
	require.NoError(t, err)
	return strings.Replace(broker, "localhost", "127.0.0.1", 1)
}

// Topic creates a topic unique to the test, and its dead letter topic, then returns its name.
func Topic(t *testing.T, broker, action string) string {
	t.Helper()
	name := fmt.Sprintf("poro.test.%s.%s", strings.ToLower(strings.NewReplacer("/", "_", "-", "_").Replace(t.Name())), action)
	client, err := kgo.NewClient(kgo.SeedBrokers(broker))
	require.NoError(t, err)
	defer client.Close()
	resp, err := kadm.NewClient(client).CreateTopics(context.Background(), 3, 1, nil, name, events.DLQTopic(name))
	require.NoError(t, err)
	for _, r := range resp.Sorted() {
		require.NoError(t, r.Err, r.Topic)
	}
	return name
}
