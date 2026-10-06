// Package inbox makes Kafka consumers idempotent: an event is applied at most
// once per consumer because its ID is recorded in the same transaction as its effect.
package inbox

import (
	"context"
	"fmt"

	"github.com/google/uuid"
	"github.com/jackc/pgx/v5/pgconn"
)

// Schema is the table every consuming service database must create in a migration.
const Schema = `CREATE TABLE IF NOT EXISTS processed_events (
    consumer     VARCHAR(100) NOT NULL,
    event_id     UUID NOT NULL,
    processed_at TIMESTAMPTZ NOT NULL DEFAULT now(),
    PRIMARY KEY (consumer, event_id)
);
CREATE INDEX IF NOT EXISTS processed_events_processed_at_idx ON processed_events (processed_at);`

// Execer is satisfied by pgx.Tx.
type Execer interface {
	Exec(ctx context.Context, sql string, args ...any) (pgconn.CommandTag, error)
}

// Claim records eventID for consumer. It returns false when the event was
// already processed; the caller must then skip the effect and commit.
func Claim(ctx context.Context, tx Execer, consumer string, eventID uuid.UUID) (bool, error) {
	tag, err := tx.Exec(ctx,
		`INSERT INTO processed_events (consumer, event_id) VALUES ($1, $2) ON CONFLICT DO NOTHING`,
		consumer, eventID)
	if err != nil {
		return false, fmt.Errorf("claim event %s: %w", eventID, err)
	}
	return tag.RowsAffected() == 1, nil
}
