// Package hiding keeps, in each service, which accounts are hidden while
// their deletion is pending (poro.auth.user.deletion_scheduled and
// .deletion_cancelled). A hidden account keeps its data: only the public
// reads leave out its profile and content, until it is erased or the
// deletion cancelled.
//
// Each service owns a table:
//
//	CREATE TABLE hidden_accounts (
//	    user_id    UUID PRIMARY KEY,
//	    hidden     BOOLEAN NOT NULL,
//	    changed_at TIMESTAMPTZ NOT NULL
//	);
//
// The two events travel on different topics, so their order is not
// guaranteed: the latest changed_at wins, which makes replays harmless.
package hiding

import (
	"context"
	"fmt"
	"strings"
	"time"

	"github.com/google/uuid"
	"github.com/jackc/pgx/v5/pgconn"

	"github.com/poro/shared-go/events"
	"github.com/poro/shared-go/kafka"
)

// Topics are the events a hiding consumer reads.
var Topics = []string{events.TypeAuthUserDeletionScheduled, events.TypeAuthUserDeletionCancelled}

// Upsert records a change unless a later one is already stored.
const Upsert = `INSERT INTO hidden_accounts (user_id, hidden, changed_at) VALUES ($1, $2, $3)
	ON CONFLICT (user_id) DO UPDATE SET hidden = EXCLUDED.hidden, changed_at = EXCLUDED.changed_at
	WHERE hidden_accounts.changed_at < EXCLUDED.changed_at`

// NotHidden is a SQL condition true when the account in column is shown.
// column must be qualified (v.owner_id, likes.user_id) or a parameter: a bare
// user_id would resolve to hidden_accounts.user_id inside the subquery.
func NotHidden(column string) string {
	if !strings.Contains(column, ".") && !strings.HasPrefix(column, "$") {
		panic("hiding.NotHidden: unqualified column " + column)
	}
	return `NOT EXISTS (SELECT 1 FROM hidden_accounts ha WHERE ha.user_id = ` + column + ` AND ha.hidden)`
}

// Change is one hiding or showing of an account.
type Change struct {
	UserID uuid.UUID
	Hidden bool
	At     time.Time
}

// Decode reads a change from a deletion_scheduled or deletion_cancelled event.
func Decode(env events.Envelope) (Change, error) {
	var (
		raw    string
		at     time.Time
		hidden bool
	)
	switch {
	case env.Type == events.TypeAuthUserDeletionScheduled && env.Version == 1:
		var d events.AuthUserDeletionScheduledV1
		if err := env.DecodeData(&d); err != nil {
			return Change{}, err
		}
		raw, at, hidden = d.UserID, d.ChangedAt, true
	case env.Type == events.TypeAuthUserDeletionCancelled && env.Version == 1:
		var d events.AuthUserDeletionCancelledV1
		if err := env.DecodeData(&d); err != nil {
			return Change{}, err
		}
		raw, at = d.UserID, d.ChangedAt
	default:
		return Change{}, fmt.Errorf("unsupported event %s v%d", env.Type, env.Version)
	}
	id, err := uuid.Parse(raw)
	if err != nil {
		return Change{}, fmt.Errorf("user_id: %w", err)
	}
	if at.IsZero() {
		return Change{}, fmt.Errorf("changed_at is required")
	}
	return Change{UserID: id, Hidden: hidden, At: at}, nil
}

// Execer is the part of a pool or transaction Handler needs.
type Execer interface {
	Exec(ctx context.Context, sql string, args ...any) (pgconn.CommandTag, error)
}

// Handler applies each change to the hidden_accounts table of db. A malformed
// event is permanent (DLQ); a database error is retried.
func Handler(db Execer) kafka.Handler {
	return func(ctx context.Context, env events.Envelope) error {
		c, err := Decode(env)
		if err != nil {
			return kafka.Permanent(err)
		}
		if _, err := db.Exec(ctx, Upsert, c.UserID, c.Hidden, c.At); err != nil {
			return fmt.Errorf("hide account: %w", err)
		}
		return nil
	}
}
