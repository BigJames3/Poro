package worker

import (
	"bytes"
	"context"
	"errors"
	"fmt"
	"io"

	"github.com/google/uuid"
	"go.uber.org/zap"

	"github.com/poro/shared-go/events"
	"github.com/poro/shared-go/inbox"
	"github.com/poro/shared-go/outbox"

	"github.com/poro/video/internal/imaging"
	"github.com/poro/video/internal/model"
	"github.com/poro/video/internal/repository"
	"github.com/poro/video/internal/text"
)

// processed is one image ready to be written.
type processed struct {
	image model.Image
	out   *imaging.Result
}

// handlePhoto processes a photo post all or nothing (ADR-0020):
//  1. every source is read again and revalidated (size, magic bytes, type,
//     40 Mpx) before any decoding: the presigned PUT stays valid an hour, so
//     an image may have been replaced after complete;
//  2. only when every image is processed are the public objects written;
//  3. the post becomes ready, with its images, in the transaction of the
//     inbox claim and of poro.video.ready;
//  4. the originals, which may carry GPS data, are then deleted.
//
// One invalid image fails the whole post: nothing is published, the objects
// already written are quarantined and the sources are deleted.
func (h *Handler) handlePhoto(ctx context.Context, env events.Envelope, row *model.Video, videoID, userID uuid.UUID) error {
	images, err := h.repo.Images(ctx, row.ID)
	if err != nil {
		return err
	}
	if len(images) == 0 {
		return h.failPhoto(ctx, env, row, nil, events.VideoFailInvalid, errors.New("photo post without images"))
	}

	done := make([]processed, 0, len(images))
	for _, img := range images {
		raw, err := h.readSource(ctx, img)
		if err != nil {
			return h.failPhoto(ctx, env, row, images, events.VideoFailInvalid, fmt.Errorf("image %d: %w", img.Position, err))
		}
		out, err := imaging.Process(raw, img.ContentType, img.SizeBytes)
		if err != nil {
			return h.failPhoto(ctx, env, row, images, events.VideoFailInvalid, fmt.Errorf("image %d: %w", img.Position, err))
		}
		done = append(done, processed{image: img, out: out})
	}

	ready := make([]model.Image, len(done))
	eventImages := make([]events.VideoImageV1, len(done))
	for i, p := range done {
		imageKey := model.ImageKey(row.UserID, row.ID, p.image.Position)
		thumbKey := model.ImageThumbKey(row.UserID, row.ID, p.image.Position)
		if err := h.putMedia(ctx, imageKey, p.out.Image); err != nil {
			return h.failPhoto(ctx, env, row, images, events.VideoFailTranscode, err)
		}
		if err := h.putMedia(ctx, thumbKey, p.out.Thumbnail); err != nil {
			return h.failPhoto(ctx, env, row, images, events.VideoFailTranscode, err)
		}
		w, hgt := p.out.Width, p.out.Height
		ready[i] = p.image
		ready[i].ImageKey, ready[i].ThumbnailKey, ready[i].Width, ready[i].Height = &imageKey, &thumbKey, &w, &hgt
		eventImages[i] = events.VideoImageV1{ImageKey: imageKey, ThumbnailKey: thumbKey, Width: w, Height: hgt}
	}

	now := h.now().UTC()
	ev, err := events.New(events.TypeVideoReady, 1, "media-worker", row.ID, events.VideoReadyV1{
		VideoID: row.ID, UserID: row.UserID, Width: eventImages[0].Width, Height: eventImages[0].Height,
		ThumbnailKey: eventImages[0].ThumbnailKey, Renditions: []events.VideoRenditionV1{}, ReadyAt: now,
		Title: row.Title, Description: row.Description, Hashtags: text.Hashtags(row.Description),
		PublishedAt: now, MediaType: model.MediaTypePhoto, Images: eventImages,
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
		switch err := h.repo.MarkPhotoReady(ctx, tx, videoID, ready); {
		case errors.Is(err, repository.ErrNotFound):
			// Deleted or aborted while processing: nothing becomes public.
			if err := tx.Commit(ctx); err != nil {
				return err
			}
			if err := h.HideIfWithdrawn(ctx, videoID); err != nil {
				return err
			}
			h.deleteSources(ctx, row.ID, images)
			return nil
		case err != nil:
			return err
		}
		if err := outbox.Enqueue(ctx, tx, ev); err != nil {
			return err
		}
	}
	if err := tx.Commit(ctx); err != nil {
		return err
	}
	if first {
		h.log.Info("photo post ready", zap.String("video_id", row.ID), zap.Int("images", len(ready)))
	}
	if err := h.HideIfWithdrawn(ctx, videoID); err != nil {
		return err
	}
	h.deleteSources(ctx, row.ID, images)
	return nil
}

// readSource reads one original with the storage retries, never more than
// the photo limit plus one byte, so an oversized replacement is caught
// without being read whole.
func (h *Handler) readSource(ctx context.Context, img model.Image) ([]byte, error) {
	var buf bytes.Buffer
	tooLarge := false
	err := h.retry(ctx, "get image source", func() error {
		buf.Reset()
		err := h.store.GetSource(ctx, img.SourceKey, &limitedWriter{w: &buf, left: model.MaxImageBytes + 1})
		if errors.Is(err, errTooLarge) {
			tooLarge = true // final: reading again would not shrink it
			return nil
		}
		return err
	})
	if tooLarge {
		return nil, fmt.Errorf("%w: over %d bytes", imaging.ErrInvalid, model.MaxImageBytes)
	}
	return buf.Bytes(), err
}

func (h *Handler) putMedia(ctx context.Context, key string, body []byte) error {
	return h.retry(ctx, "put image", func() error {
		return h.store.PutMedia(ctx, key, "image/jpeg", bytes.NewReader(body))
	})
}

// failPhoto marks the post failed, quarantines any public object already
// written and deletes the originals. The failure is committed first: a post
// is never left processing because a cleanup failed.
func (h *Handler) failPhoto(ctx context.Context, env events.Envelope, row *model.Video, images []model.Image, code string, cause error) error {
	videoID, userID := uuid.MustParse(row.ID), uuid.MustParse(row.UserID)
	if err := h.fail(ctx, env, videoID, userID, code, cause); err != nil {
		return err
	}
	if _, err := h.store.Hide(ctx, model.MediaPrefix(row.UserID, row.ID)); err != nil {
		h.log.Warn("quarantine media of a failed photo post", zap.String("video_id", row.ID), zap.Error(err))
	}
	h.deleteSources(ctx, row.ID, images)
	return nil
}

// deleteSources deletes the originals once a post is settled. A failure is
// logged: the object stays private, and account erasure deletes it.
func (h *Handler) deleteSources(ctx context.Context, videoID string, images []model.Image) {
	for _, img := range images {
		if err := h.retry(ctx, "delete image source", func() error {
			return h.store.DeleteSource(ctx, img.SourceKey)
		}); err != nil {
			h.log.Warn("delete photo source", zap.String("video_id", videoID), zap.Int("position", img.Position), zap.Error(err))
		}
	}
}

var errTooLarge = errors.New("source larger than allowed")

// limitedWriter fails once more than left bytes were written.
type limitedWriter struct {
	w    io.Writer
	left int64
}

func (l *limitedWriter) Write(p []byte) (int, error) {
	if int64(len(p)) > l.left {
		return 0, errTooLarge
	}
	l.left -= int64(len(p))
	return l.w.Write(p)
}
