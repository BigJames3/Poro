package consumer

import (
	"context"
	"errors"
	"fmt"

	"github.com/google/uuid"
	"github.com/jackc/pgx/v5/pgxpool"
	"go.uber.org/zap"

	"github.com/poro/shared-go/events"
	"github.com/poro/shared-go/kafka"

	"github.com/poro/auth/internal/model"
)

// CreatorRolesGroup is the consumer group granting CREATOR on poro.user.creator.activated.
const CreatorRolesGroup = "poro-auth-creator-roles"

// NewCreatorActivated grants CREATOR once per event.
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
		return grantRole(ctx, pool, log, CreatorRolesGroup, env.ID, userID, model.RoleCreator, data.ActivatedAt)
	}
}
