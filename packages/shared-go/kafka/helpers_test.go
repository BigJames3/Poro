package kafka_test

import (
	"context"
	"encoding/json"
	"testing"

	"github.com/google/uuid"
	"github.com/jackc/pgx/v5/pgxpool"
	"github.com/stretchr/testify/require"

	"github.com/poro/shared-go/events"
)

func jsonValue(env events.Envelope) ([]byte, error) { return json.Marshal(env) }

func mustValue(t *testing.T, pool *pgxpool.Pool, id uuid.UUID) []byte {
	t.Helper()
	var payload string
	require.NoError(t, pool.QueryRow(context.Background(), `SELECT payload::text FROM outbox_events WHERE id = $1`, id).Scan(&payload))
	return []byte(payload)
}
