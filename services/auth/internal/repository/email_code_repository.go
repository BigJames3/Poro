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

// ErrEmailCodeNotFound is returned when no usable email code matches the query.
var ErrEmailCodeNotFound = errors.New("email code not found")

const emailCodeColumns = `id, user_id, purpose, email, code_hash, attempts, max_attempts, expires_at, consumed_at, created_at`

// EmailCodeRepository persists hashed email codes and applies what they unlock.
type EmailCodeRepository interface {
	Create(ctx context.Context, code *model.EmailCode) error
	// Latest returns the newest code of userID for purpose.
	Latest(ctx context.Context, userID uuid.UUID, purpose model.EmailCodePurpose) (*model.EmailCode, error)
	// ConsumeAttempt spends one attempt on a live code and returns the attempts used.
	ConsumeAttempt(ctx context.Context, id uuid.UUID, now time.Time) (int, error)
	// VerifyEmail uses the code and marks its address verified, if it is still the account's.
	VerifyEmail(ctx context.Context, code *model.EmailCode, now time.Time) error
	// ResetPassword uses the code, stores the new hash, marks the address verified
	// and ends every session of the account, in one transaction.
	ResetPassword(ctx context.Context, code *model.EmailCode, passwordHash string, now time.Time) error
	DeleteExpired(ctx context.Context, before time.Time) (int64, error)
}

type emailCodeRepository struct {
	pool *pgxpool.Pool
}

// NewEmailCodeRepository returns a Postgres-backed email code repository.
func NewEmailCodeRepository(pool *pgxpool.Pool) EmailCodeRepository {
	return &emailCodeRepository{pool: pool}
}

func (r *emailCodeRepository) Create(ctx context.Context, code *model.EmailCode) error {
	if code == nil {
		return fmt.Errorf("create email code: code is nil")
	}
	if code.ID == uuid.Nil {
		id, err := uuid.NewV7()
		if err != nil {
			return fmt.Errorf("create email code: new id: %w", err)
		}
		code.ID = id
	}
	if code.CreatedAt.IsZero() {
		code.CreatedAt = time.Now().UTC()
	}
	const q = `INSERT INTO email_codes (` + emailCodeColumns + `) VALUES ($1, $2, $3, $4, $5, $6, $7, $8, $9, $10)`
	_, err := r.pool.Exec(ctx, q,
		code.ID, code.UserID, code.Purpose, code.Email, code.CodeHash, code.Attempts,
		code.MaxAttempts, code.ExpiresAt, code.ConsumedAt, code.CreatedAt,
	)
	if err != nil {
		return fmt.Errorf("create email code: %w", err)
	}
	return nil
}

func (r *emailCodeRepository) Latest(ctx context.Context, userID uuid.UUID, purpose model.EmailCodePurpose) (*model.EmailCode, error) {
	const q = `SELECT ` + emailCodeColumns + ` FROM email_codes
		WHERE user_id = $1 AND purpose = $2 ORDER BY created_at DESC, id DESC LIMIT 1`
	var code model.EmailCode
	err := r.pool.QueryRow(ctx, q, userID, purpose).Scan(
		&code.ID, &code.UserID, &code.Purpose, &code.Email, &code.CodeHash, &code.Attempts,
		&code.MaxAttempts, &code.ExpiresAt, &code.ConsumedAt, &code.CreatedAt,
	)
	if errors.Is(err, pgx.ErrNoRows) {
		return nil, fmt.Errorf("latest email code: %w", ErrEmailCodeNotFound)
	}
	if err != nil {
		return nil, fmt.Errorf("latest email code: %w", err)
	}
	return &code, nil
}

// ConsumeAttempt checks and increments in one statement, so parallel guesses
// cannot exceed the budget. ErrEmailCodeNotFound means the code is used,
// expired or out of attempts.
func (r *emailCodeRepository) ConsumeAttempt(ctx context.Context, id uuid.UUID, now time.Time) (int, error) {
	const q = `
		UPDATE email_codes SET attempts = attempts + 1
		WHERE id = $1 AND consumed_at IS NULL AND attempts < max_attempts AND expires_at > $2
		RETURNING attempts`
	var attempts int
	err := r.pool.QueryRow(ctx, q, id, now).Scan(&attempts)
	if errors.Is(err, pgx.ErrNoRows) {
		return 0, fmt.Errorf("consume email code attempt: %w", ErrEmailCodeNotFound)
	}
	if err != nil {
		return 0, fmt.Errorf("consume email code attempt: %w", err)
	}
	return attempts, nil
}

func (r *emailCodeRepository) VerifyEmail(ctx context.Context, code *model.EmailCode, now time.Time) error {
	return r.inTx(ctx, "verify email", func(tx pgx.Tx) error {
		if err := consume(ctx, tx, code.ID, now); err != nil {
			return err
		}
		return markVerified(ctx, tx, code, now)
	})
}

func (r *emailCodeRepository) ResetPassword(ctx context.Context, code *model.EmailCode, passwordHash string, now time.Time) error {
	return r.inTx(ctx, "reset password", func(tx pgx.Tx) error {
		if err := consume(ctx, tx, code.ID, now); err != nil {
			return err
		}
		const setPassword = `
			UPDATE users SET password_hash = $2, updated_at = $3
			WHERE id = $1 AND deleted_at IS NULL`
		tag, err := tx.Exec(ctx, setPassword, code.UserID, passwordHash, now)
		if err != nil {
			return fmt.Errorf("set password: %w", err)
		}
		if tag.RowsAffected() == 0 {
			return ErrUserNotFound
		}
		if err := markVerified(ctx, tx, code, now); err != nil {
			return err
		}
		// A reset is what an owner does after losing control: every device signs in again.
		const revoke = `UPDATE refresh_tokens SET revoked_at = $2 WHERE user_id = $1 AND revoked_at IS NULL`
		if _, err := tx.Exec(ctx, revoke, code.UserID, now); err != nil {
			return fmt.Errorf("revoke sessions: %w", err)
		}
		return nil
	})
}

func (r *emailCodeRepository) DeleteExpired(ctx context.Context, before time.Time) (int64, error) {
	tag, err := r.pool.Exec(ctx, `DELETE FROM email_codes WHERE expires_at < $1`, before)
	if err != nil {
		return 0, fmt.Errorf("delete expired email codes: %w", err)
	}
	return tag.RowsAffected(), nil
}

func (r *emailCodeRepository) inTx(ctx context.Context, op string, fn func(tx pgx.Tx) error) error {
	tx, err := r.pool.Begin(ctx)
	if err != nil {
		return fmt.Errorf("%s: begin: %w", op, err)
	}
	defer func() { _ = tx.Rollback(ctx) }()
	if err := fn(tx); err != nil {
		return fmt.Errorf("%s: %w", op, err)
	}
	if err := tx.Commit(ctx); err != nil {
		return fmt.Errorf("%s: commit: %w", op, err)
	}
	return nil
}

// consume marks the code used. A code already used, by a parallel request, is not found.
func consume(ctx context.Context, tx pgx.Tx, id uuid.UUID, now time.Time) error {
	tag, err := tx.Exec(ctx, `UPDATE email_codes SET consumed_at = $2 WHERE id = $1 AND consumed_at IS NULL`, id, now)
	if err != nil {
		return fmt.Errorf("consume email code: %w", err)
	}
	if tag.RowsAffected() == 0 {
		return ErrEmailCodeNotFound
	}
	return nil
}

// markVerified only verifies the address the code was sent to, if the account still has it.
func markVerified(ctx context.Context, tx pgx.Tx, code *model.EmailCode, now time.Time) error {
	const q = `
		UPDATE users SET email_verified_at = COALESCE(email_verified_at, $3), updated_at = $3
		WHERE id = $1 AND email = $2 AND deleted_at IS NULL`
	tag, err := tx.Exec(ctx, q, code.UserID, code.Email, now)
	if err != nil {
		return fmt.Errorf("mark email verified: %w", err)
	}
	if tag.RowsAffected() == 0 {
		return ErrUserNotFound
	}
	return nil
}
