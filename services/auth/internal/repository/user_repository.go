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

var (
	// ErrUserNotFound is returned when no active user matches the query.
	ErrUserNotFound = errors.New("user not found")
	// ErrUserAlreadyExists is returned when phone or email is already taken by an active user.
	ErrUserAlreadyExists = errors.New("user already exists")
)

const (
	userColumns = `id, phone, email, password_hash, status, country_code, language, last_login_at, created_at, updated_at, deleted_at`
	userSelect  = `
		SELECT u.id, u.phone, u.email, u.password_hash, u.status, u.country_code, u.language,
			u.last_login_at, u.created_at, u.updated_at, u.deleted_at,
			COALESCE((SELECT array_agg(r.role ORDER BY r.granted_at, r.role) FROM user_roles r WHERE r.user_id = u.id), '{}')
		FROM users u`
)

// UserRepository persists auth accounts and their roles.
type UserRepository interface {
	Create(ctx context.Context, user *model.User) error
	GetByID(ctx context.Context, id uuid.UUID) (*model.User, error)
	GetByPhone(ctx context.Context, phone string) (*model.User, error)
	GetByEmail(ctx context.Context, email string) (*model.User, error)
	Update(ctx context.Context, user *model.User) error
	UpdateLastLogin(ctx context.Context, id uuid.UUID) error
	SoftDelete(ctx context.Context, id uuid.UUID) error
	ExistsByPhone(ctx context.Context, phone string) (bool, error)
	ExistsByEmail(ctx context.Context, email string) (bool, error)
}

type userRepository struct {
	pool *pgxpool.Pool
}

// NewUserRepository returns a Postgres-backed user repository.
func NewUserRepository(pool *pgxpool.Pool) UserRepository {
	return &userRepository{pool: pool}
}

// Create inserts the account and its roles in one transaction.
// An account without roles gets PERSONAL.
func (r *userRepository) Create(ctx context.Context, user *model.User) error {
	if user == nil {
		return fmt.Errorf("create user: user is nil")
	}
	if err := prepareNewUser(user); err != nil {
		return fmt.Errorf("create user: %w", err)
	}

	tx, err := r.pool.Begin(ctx)
	if err != nil {
		return fmt.Errorf("create user: begin: %w", err)
	}
	defer func() { _ = tx.Rollback(ctx) }()

	const insertUser = `INSERT INTO users (` + userColumns + `) VALUES ($1, $2, $3, $4, $5, $6, $7, $8, $9, $10, $11)`
	if _, err := tx.Exec(ctx, insertUser,
		user.ID, user.Phone, user.Email, user.PasswordHash, user.Status,
		user.CountryCode, user.Language, user.LastLoginAt, user.CreatedAt, user.UpdatedAt, user.DeletedAt,
	); err != nil {
		return mapUserWriteErr("create user", err)
	}

	const insertRoles = `INSERT INTO user_roles (user_id, role, granted_at) SELECT $1, unnest($2::varchar[]), $3`
	if _, err := tx.Exec(ctx, insertRoles, user.ID, user.RoleNames(), user.CreatedAt); err != nil {
		return fmt.Errorf("create user: insert roles: %w", err)
	}

	if err := tx.Commit(ctx); err != nil {
		return mapUserWriteErr("create user", err)
	}
	return nil
}

func (r *userRepository) GetByID(ctx context.Context, id uuid.UUID) (*model.User, error) {
	const q = userSelect + ` WHERE u.id = $1 AND u.deleted_at IS NULL`
	user, err := scanUser(r.pool.QueryRow(ctx, q, id))
	if err != nil {
		return nil, fmt.Errorf("get user by id: %w", err)
	}
	return user, nil
}

func (r *userRepository) GetByPhone(ctx context.Context, phone string) (*model.User, error) {
	const q = userSelect + ` WHERE u.phone = $1 AND u.deleted_at IS NULL`
	user, err := scanUser(r.pool.QueryRow(ctx, q, phone))
	if err != nil {
		return nil, fmt.Errorf("get user by phone: %w", err)
	}
	return user, nil
}

func (r *userRepository) GetByEmail(ctx context.Context, email string) (*model.User, error) {
	const q = userSelect + ` WHERE u.email = $1 AND u.deleted_at IS NULL`
	user, err := scanUser(r.pool.QueryRow(ctx, q, email))
	if err != nil {
		return nil, fmt.Errorf("get user by email: %w", err)
	}
	return user, nil
}

// Update writes the account columns. Roles are not changed.
func (r *userRepository) Update(ctx context.Context, user *model.User) error {
	if user == nil {
		return fmt.Errorf("update user: user is nil")
	}
	user.UpdatedAt = time.Now().UTC()
	const q = `
		UPDATE users SET
			phone = $2,
			email = $3,
			password_hash = $4,
			status = $5,
			country_code = $6,
			language = $7,
			updated_at = $8
		WHERE id = $1 AND deleted_at IS NULL`
	tag, err := r.pool.Exec(ctx, q,
		user.ID, user.Phone, user.Email, user.PasswordHash, user.Status,
		user.CountryCode, user.Language, user.UpdatedAt,
	)
	if err != nil {
		return mapUserWriteErr("update user", err)
	}
	if tag.RowsAffected() == 0 {
		return fmt.Errorf("update user: %w", ErrUserNotFound)
	}
	return nil
}

func (r *userRepository) UpdateLastLogin(ctx context.Context, id uuid.UUID) error {
	now := time.Now().UTC()
	const q = `UPDATE users SET last_login_at = $2, updated_at = $2 WHERE id = $1 AND deleted_at IS NULL`
	tag, err := r.pool.Exec(ctx, q, id, now)
	if err != nil {
		return fmt.Errorf("update last login: %w", err)
	}
	if tag.RowsAffected() == 0 {
		return fmt.Errorf("update last login: %w", ErrUserNotFound)
	}
	return nil
}

func (r *userRepository) SoftDelete(ctx context.Context, id uuid.UUID) error {
	now := time.Now().UTC()
	const q = `
		UPDATE users SET status = $2, deleted_at = $3, updated_at = $3
		WHERE id = $1 AND deleted_at IS NULL`
	tag, err := r.pool.Exec(ctx, q, id, model.StatusDeleted, now)
	if err != nil {
		return fmt.Errorf("soft delete user: %w", err)
	}
	if tag.RowsAffected() == 0 {
		return fmt.Errorf("soft delete user: %w", ErrUserNotFound)
	}
	return nil
}

func (r *userRepository) ExistsByPhone(ctx context.Context, phone string) (bool, error) {
	return r.exists(ctx, `SELECT EXISTS(SELECT 1 FROM users WHERE phone = $1 AND deleted_at IS NULL)`, phone, "phone")
}

func (r *userRepository) ExistsByEmail(ctx context.Context, email string) (bool, error) {
	return r.exists(ctx, `SELECT EXISTS(SELECT 1 FROM users WHERE email = $1 AND deleted_at IS NULL)`, email, "email")
}

func (r *userRepository) exists(ctx context.Context, q, value, field string) (bool, error) {
	var found bool
	if err := r.pool.QueryRow(ctx, q, value).Scan(&found); err != nil {
		return false, fmt.Errorf("exists by %s: %w", field, err)
	}
	return found, nil
}

func prepareNewUser(user *model.User) error {
	if user.ID == uuid.Nil {
		id, err := uuid.NewV7()
		if err != nil {
			return fmt.Errorf("new id: %w", err)
		}
		user.ID = id
	}
	now := time.Now().UTC()
	if user.CreatedAt.IsZero() {
		user.CreatedAt = now
	}
	if user.UpdatedAt.IsZero() {
		user.UpdatedAt = user.CreatedAt
	}
	if len(user.Roles) == 0 {
		user.Roles = []model.UserRole{model.RolePersonal}
	}
	if user.Status == "" {
		user.Status = model.StatusPending
	}
	if user.Language == "" {
		user.Language = "fr"
	}
	return nil
}

func scanUser(row pgx.Row) (*model.User, error) {
	var user model.User
	var roles []string
	err := row.Scan(
		&user.ID, &user.Phone, &user.Email, &user.PasswordHash, &user.Status,
		&user.CountryCode, &user.Language, &user.LastLoginAt, &user.CreatedAt, &user.UpdatedAt, &user.DeletedAt,
		&roles,
	)
	if errors.Is(err, pgx.ErrNoRows) {
		return nil, ErrUserNotFound
	}
	if err != nil {
		return nil, fmt.Errorf("scan user: %w", err)
	}
	user.Roles = make([]model.UserRole, len(roles))
	for i, role := range roles {
		user.Roles[i] = model.UserRole(role)
	}
	return &user, nil
}

func mapUserWriteErr(op string, err error) error {
	var pgErr *pgconn.PgError
	if errors.As(err, &pgErr) && pgErr.Code == "23505" {
		return fmt.Errorf("%s: %w", op, ErrUserAlreadyExists)
	}
	return fmt.Errorf("%s: %w", op, err)
}
