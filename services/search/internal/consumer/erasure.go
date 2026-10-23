package consumer

import (
	"context"
	"fmt"

	"github.com/jackc/pgx/v5/pgxpool"
	"go.uber.org/zap"

	"github.com/poro/shared-go/events"
	"github.com/poro/shared-go/inbox"
	"github.com/poro/shared-go/kafka"

	"github.com/poro/search/internal/repository"
)

// ErasureGroup is the Kafka consumer group that erases accounts (ADR-0018).
const ErasureGroup = "poro-search-account-erasure"

// HidingGroup is the Kafka consumer group that hides the accounts whose
// deletion is pending (shared-go/hiding).
const HidingGroup = "poro-search-account-hiding"

// ErasureTopics are the events the erasure group consumes.
var ErasureTopics = []string{events.TypeAuthUserDeleted}

// NewEraser forgets an erased account in the transaction of its inbox claim.
// Search holds no object in storage, so the claim and the erasure commit together.
func NewEraser(pool *pgxpool.Pool, log *zap.Logger) kafka.Handler {
	return func(ctx context.Context, env events.Envelope) error {
		if env.Type != events.TypeAuthUserDeleted || env.Version != 1 {
			return kafka.Permanent(fmt.Errorf("unsupported event %s v%d", env.Type, env.Version))
		}
		var d events.AuthUserDeletedV1
		if err := env.DecodeData(&d); err != nil {
			return kafka.Permanent(err)
		}
		ids, err := parseIDs(d.UserID)
		if err != nil {
			return kafka.Permanent(err)
		}
		at := orNow(d.DeletedAt)
		tx, err := pool.Begin(ctx)
		if err != nil {
			return fmt.Errorf("begin: %w", err)
		}
		defer func() { _ = tx.Rollback(ctx) }()
		first, err := inbox.Claim(ctx, tx, ErasureGroup, env.ID)
		if err != nil {
			return err
		}
		if first {
			if err := repository.EraseUser(ctx, tx, ids[0], at); err != nil {
				return err
			}
			log.Info("account erased", zap.String("user_id", ids[0].String()))
		}
		if err := tx.Commit(ctx); err != nil {
			return fmt.Errorf("commit: %w", err)
		}
		return nil
	}
}
