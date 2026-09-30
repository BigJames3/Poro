// Package consumer applies events from other services to auth state.
package consumer

import (
	"context"
	"errors"
	"fmt"

	"github.com/google/uuid"
	"github.com/jackc/pgx/v5/pgxpool"
	"go.uber.org/zap"

	"github.com/poro/shared-go/events"
	"github.com/poro/shared-go/inbox"
	"github.com/poro/shared-go/kafka"

	"github.com/poro/auth/internal/model"
	"github.com/poro/auth/internal/repository"
)

// CreatorRolesGroup is the consumer group granting CREATOR on poro.user.creator.activated.
const CreatorRolesGroup = "poro-auth-creator-roles"

// NewCreatorActivated grants CREATOR once per event. The role reaches the
// access token at the next refresh; auth stays the only writer of roles.
func NewCreatorActivated(pool *pgxpool.Pool, log *zap.Logger) kafka.Handler {
	return func(ctx context.Context, env events.Envelope) error {
		if env.Type != events.TypeUserCreatorActivated || env.Version != 1 {
			return kafka.Permanent(fmt.Errorf("unsupported event %s v%d", env.Type, env.Version))
		}
		var data events.UserCreatorActivatedV1
		if err := env.DecodeData(&data); err != nil {
			return kafka.Permanent(err)
		}
		userID, err := uuid.Parse(data.UserID)
		if err != nil {
			return kafka.Permanent(fmt.Errorf("user_id: %w", err))
		}
		if data.ActivatedAt.IsZero() {
			return kafka.Permanent(errors.New("activated_at is required"))
		}

		tx, err := pool.Begin(ctx)
		if err != nil {
			return fmt.Errorf("begin: %w", err)
		}
		defer func() { _ = tx.Rollback(ctx) }()

		first, err := inbox.Claim(ctx, tx, CreatorRolesGroup, env.ID)
		if err != nil {
			return err
		}
		if first {
			granted, err := repository.GrantRole(ctx, tx, userID, model.RoleCreator, data.ActivatedAt)
			if err != nil {
				return err
			}
			log.Info("creator activated", zap.String("user_id", userID.String()), zap.Bool("granted", granted))
		}
		if err := tx.Commit(ctx); err != nil {
			return fmt.Errorf("commit: %w", err)
		}
		return nil
	}
}
