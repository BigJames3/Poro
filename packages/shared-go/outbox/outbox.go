// Package outbox implements the transactional outbox: services write events in
// the same transaction as their state change, and a relay publishes them to Kafka.
// Delivery is at least once; consumers deduplicate with the inbox package.
package outbox

import (
	"context"
	"encoding/json"
	"fmt"

	"github.com/jackc/pgx/v5/pgconn"

	"github.com/poro/shared-go/events"
)

// Schema is the outbox table every service database must create in a migration.
// The relay only relies on these columns.
const Schema = `CREATE TABLE IF NOT EXISTS outbox_events (
    id           UUID PRIMARY KEY,
    topic        VARCHAR(200) NOT NULL,
    event_key    VARCHAR(200) NOT NULL,
    payload      JSONB NOT NULL,
    created_at   TIMESTAMPTZ NOT NULL DEFAULT now(),
    published_at TIMESTAMPTZ,
    attempts     INT NOT NULL DEFAULT 0,
    last_error   TEXT
);
CREATE INDEX IF NOT EXISTS outbox_events_pending_idx ON outbox_events (created_at, id) WHERE published_at IS NULL;
CREATE INDEX IF NOT EXISTS outbox_events_published_idx ON outbox_events (published_at) WHERE published_at IS NOT NULL;`

// Execer is satisfied by pgx.Tx, pgxpool.Pool and pgx.Conn.
type Execer interface {
	Exec(ctx context.Context, sql string, args ...any) (pgconn.CommandTag, error)
}

// Enqueue stores env for publication. Call it with the transaction that changes
// the state the event describes, so both commit or neither does.
func Enqueue(ctx context.Context, db Execer, env events.Envelope) error {
	if err := env.Validate(); err != nil {
		return err
	}
	payload, err := json.Marshal(env)
	if err != nil {
		return fmt.Errorf("marshal envelope: %w", err)
	}
	_, err = db.Exec(ctx,
		`INSERT INTO outbox_events (id, topic, event_key, payload, created_at) VALUES ($1, $2, $3, $4, $5)`,
		env.ID, env.Topic(), env.Subject, payload, env.OccurredAt,
	)
	if err != nil {
		return fmt.Errorf("enqueue %s: %w", env.Type, err)
	}
	return nil
}
