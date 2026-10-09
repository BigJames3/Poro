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

// BusinessRolesGroup is the consumer group granting BUSINESS on poro.shop.shop.created.
const BusinessRolesGroup = "poro-auth-business-roles"

// NewShopCreated grants BUSINESS to the owner of a new shop once per event.
// Closing the shop keeps the role: it only unlocks the seller screens.
func NewShopCreated(pool *pgxpool.Pool, log *zap.Logger) kafka.Handler {
	return func(ctx context.Context, env events.Envelope) error {
		if env.Type != events.TypeShopShopCreated || env.Version != 1 {
			return kafka.Permanent(fmt.Errorf("unsupported event %s v%d", env.Type, env.Version))
		}
		var data events.ShopShopCreatedV1
		if err := env.DecodeData(&data); err != nil {
			return kafka.Permanent(err)
		}
		ownerID, err := uuid.Parse(data.OwnerID)
		if err != nil {
			return kafka.Permanent(fmt.Errorf("owner_id: %w", err))
		}
		if data.CreatedAt.IsZero() {
			return kafka.Permanent(errors.New("created_at is required"))
		}
		return grantRole(ctx, pool, log, BusinessRolesGroup, env.ID, ownerID, model.RoleBusiness, data.CreatedAt)
	}
}
