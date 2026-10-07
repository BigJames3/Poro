// Package repository holds the SQL of the social service. Every function
// takes a DBTX so callers choose between the pool and a transaction.
package repository

import (
	"context"
	"errors"
	"time"

	"github.com/google/uuid"
	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgconn"

	"github.com/poro/social/internal/cursor"
)

// ErrNotFound is returned when a row does not exist.
var ErrNotFound = errors.New("not found")

// DBTX is satisfied by *pgxpool.Pool and pgx.Tx.
type DBTX interface {
	Exec(ctx context.Context, sql string, args ...any) (pgconn.CommandTag, error)
	Query(ctx context.Context, sql string, args ...any) (pgx.Rows, error)
	QueryRow(ctx context.Context, sql string, args ...any) pgx.Row
}

// keyset turns an optional cursor into the two nullable query arguments.
func keyset(after *cursor.Position) (*time.Time, *uuid.UUID) {
	if after == nil {
		return nil, nil
	}
	return &after.CreatedAt, &after.ID
}

func notFound(err error) error {
	if errors.Is(err, pgx.ErrNoRows) {
		return ErrNotFound
	}
	return err
}
