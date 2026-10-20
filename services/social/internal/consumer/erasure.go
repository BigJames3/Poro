package consumer

import (
	"context"
	"fmt"
	"time"

	"github.com/google/uuid"
	"github.com/jackc/pgx/v5/pgxpool"
	"go.uber.org/zap"

	"github.com/poro/shared-go/events"
	"github.com/poro/shared-go/inbox"
	"github.com/poro/shared-go/kafka"

	"github.com/poro/social/internal/service"
)

// ErasureGroup is the Kafka consumer group that erases accounts.
const ErasureGroup = "poro-social-account-erasure"

// ErasureTopics are the events the erasure consumes.
var ErasureTopics = []string{events.TypeAuthUserDeleted}

// NewErasure erases each account once, in the transaction of its inbox claim
// (see service.EraseAccount). Social holds no object, so nothing happens
// after the commit.
func NewErasure(pool *pgxpool.Pool, log *zap.Logger) kafka.Handler {
	return func(ctx context.Context, env events.Envelope) error {
		if env.Type != events.TypeAuthUserDeleted || env.Version != 1 {
			return kafka.Permanent(fmt.Errorf("unsupported event %s v%d", env.Type, env.Version))
		}
		var d events.AuthUserDeletedV1
		if err := env.DecodeData(&d); err != nil {
			return kafka.Permanent(err)
		}
		user, err := uuid.Parse(d.UserID)
		if err != nil || user == uuid.Nil {
			return kafka.Permanent(fmt.Errorf("invalid user_id %q", d.UserID))
		}
		at := d.DeletedAt
		if at.IsZero() {
			at = time.Now()
		}

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
			if err := service.EraseAccount(ctx, tx, user, at); err != nil {
				return err
			}
		}
		if err := tx.Commit(ctx); err != nil {
			return fmt.Errorf("commit: %w", err)
		}
		if first {
			log.Info("account erased", zap.String("user_id", user.String()))
		}
		return nil
	}
}
