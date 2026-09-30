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

// ErrOTPNotFound is returned when no usable OTP code matches the query.
var ErrOTPNotFound = errors.New("otp code not found")

const otpColumns = `id, phone, code_hash, attempts, max_attempts, expires_at, verified_at, created_at`

// OTPRepository persists hashed SMS codes.
type OTPRepository interface {
	Create(ctx context.Context, otp *model.OTPCode) error
	GetLatestByPhone(ctx context.Context, phone string) (*model.OTPCode, error)
	ConsumeAttempt(ctx context.Context, id uuid.UUID, now time.Time) (int, error)
	MarkVerified(ctx context.Context, id uuid.UUID, now time.Time) error
	DeleteExpired(ctx context.Context) (int64, error)
}

type otpRepository struct {
	pool *pgxpool.Pool
}

// NewOTPRepository returns a Postgres-backed OTP repository.
func NewOTPRepository(pool *pgxpool.Pool) OTPRepository {
	return &otpRepository{pool: pool}
}

func (r *otpRepository) Create(ctx context.Context, otp *model.OTPCode) error {
	if otp == nil {
		return fmt.Errorf("create otp: otp is nil")
	}
	if otp.ID == uuid.Nil {
		id, err := uuid.NewV7()
		if err != nil {
			return fmt.Errorf("create otp: new id: %w", err)
		}
		otp.ID = id
	}
	if otp.MaxAttempts == 0 {
		otp.MaxAttempts = 3
	}
	if otp.CreatedAt.IsZero() {
		otp.CreatedAt = time.Now().UTC()
	}

	const q = `INSERT INTO otp_codes (` + otpColumns + `) VALUES ($1, $2, $3, $4, $5, $6, $7, $8)`
	_, err := r.pool.Exec(ctx, q,
		otp.ID, otp.Phone, otp.CodeHash, otp.Attempts, otp.MaxAttempts,
		otp.ExpiresAt, otp.VerifiedAt, otp.CreatedAt,
	)
	if err != nil {
		return fmt.Errorf("create otp: %w", err)
	}
	return nil
}

func (r *otpRepository) GetLatestByPhone(ctx context.Context, phone string) (*model.OTPCode, error) {
	const q = `SELECT ` + otpColumns + ` FROM otp_codes WHERE phone = $1 ORDER BY created_at DESC LIMIT 1`
	otp, err := scanOTP(r.pool.QueryRow(ctx, q, phone))
	if err != nil {
		return nil, fmt.Errorf("get latest otp: %w", err)
	}
	return otp, nil
}

// ConsumeAttempt spends one attempt on a live code and returns the attempts used so far.
// The check and the increment are one statement, so parallel guesses cannot exceed the budget.
// ErrOTPNotFound means the code is unknown, verified, expired, or out of attempts.
func (r *otpRepository) ConsumeAttempt(ctx context.Context, id uuid.UUID, now time.Time) (int, error) {
	const q = `
		UPDATE otp_codes SET attempts = attempts + 1
		WHERE id = $1 AND verified_at IS NULL AND attempts < max_attempts AND expires_at > $2
		RETURNING attempts`
	var attempts int
	err := r.pool.QueryRow(ctx, q, id, now).Scan(&attempts)
	if errors.Is(err, pgx.ErrNoRows) {
		return 0, fmt.Errorf("consume otp attempt: %w", ErrOTPNotFound)
	}
	if err != nil {
		return 0, fmt.Errorf("consume otp attempt: %w", err)
	}
	return attempts, nil
}

// MarkVerified flags the code as used. ErrOTPNotFound means it was already used.
func (r *otpRepository) MarkVerified(ctx context.Context, id uuid.UUID, now time.Time) error {
	const q = `UPDATE otp_codes SET verified_at = $2 WHERE id = $1 AND verified_at IS NULL`
	tag, err := r.pool.Exec(ctx, q, id, now)
	if err != nil {
		return fmt.Errorf("mark otp verified: %w", err)
	}
	if tag.RowsAffected() == 0 {
		return fmt.Errorf("mark otp verified: %w", ErrOTPNotFound)
	}
	return nil
}

func (r *otpRepository) DeleteExpired(ctx context.Context) (int64, error) {
	const q = `DELETE FROM otp_codes WHERE expires_at < $1`
	tag, err := r.pool.Exec(ctx, q, time.Now().UTC())
	if err != nil {
		return 0, fmt.Errorf("delete expired otp codes: %w", err)
	}
	return tag.RowsAffected(), nil
}

func scanOTP(row pgx.Row) (*model.OTPCode, error) {
	var otp model.OTPCode
	err := row.Scan(
		&otp.ID, &otp.Phone, &otp.CodeHash, &otp.Attempts, &otp.MaxAttempts,
		&otp.ExpiresAt, &otp.VerifiedAt, &otp.CreatedAt,
	)
	if errors.Is(err, pgx.ErrNoRows) {
		return nil, ErrOTPNotFound
	}
	if err != nil {
		return nil, fmt.Errorf("scan otp: %w", err)
	}
	return &otp, nil
}
