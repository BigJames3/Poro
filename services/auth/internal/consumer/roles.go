// Package consumer applies events from other services to auth state.
package consumer

import (
	"context"
	"fmt"
	"time"

	"github.com/google/uuid"
	"github.com/jackc/pgx/v5/pgxpool"
	"go.uber.org/zap"

	"github.com/poro/shared-go/inbox"

	"github.com/poro/auth/internal/model"
	"github.com/poro/auth/internal/repository"
)

// grantRole grants role once per event in one transaction with the inbox
// claim. The role reaches the access token at the next refresh; auth stays
// the only writer of roles. An unknown account is skipped.
func grantRole(ctx context.Context, pool *pgxpool.Pool, log *zap.Logger, group string, eventID uuid.UUID,
	userID uuid.UUID, role model.UserRole, at time.Time) error {
	tx, err := pool.Begin(ctx)
	if err != nil {
		return fmt.Errorf("begin: %w", err)
	}
	defer func() { _ = tx.Rollback(ctx) }()

	first, err := inbox.Claim(ctx, tx, group, eventID)
	if err != nil {
		return err
	}
	if first {
		granted, err := repository.GrantRole(ctx, tx, userID, role, at)
		if err != nil {
			return err
		}
		log.Info("role granted", zap.String("user_id", userID.String()), zap.String("role", string(role)), zap.Bool("granted", granted))
	}
	if err := tx.Commit(ctx); err != nil {
		return fmt.Errorf("commit: %w", err)
	}
	return nil
}
