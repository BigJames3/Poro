package repository

import (
	"context"
	"fmt"
	"time"

	"github.com/google/uuid"
	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgxpool"

	"github.com/poro/shared-go/events"
	"github.com/poro/shared-go/outbox"

	"github.com/poro/auth/internal/model"
)

// AccountPurger erases the accounts whose deletion date has passed.
type AccountPurger interface {
	// PurgeDue erases at most limit due accounts and returns their ids.
	PurgeDue(ctx context.Context, now time.Time, limit int) ([]uuid.UUID, error)
}

type accountPurger struct {
	pool *pgxpool.Pool
}

// NewAccountPurger returns a Postgres-backed account purger.
func NewAccountPurger(pool *pgxpool.Pool) AccountPurger {
	return &accountPurger{pool: pool}
}

// PurgeDue runs in one transaction. SKIP LOCKED lets several replicas purge
// side by side; a sign-in that cancels the deletion waits for the row lock and
// then finds the account gone.
//
// The row and its id stay, without any identifier: phone and email become free
// again, and the account can never sign in. Roles, sessions and pending OTP
// and email codes are deleted, and poro.auth.user.deleted tells the other services.
func (p *accountPurger) PurgeDue(ctx context.Context, now time.Time, limit int) ([]uuid.UUID, error) {
	tx, err := p.pool.Begin(ctx)
	if err != nil {
		return nil, fmt.Errorf("purge accounts: begin: %w", err)
	}
	defer func() { _ = tx.Rollback(ctx) }()

	const due = `
		SELECT id, phone FROM users
		WHERE deletion_scheduled_at <= $1 AND deleted_at IS NULL
		ORDER BY deletion_scheduled_at, id
		LIMIT $2
		FOR UPDATE SKIP LOCKED`
	rows, err := tx.Query(ctx, due, now, limit)
	if err != nil {
		return nil, fmt.Errorf("purge accounts: select: %w", err)
	}
	type dueAccount struct {
		id    uuid.UUID
		phone *string
	}
	accounts, err := pgx.CollectRows(rows, func(row pgx.CollectableRow) (dueAccount, error) {
		var a dueAccount
		err := row.Scan(&a.id, &a.phone)
		return a, err
	})
	if err != nil {
		return nil, fmt.Errorf("purge accounts: select: %w", err)
	}

	ids := make([]uuid.UUID, 0, len(accounts))
	for _, a := range accounts {
		if err := erase(ctx, tx, a.id, a.phone, now); err != nil {
			return nil, fmt.Errorf("purge account %s: %w", a.id, err)
		}
		ids = append(ids, a.id)
	}
	if err := tx.Commit(ctx); err != nil {
		return nil, fmt.Errorf("purge accounts: commit: %w", err)
	}
	return ids, nil
}

func erase(ctx context.Context, tx pgx.Tx, id uuid.UUID, phone *string, now time.Time) error {
	const anonymize = `
		UPDATE users SET
			phone = NULL, email = NULL, password_hash = NULL, country_code = NULL, email_verified_at = NULL,
			status = $2, deleted_at = $3, updated_at = $3, deletion_scheduled_at = NULL
		WHERE id = $1`
	if _, err := tx.Exec(ctx, anonymize, id, model.StatusDeleted, now); err != nil {
		return fmt.Errorf("anonymize: %w", err)
	}
	if _, err := tx.Exec(ctx, `DELETE FROM user_roles WHERE user_id = $1`, id); err != nil {
		return fmt.Errorf("delete roles: %w", err)
	}
	if _, err := tx.Exec(ctx, `DELETE FROM refresh_tokens WHERE user_id = $1`, id); err != nil {
		return fmt.Errorf("delete sessions: %w", err)
	}
	if _, err := tx.Exec(ctx, `DELETE FROM email_codes WHERE user_id = $1`, id); err != nil {
		return fmt.Errorf("delete email codes: %w", err)
	}
	if phone != nil {
		if _, err := tx.Exec(ctx, `DELETE FROM otp_codes WHERE phone = $1`, *phone); err != nil {
			return fmt.Errorf("delete otp codes: %w", err)
		}
	}
	deleted, err := events.New(events.TypeAuthUserDeleted, 1, EventSource, id.String(), events.AuthUserDeletedV1{
		UserID:    id.String(),
		DeletedAt: now,
	}, now)
	if err != nil {
		return err
	}
	return outbox.Enqueue(ctx, tx, deleted)
}
