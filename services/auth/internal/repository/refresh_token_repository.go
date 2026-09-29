package repository

import (
	"context"
	"errors"
	"fmt"
	"time"

	"github.com/google/uuid"
	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgxpool"

	"github.com/poro/auth/internal/model"
)

// ErrRefreshTokenNotFound is returned when no refresh token matches the query.
var ErrRefreshTokenNotFound = errors.New("refresh token not found")

const refreshTokenColumns = `id, user_id, token_hash, expires_at, revoked_at, user_agent, ip_address, created_at`

// RefreshTokenRepository persists hashed refresh tokens.
type RefreshTokenRepository interface {
	Create(ctx context.Context, token *model.RefreshToken) error
	GetByHash(ctx context.Context, hash string) (*model.RefreshToken, error)
	Revoke(ctx context.Context, id uuid.UUID) error
	RevokeAllForUser(ctx context.Context, userID uuid.UUID) error
	DeleteExpired(ctx context.Context) (int64, error)
}

type refreshTokenRepository struct {
	pool *pgxpool.Pool
}

// NewRefreshTokenRepository returns a Postgres-backed refresh token repository.
func NewRefreshTokenRepository(pool *pgxpool.Pool) RefreshTokenRepository {
	return &refreshTokenRepository{pool: pool}
}

func (r *refreshTokenRepository) Create(ctx context.Context, token *model.RefreshToken) error {
	if token == nil {
		return fmt.Errorf("create refresh token: token is nil")
	}
	if token.ID == uuid.Nil {
		id, err := uuid.NewV7()
		if err != nil {
			return fmt.Errorf("create refresh token: new id: %w", err)
		}
		token.ID = id
	}
	if token.CreatedAt.IsZero() {
		token.CreatedAt = time.Now().UTC()
	}

	const q = `INSERT INTO refresh_tokens (` + refreshTokenColumns + `) VALUES ($1, $2, $3, $4, $5, $6, $7, $8)`
	_, err := r.pool.Exec(ctx, q,
		token.ID, token.UserID, token.TokenHash, token.ExpiresAt, token.RevokedAt,
		token.UserAgent, token.IPAddress, token.CreatedAt,
	)
	if err != nil {
		return fmt.Errorf("create refresh token: %w", err)
	}
	return nil
}

func (r *refreshTokenRepository) GetByHash(ctx context.Context, hash string) (*model.RefreshToken, error) {
	const q = `SELECT ` + refreshTokenColumns + ` FROM refresh_tokens WHERE token_hash = $1`
	token, err := scanRefreshToken(r.pool.QueryRow(ctx, q, hash))
	if err != nil {
		return nil, fmt.Errorf("get refresh token by hash: %w", err)
	}
	return token, nil
}

func (r *refreshTokenRepository) Revoke(ctx context.Context, id uuid.UUID) error {
	now := time.Now().UTC()
	const q = `UPDATE refresh_tokens SET revoked_at = COALESCE(revoked_at, $2) WHERE id = $1`
	tag, err := r.pool.Exec(ctx, q, id, now)
	if err != nil {
		return fmt.Errorf("revoke refresh token: %w", err)
	}
	if tag.RowsAffected() == 0 {
		return fmt.Errorf("revoke refresh token: %w", ErrRefreshTokenNotFound)
	}
	return nil
}

func (r *refreshTokenRepository) RevokeAllForUser(ctx context.Context, userID uuid.UUID) error {
	now := time.Now().UTC()
	const q = `UPDATE refresh_tokens SET revoked_at = $2 WHERE user_id = $1 AND revoked_at IS NULL`
	if _, err := r.pool.Exec(ctx, q, userID, now); err != nil {
		return fmt.Errorf("revoke refresh tokens for user: %w", err)
	}
	return nil
}

func (r *refreshTokenRepository) DeleteExpired(ctx context.Context) (int64, error) {
	const q = `DELETE FROM refresh_tokens WHERE expires_at < $1`
	tag, err := r.pool.Exec(ctx, q, time.Now().UTC())
	if err != nil {
		return 0, fmt.Errorf("delete expired refresh tokens: %w", err)
	}
	return tag.RowsAffected(), nil
}

func scanRefreshToken(row pgx.Row) (*model.RefreshToken, error) {
	var token model.RefreshToken
	err := row.Scan(
		&token.ID, &token.UserID, &token.TokenHash, &token.ExpiresAt, &token.RevokedAt,
		&token.UserAgent, &token.IPAddress, &token.CreatedAt,
	)
	if errors.Is(err, pgx.ErrNoRows) {
		return nil, ErrRefreshTokenNotFound
	}
	if err != nil {
		return nil, fmt.Errorf("scan refresh token: %w", err)
	}
	return &token, nil
}
