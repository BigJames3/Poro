package repository

import (
	"context"
	"errors"
	"fmt"
	"time"

	"github.com/google/uuid"
	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgconn"
	"github.com/jackc/pgx/v5/pgxpool"

	"github.com/poro/auth/internal/model"
)

// ErrRefreshTokenNotFound is returned when no refresh token matches the query.
var ErrRefreshTokenNotFound = errors.New("refresh token not found")

//nolint:gosec // G101: a column list that names token_hash, not a credential
const refreshTokenColumns = `id, user_id, family_id, token_hash, expires_at, revoked_at, replaced_by, user_agent, ip_address, created_at`

// RefreshTokenStore holds the operations available inside and outside a transaction.
type RefreshTokenStore interface {
	Create(ctx context.Context, token *model.RefreshToken) error
	GetByHash(ctx context.Context, hash string) (*model.RefreshToken, error)
}

// RefreshTokenTx runs inside a transaction. The ForUpdate reads lock the row
// until the transaction ends, which serializes concurrent rotations of one token.
type RefreshTokenTx interface {
	RefreshTokenStore
	GetByHashForUpdate(ctx context.Context, hash string) (*model.RefreshToken, error)
	GetByIDForUpdate(ctx context.Context, id uuid.UUID) (*model.RefreshToken, error)
	MarkReplaced(ctx context.Context, id, replacedBy uuid.UUID, at time.Time) error
	Revoke(ctx context.Context, id uuid.UUID, at time.Time) error
	RevokeFamily(ctx context.Context, familyID uuid.UUID, at time.Time) (int64, error)
}

// RefreshTokenRepository persists hashed refresh tokens grouped in session families.
type RefreshTokenRepository interface {
	RefreshTokenStore
	// InTx commits when fn returns nil and rolls back otherwise.
	InTx(ctx context.Context, fn func(tx RefreshTokenTx) error) error
	RevokeFamily(ctx context.Context, userID, familyID uuid.UUID) (int64, error)
	RevokeAllForUser(ctx context.Context, userID uuid.UUID) error
	DeleteExpired(ctx context.Context) (int64, error)
}

type dbtx interface {
	Exec(ctx context.Context, sql string, args ...any) (pgconn.CommandTag, error)
	QueryRow(ctx context.Context, sql string, args ...any) pgx.Row
}

type refreshTokenRepository struct {
	pool *pgxpool.Pool
	refreshTokenQueries
}

type refreshTokenQueries struct {
	db dbtx
}

// NewRefreshTokenRepository returns a Postgres-backed refresh token repository.
func NewRefreshTokenRepository(pool *pgxpool.Pool) RefreshTokenRepository {
	return &refreshTokenRepository{pool: pool, refreshTokenQueries: refreshTokenQueries{db: pool}}
}

func (r *refreshTokenRepository) InTx(ctx context.Context, fn func(tx RefreshTokenTx) error) error {
	tx, err := r.pool.Begin(ctx)
	if err != nil {
		return fmt.Errorf("refresh token tx: begin: %w", err)
	}
	defer func() { _ = tx.Rollback(ctx) }()
	if err := fn(&refreshTokenQueries{db: tx}); err != nil {
		return err
	}
	if err := tx.Commit(ctx); err != nil {
		return fmt.Errorf("refresh token tx: commit: %w", err)
	}
	return nil
}

func (r *refreshTokenRepository) RevokeFamily(ctx context.Context, userID, familyID uuid.UUID) (int64, error) {
	const q = `UPDATE refresh_tokens SET revoked_at = $3 WHERE family_id = $1 AND user_id = $2 AND revoked_at IS NULL`
	tag, err := r.pool.Exec(ctx, q, familyID, userID, time.Now().UTC())
	if err != nil {
		return 0, fmt.Errorf("revoke refresh token family: %w", err)
	}
	return tag.RowsAffected(), nil
}

func (r *refreshTokenRepository) RevokeAllForUser(ctx context.Context, userID uuid.UUID) error {
	const q = `UPDATE refresh_tokens SET revoked_at = $2 WHERE user_id = $1 AND revoked_at IS NULL`
	if _, err := r.pool.Exec(ctx, q, userID, time.Now().UTC()); err != nil {
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

// Create inserts the token. A token without FamilyID starts a new session family.
func (s *refreshTokenQueries) Create(ctx context.Context, token *model.RefreshToken) error {
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
	if token.FamilyID == uuid.Nil {
		token.FamilyID = token.ID
	}
	if token.CreatedAt.IsZero() {
		token.CreatedAt = time.Now().UTC()
	}

	const q = `INSERT INTO refresh_tokens (` + refreshTokenColumns + `) VALUES ($1, $2, $3, $4, $5, $6, $7, $8, $9, $10)`
	_, err := s.db.Exec(ctx, q,
		token.ID, token.UserID, token.FamilyID, token.TokenHash, token.ExpiresAt, token.RevokedAt,
		token.ReplacedBy, token.UserAgent, token.IPAddress, token.CreatedAt,
	)
	if err != nil {
		return fmt.Errorf("create refresh token: %w", err)
	}
	return nil
}

func (s *refreshTokenQueries) GetByHash(ctx context.Context, hash string) (*model.RefreshToken, error) {
	const q = `SELECT ` + refreshTokenColumns + ` FROM refresh_tokens WHERE token_hash = $1`
	token, err := scanRefreshToken(s.db.QueryRow(ctx, q, hash))
	if err != nil {
		return nil, fmt.Errorf("get refresh token by hash: %w", err)
	}
	return token, nil
}

func (s *refreshTokenQueries) GetByHashForUpdate(ctx context.Context, hash string) (*model.RefreshToken, error) {
	const q = `SELECT ` + refreshTokenColumns + ` FROM refresh_tokens WHERE token_hash = $1 FOR UPDATE`
	token, err := scanRefreshToken(s.db.QueryRow(ctx, q, hash))
	if err != nil {
		return nil, fmt.Errorf("lock refresh token by hash: %w", err)
	}
	return token, nil
}

func (s *refreshTokenQueries) GetByIDForUpdate(ctx context.Context, id uuid.UUID) (*model.RefreshToken, error) {
	const q = `SELECT ` + refreshTokenColumns + ` FROM refresh_tokens WHERE id = $1 FOR UPDATE`
	token, err := scanRefreshToken(s.db.QueryRow(ctx, q, id))
	if err != nil {
		return nil, fmt.Errorf("lock refresh token by id: %w", err)
	}
	return token, nil
}

// MarkReplaced revokes the token (keeping an earlier revocation time) and links its successor.
func (s *refreshTokenQueries) MarkReplaced(ctx context.Context, id, replacedBy uuid.UUID, at time.Time) error {
	const q = `UPDATE refresh_tokens SET revoked_at = COALESCE(revoked_at, $3), replaced_by = $2 WHERE id = $1`
	tag, err := s.db.Exec(ctx, q, id, replacedBy, at)
	if err != nil {
		return fmt.Errorf("mark refresh token replaced: %w", err)
	}
	if tag.RowsAffected() == 0 {
		return fmt.Errorf("mark refresh token replaced: %w", ErrRefreshTokenNotFound)
	}
	return nil
}

func (s *refreshTokenQueries) Revoke(ctx context.Context, id uuid.UUID, at time.Time) error {
	const q = `UPDATE refresh_tokens SET revoked_at = COALESCE(revoked_at, $2) WHERE id = $1`
	tag, err := s.db.Exec(ctx, q, id, at)
	if err != nil {
		return fmt.Errorf("revoke refresh token: %w", err)
	}
	if tag.RowsAffected() == 0 {
		return fmt.Errorf("revoke refresh token: %w", ErrRefreshTokenNotFound)
	}
	return nil
}

func (s *refreshTokenQueries) RevokeFamily(ctx context.Context, familyID uuid.UUID, at time.Time) (int64, error) {
	const q = `UPDATE refresh_tokens SET revoked_at = $2 WHERE family_id = $1 AND revoked_at IS NULL`
	tag, err := s.db.Exec(ctx, q, familyID, at)
	if err != nil {
		return 0, fmt.Errorf("revoke refresh token family: %w", err)
	}
	return tag.RowsAffected(), nil
}

func scanRefreshToken(row pgx.Row) (*model.RefreshToken, error) {
	var token model.RefreshToken
	err := row.Scan(
		&token.ID, &token.UserID, &token.FamilyID, &token.TokenHash, &token.ExpiresAt, &token.RevokedAt,
		&token.ReplacedBy, &token.UserAgent, &token.IPAddress, &token.CreatedAt,
	)
	if errors.Is(err, pgx.ErrNoRows) {
		return nil, ErrRefreshTokenNotFound
	}
	if err != nil {
		return nil, fmt.Errorf("scan refresh token: %w", err)
	}
	return &token, nil
}
