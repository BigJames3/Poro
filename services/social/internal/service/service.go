// Package service implements the social rules. Every write runs in one
// transaction that changes the rows, the counters and the outbox together.
package service

import (
	"context"
	"errors"
	"fmt"
	"time"

	"github.com/gofiber/fiber/v2"
	"github.com/google/uuid"
	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgxpool"
	"go.uber.org/zap"

	"github.com/poro/shared-go/events"
	"github.com/poro/shared-go/httpx"
	"github.com/poro/shared-go/outbox"

	"github.com/poro/social/internal/cursor"
	"github.com/poro/social/internal/dto"
	"github.com/poro/social/internal/model"
	"github.com/poro/social/internal/repository"
)

var (
	errVideoNotFound   = httpx.NewAPIError(fiber.StatusNotFound, "video_not_found", "video not found")
	errCommentNotFound = httpx.NewAPIError(fiber.StatusNotFound, "comment_not_found", "comment not found")
	errUserNotFound    = httpx.NewAPIError(fiber.StatusNotFound, "user_not_found", "user not found")
	errFollowSelf      = httpx.NewAPIError(fiber.StatusBadRequest, "cannot_follow_self", "you cannot follow yourself")
	errCommentInvalid  = httpx.NewAPIError(fiber.StatusUnprocessableEntity, "comment_invalid",
		"comment must be 1 to 1000 characters without control characters")
	errReplyDepth     = httpx.NewAPIError(fiber.StatusUnprocessableEntity, "comment_reply_depth", "replies to a reply are not allowed")
	errParentInvalid  = httpx.NewAPIError(fiber.StatusUnprocessableEntity, "parent_invalid", "parent comment is not on this video")
	errEditWindow     = httpx.NewAPIError(fiber.StatusForbidden, "comment_edit_window_closed", "a comment can only be edited for 15 minutes")
	errNotAuthor      = httpx.NewAPIError(fiber.StatusForbidden, "forbidden", "only the author can do this")
	errChannelInvalid = httpx.NewAPIError(fiber.StatusUnprocessableEntity, "channel_invalid", "channel must be whatsapp, copy_link or other")
	errCursor         = httpx.NewAPIError(fiber.StatusBadRequest, "invalid_cursor", "invalid cursor")
	errInternal       = httpx.NewAPIError(fiber.StatusInternalServerError, "internal_error", "internal server error")
)

// Social is the application service of the social domain.
type Social struct {
	pool *pgxpool.Pool
	log  *zap.Logger
	now  func() time.Time
}

// New builds the service.
func New(pool *pgxpool.Pool, log *zap.Logger) *Social {
	return &Social{pool: pool, log: log, now: time.Now}
}

// clock returns the current time at the precision Postgres stores.
func (s *Social) clock() time.Time {
	return s.now().UTC().Truncate(time.Microsecond)
}

// inTx runs fn in a transaction. Errors that are not API errors become 500.
func (s *Social) inTx(ctx context.Context, fn func(pgx.Tx) error) error {
	tx, err := s.pool.Begin(ctx)
	if err != nil {
		return errInternal.WithCause(fmt.Errorf("begin: %w", err))
	}
	defer func() { _ = tx.Rollback(ctx) }()
	if err := fn(tx); err != nil {
		return asAPIError(err)
	}
	if err := tx.Commit(ctx); err != nil {
		return errInternal.WithCause(fmt.Errorf("commit: %w", err))
	}
	return nil
}

func asAPIError(err error) error {
	var apiErr *httpx.APIError
	if errors.As(err, &apiErr) {
		return err
	}
	return errInternal.WithCause(err)
}

// emit enqueues an event in the caller's transaction.
func emit(ctx context.Context, tx pgx.Tx, eventType, subject string, data any, at time.Time) error {
	env, err := events.New(eventType, 1, model.EventSource, subject, data, at)
	if err != nil {
		return err
	}
	return outbox.Enqueue(ctx, tx, env)
}

func newID() uuid.UUID {
	return uuid.Must(uuid.NewV7())
}

// readyVideo returns the owner of a ready video or video_not_found.
func readyVideo(ctx context.Context, db repository.DBTX, videoID uuid.UUID) (uuid.UUID, error) {
	owner, err := repository.ReadyVideoOwner(ctx, db, videoID)
	if errors.Is(err, repository.ErrNotFound) {
		return uuid.Nil, errVideoNotFound
	}
	return owner, err
}

func knownUser(ctx context.Context, db repository.DBTX, userID uuid.UUID) error {
	ok, err := repository.UserExists(ctx, db, userID)
	if err != nil {
		return err
	}
	if !ok {
		return errUserNotFound
	}
	return nil
}

func decodeCursor(raw string) (*cursor.Position, error) {
	pos, err := cursor.Decode(raw)
	if err != nil {
		return nil, errCursor
	}
	return pos, nil
}

func viewerID(viewer *uuid.UUID) uuid.UUID {
	if viewer == nil {
		return uuid.Nil
	}
	return *viewer
}

// page fetches limit+1 rows to know whether a next page exists.
func page[R, T any](rows []R, limit int, pos func(R) cursor.Position, view func(R) T) dto.Page[T] {
	out := dto.Page[T]{Items: make([]T, 0, min(len(rows), limit))}
	for i, r := range rows {
		if i == limit {
			next := cursor.Encode(pos(rows[limit-1]))
			out.NextCursor = &next
			break
		}
		out.Items = append(out.Items, view(r))
	}
	return out
}
