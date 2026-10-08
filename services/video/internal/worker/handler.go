package worker

import (
	"bytes"
	"context"
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"time"

	"github.com/google/uuid"
	"github.com/jackc/pgx/v5/pgxpool"
	"go.uber.org/zap"

	"github.com/poro/shared-go/events"
	"github.com/poro/shared-go/inbox"
	"github.com/poro/shared-go/kafka"
	"github.com/poro/shared-go/outbox"

	"github.com/poro/video/internal/model"
	"github.com/poro/video/internal/repository"
	"github.com/poro/video/internal/storage"
	"github.com/poro/video/internal/text"
	"github.com/poro/video/internal/transcode"
)

// Group is the Kafka consumer group that transcodes uploads.
const Group = "poro-media-transcode"

// Handler downloads a source object, transcodes it, and writes ready/failed
// in the same transaction as the inbox claim.
type Handler struct {
	pool   *pgxpool.Pool
	repo   *repository.Videos
	store  storage.Storage
	ffmpeg transcode.Transcoder
	log    *zap.Logger
	now    func() time.Time
}

// New builds a Kafka handler.
func New(pool *pgxpool.Pool, repo *repository.Videos, store storage.Storage, ffmpeg transcode.Transcoder, log *zap.Logger) *Handler {
	return &Handler{pool: pool, repo: repo, store: store, ffmpeg: ffmpeg, log: log, now: time.Now}
}

// Handle implements kafka.Handler.
func (h *Handler) Handle(ctx context.Context, env events.Envelope) error {
	if env.Type != events.TypeVideoUploaded || env.Version != 1 {
		return kafka.Permanent(fmt.Errorf("unsupported event %s v%d", env.Type, env.Version))
	}
	var data events.VideoUploadedV1
	if err := env.DecodeData(&data); err != nil {
		return kafka.Permanent(err)
	}
	videoID, err := uuid.Parse(data.VideoID)
	if err != nil {
		return kafka.Permanent(fmt.Errorf("video_id: %w", err))
	}
	userID, err := uuid.Parse(data.UserID)
	if err != nil {
		return kafka.Permanent(fmt.Errorf("user_id: %w", err))
	}

	row, err := h.repo.GetAny(ctx, videoID)
	if err != nil {
		if errors.Is(err, repository.ErrNotFound) {
			return kafka.Permanent(err)
		}
		return err
	}
	if row.DeletedAt != nil || row.Status == model.StatusReady || row.Status == model.StatusFailed {
		if err := h.claimOnly(ctx, env.ID); err != nil {
			return err
		}
		return h.HideIfWithdrawn(ctx, videoID)
	}
	if row.Status != model.StatusProcessing {
		return kafka.Permanent(fmt.Errorf("unexpected status %s", row.Status))
	}

	work, err := os.MkdirTemp("", "poro-video-")
	if err != nil {
		return err
	}
	defer func() { _ = os.RemoveAll(work) }()

	srcPath := filepath.Join(work, "source")
	srcFile, err := os.Create(srcPath)
	if err != nil {
		return err
	}
	if err := h.store.Get(ctx, data.SourceKey, srcFile); err != nil {
		_ = srcFile.Close()
		return h.fail(ctx, env, videoID, userID, events.VideoFailInvalid, err)
	}
	if err := srcFile.Close(); err != nil {
		return err
	}

	outDir := filepath.Join(work, "out")
	if err := os.MkdirAll(outDir, 0o755); err != nil {
		return err
	}
	out, err := h.ffmpeg.Transcode(ctx, srcPath, outDir, userID.String(), videoID.String())
	if err != nil {
		return h.fail(ctx, env, videoID, userID, failCode(err), err)
	}
	if err := h.uploadOutput(ctx, outDir, userID.String(), videoID.String(), out); err != nil {
		return h.fail(ctx, env, videoID, userID, events.VideoFailTranscode, err)
	}

	now := h.now().UTC()
	hlsKey := model.HLSDir(userID.String(), videoID.String()) + "/master.m3u8"
	thumbKey := model.ThumbKey(userID.String(), videoID.String())
	ready := events.VideoReadyV1{
		VideoID: videoID.String(), UserID: userID.String(), DurationMs: out.DurationMs,
		Width: out.Width, Height: out.Height, HLSKey: hlsKey, ThumbnailKey: thumbKey,
		Renditions: toEventRenditions(out.Renditions), ReadyAt: now,
		Title: row.Title, Description: row.Description, Hashtags: text.Hashtags(row.Description),
		PublishedAt: now,
	}
	ev, err := events.New(events.TypeVideoReady, 1, "media-worker", videoID.String(), ready, now)
	if err != nil {
		return err
	}

	tx, err := h.pool.Begin(ctx)
	if err != nil {
		return err
	}
	defer func() { _ = tx.Rollback(ctx) }()
	first, err := inbox.Claim(ctx, tx, Group, env.ID)
	if err != nil {
		return err
	}
	if first {
		if err := h.repo.MarkReady(ctx, tx, videoID, out.DurationMs, out.Width, out.Height, hlsKey, thumbKey, out.Renditions); err != nil {
			if errors.Is(err, repository.ErrNotFound) {
				// Deleted while transcoding: the uploads must not stay public.
				if err := tx.Commit(ctx); err != nil {
					return err
				}
				return h.HideIfWithdrawn(ctx, videoID)
			}
			return err
		}
		if err := outbox.Enqueue(ctx, tx, ev); err != nil {
			return err
		}
		h.log.Info("video ready", zap.String("video_id", videoID.String()), zap.Int("duration_ms", out.DurationMs))
	}
	if err := tx.Commit(ctx); err != nil {
		return err
	}
	return h.HideIfWithdrawn(ctx, videoID)
}

// HideIfWithdrawn moves the media of a video removed by moderation or deleted
// by its owner while it was transcoding. The video API commits the status
// before it moves media, so either this read sees the status or the API's
// move runs after this worker's uploads.
func (h *Handler) HideIfWithdrawn(ctx context.Context, videoID uuid.UUID) error {
	row, err := h.repo.GetAny(ctx, videoID)
	if errors.Is(err, repository.ErrNotFound) {
		return nil
	}
	if err != nil {
		return err
	}
	if row.DeletedAt == nil && row.ModerationStatus != model.ModerationRemoved {
		return nil
	}
	n, err := h.store.Hide(ctx, model.MediaPrefix(row.UserID, row.ID))
	if err != nil {
		return fmt.Errorf("hide media: %w", err)
	}
	if n > 0 {
		h.log.Info("withdrawn video media quarantined", zap.String("video_id", row.ID), zap.Int("objects", n))
	}
	return nil
}

func (h *Handler) uploadOutput(ctx context.Context, outDir, userID, videoID string, out *transcode.Output) error {
	for _, f := range out.Files {
		path := filepath.Join(outDir, filepath.FromSlash(f.Rel))
		raw, err := os.ReadFile(path)
		if err != nil {
			return err
		}
		key := objectKey(userID, videoID, f.Rel)
		if err := h.store.Put(ctx, key, f.ContentType, bytes.NewReader(raw)); err != nil {
			return err
		}
	}
	return nil
}

func objectKey(userID, videoID, rel string) string {
	rel = filepath.ToSlash(rel)
	switch rel {
	case "thumb.jpg":
		return model.ThumbKey(userID, videoID)
	default:
		return model.HLSDir(userID, videoID) + "/" + strings.TrimPrefix(rel, "hls/")
	}
}

func (h *Handler) fail(ctx context.Context, env events.Envelope, videoID, userID uuid.UUID, code string, cause error) error {
	now := h.now().UTC()
	ev, err := events.New(events.TypeVideoFailed, 1, "media-worker", videoID.String(), events.VideoFailedV1{
		VideoID: videoID.String(), UserID: userID.String(), Code: code, FailedAt: now,
	}, now)
	if err != nil {
		return err
	}
	tx, err := h.pool.Begin(ctx)
	if err != nil {
		return err
	}
	defer func() { _ = tx.Rollback(ctx) }()
	first, err := inbox.Claim(ctx, tx, Group, env.ID)
	if err != nil {
		return err
	}
	if first {
		internal := ""
		if cause != nil {
			internal = cause.Error()
			if len(internal) > 500 {
				internal = internal[:500]
			}
		}
		if err := h.repo.MarkFailed(ctx, tx, videoID, code, internal); err != nil && !errors.Is(err, repository.ErrNotFound) {
			return err
		}
		if err := outbox.Enqueue(ctx, tx, ev); err != nil {
			return err
		}
		h.log.Info("video failed", zap.String("video_id", videoID.String()), zap.String("code", code), zap.Error(cause))
	}
	return tx.Commit(ctx)
}

func (h *Handler) claimOnly(ctx context.Context, eventID uuid.UUID) error {
	tx, err := h.pool.Begin(ctx)
	if err != nil {
		return err
	}
	defer func() { _ = tx.Rollback(ctx) }()
	if _, err := inbox.Claim(ctx, tx, Group, eventID); err != nil {
		return err
	}
	return tx.Commit(ctx)
}

func failCode(err error) string {
	switch {
	case errors.Is(err, transcode.ErrTooLong):
		return events.VideoFailTooLong
	case errors.Is(err, transcode.ErrInvalid):
		return events.VideoFailInvalid
	default:
		return events.VideoFailTranscode
	}
}

func toEventRenditions(in []model.Rendition) []events.VideoRenditionV1 {
	out := make([]events.VideoRenditionV1, len(in))
	for i, r := range in {
		out[i] = events.VideoRenditionV1{
			Name: r.Name, Bandwidth: r.Bandwidth, Width: r.Width, Height: r.Height, PlaylistKey: r.PlaylistKey,
		}
	}
	return out
}
