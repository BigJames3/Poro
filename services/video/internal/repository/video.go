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
duration_ms, width, height, hls_key, thumbnail_key, renditions, failure_code, last_error, created_at, updated_at, deleted_at,
moderation_status, moderated_at, media_type`

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

// CreatePhoto inserts an uploading photo post and its images in one
// transaction: a post never exists without all of its images.
func (r *Videos) CreatePhoto(ctx context.Context, v *model.Video, images []model.Image) error {
	tx, err := r.pool.Begin(ctx)
	if err != nil {
		return fmt.Errorf("begin: %w", err)
	}
	defer func() { _ = tx.Rollback(ctx) }()
	if _, err := tx.Exec(ctx, `INSERT INTO videos (
		id, user_id, title, description, status, content_type, size_bytes, source_key, media_type, created_at, updated_at
	) VALUES ($1,$2,$3,$4,$5,$6,$7,$8,$9,$10,$11)`,
		v.ID, v.UserID, v.Title, v.Description, v.Status, v.ContentType, v.SizeBytes, v.SourceKey, model.MediaTypePhoto,
		v.CreatedAt, v.UpdatedAt); err != nil {
		return fmt.Errorf("insert photo post: %w", err)
	}
	for _, img := range images {
		if _, err := tx.Exec(ctx, `INSERT INTO video_images (video_id, position, content_type, size_bytes, source_key)
			VALUES ($1,$2,$3,$4,$5)`, v.ID, img.Position, img.ContentType, img.SizeBytes, img.SourceKey); err != nil {
			return fmt.Errorf("insert image %d: %w", img.Position, err)
		}
	}
	return tx.Commit(ctx)
}

// Images returns the images of a photo post in display order; none for a video.
func (r *Videos) Images(ctx context.Context, videoID string) ([]model.Image, error) {
	rows, err := r.pool.Query(ctx, `SELECT position, content_type, size_bytes, source_key, image_key, thumbnail_key, width, height
		FROM video_images WHERE video_id = $1 ORDER BY position`, videoID)
	if err != nil {
		return nil, fmt.Errorf("list images: %w", err)
	}
	defer rows.Close()
	var out []model.Image
	for rows.Next() {
		var img model.Image
		if err := rows.Scan(&img.Position, &img.ContentType, &img.SizeBytes, &img.SourceKey,
			&img.ImageKey, &img.ThumbnailKey, &img.Width, &img.Height); err != nil {
			return nil, fmt.Errorf("scan image: %w", err)
		}
		out = append(out, img)
	}
	return out, rows.Err()
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

// ListByUser returns the owner's publications of the given media types,
// newest first. The filter is in the query, so pages stay full and a client
// that does not display photo posts never receives one.
func (r *Videos) ListByUser(ctx context.Context, userID uuid.UUID, mediaTypes []string, cursorTime *time.Time, cursorID *uuid.UUID, limit int) ([]model.Video, error) {
	var rows pgx.Rows
	var err error
	if cursorTime == nil {
		rows, err = r.pool.Query(ctx, `SELECT `+videoColumns+` FROM videos
			WHERE user_id = $1 AND deleted_at IS NULL AND media_type = ANY($2)
			ORDER BY created_at DESC, id DESC LIMIT $3`, userID, mediaTypes, limit)
	} else {
		rows, err = r.pool.Query(ctx, `SELECT `+videoColumns+` FROM videos
			WHERE user_id = $1 AND deleted_at IS NULL AND media_type = ANY($2)
			  AND (created_at, id) < ($3, $4)
			ORDER BY created_at DESC, id DESC LIMIT $5`, userID, mediaTypes, *cursorTime, *cursorID, limit)
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

// ListStaleUploads returns live uploads created before cutoff, oldest first.
func (r *Videos) ListStaleUploads(ctx context.Context, cutoff time.Time, limit int) ([]model.Video, error) {
	rows, err := r.pool.Query(ctx, `SELECT `+videoColumns+` FROM videos
		WHERE status = $1 AND deleted_at IS NULL AND created_at < $2
		ORDER BY created_at, id LIMIT $3`, model.StatusUploading, cutoff, limit)
	if err != nil {
		return nil, fmt.Errorf("list stale uploads: %w", err)
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

// DiscardUpload soft-deletes a video only while it is still uploading. It
// returns ErrNotFound when the upload was completed, aborted or deleted meanwhile.
func (r *Videos) DiscardUpload(ctx context.Context, id string) error {
	tag, err := r.pool.Exec(ctx, `UPDATE videos SET deleted_at = now(), updated_at = now()
		WHERE id = $1 AND status = $2 AND deleted_at IS NULL`, id, model.StatusUploading)
	if err != nil {
		return fmt.Errorf("discard upload: %w", err)
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

// ListAllByUser returns every video of a user, deleted or not and whatever
// its status, oldest first (account erasure).
func (r *Videos) ListAllByUser(ctx context.Context, userID uuid.UUID) ([]model.Video, error) {
	rows, err := r.pool.Query(ctx, `SELECT `+videoColumns+` FROM videos
		WHERE user_id = $1 ORDER BY created_at, id`, userID)
	if err != nil {
		return nil, fmt.Errorf("list user videos: %w", err)
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

// EraseUser empties the title and description of every video of a user and
// soft-deletes the ones still live, whatever their status. It returns only
// the videos it deleted, so a second call returns none.
func (r *Videos) EraseUser(ctx context.Context, tx pgx.Tx, userID uuid.UUID) ([]model.Video, error) {
	if _, err := tx.Exec(ctx, `UPDATE videos SET title = '', description = '', updated_at = now()
		WHERE user_id = $1 AND deleted_at IS NOT NULL AND (title <> '' OR description <> '')`, userID); err != nil {
		return nil, fmt.Errorf("erase deleted videos: %w", err)
	}
	rows, err := tx.Query(ctx, `UPDATE videos SET title = '', description = '', deleted_at = now(), updated_at = now()
		WHERE user_id = $1 AND deleted_at IS NULL
		RETURNING `+videoColumns, userID)
	if err != nil {
		return nil, fmt.Errorf("erase videos: %w", err)
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

// SetModeration records a moderation decision, deleted or not, and returns
// the row. An unknown video is ErrNotFound.
func (r *Videos) SetModeration(ctx context.Context, id uuid.UUID, status string, at time.Time) (*model.Video, error) {
	return scanVideo(r.pool.QueryRow(ctx, `UPDATE videos SET moderation_status = $2, moderated_at = $3, updated_at = now()
		WHERE id = $1 RETURNING `+videoColumns, id, status, at))
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
		&v.CreatedAt, &v.UpdatedAt, &v.DeletedAt, &v.ModerationStatus, &v.ModeratedAt, &v.MediaType,
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
