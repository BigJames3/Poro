package repository

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"time"

	"github.com/google/uuid"
	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgxpool"

	"github.com/poro/video/internal/model"
)

// ErrNotFound is returned when a video row is missing or soft-deleted.
var ErrNotFound = errors.New("video not found")

const videoColumns = `id, user_id, title, description, status, content_type, size_bytes, source_key, s3_upload_id,
duration_ms, width, height, hls_key, thumbnail_key, renditions, failure_code, last_error, created_at, updated_at, deleted_at`

// Videos persists the VIDEO aggregate.
type Videos struct {
	pool *pgxpool.Pool
}

// New returns a repository.
func New(pool *pgxpool.Pool) *Videos { return &Videos{pool: pool} }

// Pool exposes the connection pool for transactions.
func (r *Videos) Pool() *pgxpool.Pool { return r.pool }

// Create inserts a new uploading video.
func (r *Videos) Create(ctx context.Context, v *model.Video) error {
	rend, err := json.Marshal(v.Renditions)
	if err != nil {
		return err
	}
	if rend == nil {
		rend = []byte("[]")
	}
	_, err = r.pool.Exec(ctx, `INSERT INTO videos (
		id, user_id, title, description, status, content_type, size_bytes, source_key, s3_upload_id, renditions, created_at, updated_at
	) VALUES ($1,$2,$3,$4,$5,$6,$7,$8,$9,$10,$11,$12)`,
		v.ID, v.UserID, v.Title, v.Description, v.Status, v.ContentType, v.SizeBytes, v.SourceKey, v.S3UploadID, rend, v.CreatedAt, v.UpdatedAt)
	if err != nil {
		return fmt.Errorf("insert video: %w", err)
	}
	return nil
}

// Get returns a non-deleted video.
func (r *Videos) Get(ctx context.Context, id uuid.UUID) (*model.Video, error) {
	return scanVideo(r.pool.QueryRow(ctx,
		`SELECT `+videoColumns+` FROM videos WHERE id = $1 AND deleted_at IS NULL`, id))
}

// GetAny returns a video including soft-deleted rows (worker / abort).
func (r *Videos) GetAny(ctx context.Context, id uuid.UUID) (*model.Video, error) {
	return scanVideo(r.pool.QueryRow(ctx, `SELECT `+videoColumns+` FROM videos WHERE id = $1`, id))
}

// ListByUser returns the owner's videos newest first.
func (r *Videos) ListByUser(ctx context.Context, userID uuid.UUID, cursorTime *time.Time, cursorID *uuid.UUID, limit int) ([]model.Video, error) {
	var rows pgx.Rows
	var err error
	if cursorTime == nil {
		rows, err = r.pool.Query(ctx, `SELECT `+videoColumns+` FROM videos
			WHERE user_id = $1 AND deleted_at IS NULL
			ORDER BY created_at DESC, id DESC LIMIT $2`, userID, limit)
	} else {
		rows, err = r.pool.Query(ctx, `SELECT `+videoColumns+` FROM videos
			WHERE user_id = $1 AND deleted_at IS NULL
			  AND (created_at, id) < ($2, $3)
			ORDER BY created_at DESC, id DESC LIMIT $4`, userID, *cursorTime, *cursorID, limit)
	}
	if err != nil {
		return nil, fmt.Errorf("list videos: %w", err)
	}
	defer rows.Close()
	var out []model.Video
	for rows.Next() {
		v, err := scanVideo(rows)
		if err != nil {
			return nil, err
		}
		out = append(out, *v)
	}
	return out, rows.Err()
}

// MarkProcessing moves uploading → processing after multipart complete.
func (r *Videos) MarkProcessing(ctx context.Context, tx pgx.Tx, id uuid.UUID, size int64) error {
	tag, err := tx.Exec(ctx, `UPDATE videos SET status = $2, size_bytes = $3, s3_upload_id = NULL, updated_at = now()
		WHERE id = $1 AND status = $4 AND deleted_at IS NULL`,
		id, model.StatusProcessing, size, model.StatusUploading)
	if err != nil {
		return fmt.Errorf("mark processing: %w", err)
	}
	if tag.RowsAffected() != 1 {
		return ErrNotFound
	}
	return nil
}

// SoftDelete marks the row deleted. It is a no-op if already deleted.
func (r *Videos) SoftDelete(ctx context.Context, tx pgx.Tx, id, userID uuid.UUID) (*model.Video, error) {
	row := tx.QueryRow(ctx, `UPDATE videos SET deleted_at = now(), updated_at = now()
		WHERE id = $1 AND user_id = $2 AND deleted_at IS NULL
		RETURNING `+videoColumns, id, userID)
	return scanVideo(row)
}

// MarkReady records HLS output. Only processing rows change.
func (r *Videos) MarkReady(ctx context.Context, tx pgx.Tx, id uuid.UUID, durationMs, width, height int, hlsKey, thumbKey string, renditions []model.Rendition) error {
	raw, err := json.Marshal(renditions)
	if err != nil {
		return err
	}
	tag, err := tx.Exec(ctx, `UPDATE videos SET
		status = $2, duration_ms = $3, width = $4, height = $5, hls_key = $6, thumbnail_key = $7,
		renditions = $8, failure_code = NULL, last_error = NULL, updated_at = now()
		WHERE id = $1 AND status = $9 AND deleted_at IS NULL`,
		id, model.StatusReady, durationMs, width, height, hlsKey, thumbKey, raw, model.StatusProcessing)
	if err != nil {
		return fmt.Errorf("mark ready: %w", err)
	}
	if tag.RowsAffected() != 1 {
		return ErrNotFound
	}
	return nil
}

// MarkFailed records a public failure code. internalErr is never returned to clients.
func (r *Videos) MarkFailed(ctx context.Context, tx pgx.Tx, id uuid.UUID, code, internalErr string) error {
	tag, err := tx.Exec(ctx, `UPDATE videos SET status = $2, failure_code = $3, last_error = $4, updated_at = now()
		WHERE id = $1 AND status = $5 AND deleted_at IS NULL`,
		id, model.StatusFailed, code, internalErr, model.StatusProcessing)
	if err != nil {
		return fmt.Errorf("mark failed: %w", err)
	}
	if tag.RowsAffected() != 1 {
		return ErrNotFound
	}
	return nil
}

type scannable interface {
	Scan(dest ...any) error
}

func scanVideo(row scannable) (*model.Video, error) {
	var v model.Video
	var rend []byte
	err := row.Scan(
		&v.ID, &v.UserID, &v.Title, &v.Description, &v.Status, &v.ContentType, &v.SizeBytes, &v.SourceKey, &v.S3UploadID,
		&v.DurationMs, &v.Width, &v.Height, &v.HLSKey, &v.ThumbnailKey, &rend, &v.FailureCode, &v.LastError,
		&v.CreatedAt, &v.UpdatedAt, &v.DeletedAt,
	)
	if errors.Is(err, pgx.ErrNoRows) {
		return nil, ErrNotFound
	}
	if err != nil {
		return nil, fmt.Errorf("scan video: %w", err)
	}
	if len(rend) > 0 {
		if err := json.Unmarshal(rend, &v.Renditions); err != nil {
			return nil, fmt.Errorf("renditions: %w", err)
		}
	}
	return &v, nil
}
