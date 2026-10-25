package service

import (
	"context"
	"errors"
	"strings"

	"github.com/gofiber/fiber/v2"
	"github.com/google/uuid"
	"go.uber.org/zap"

	"github.com/poro/shared-go/events"
	"github.com/poro/shared-go/httpx"
	"github.com/poro/shared-go/outbox"

	"github.com/poro/video/internal/dto"
	"github.com/poro/video/internal/media"
	"github.com/poro/video/internal/model"
	"github.com/poro/video/internal/repository"
	"github.com/poro/video/internal/storage"
	"github.com/poro/video/internal/text"
)

var (
	errImageCount   = httpx.NewAPIError(fiber.StatusUnprocessableEntity, "image_count_invalid", "a photo post has 1 to 10 images")
	errInvalidImage = httpx.NewAPIError(fiber.StatusUnprocessableEntity, "invalid_media", "file is not a supported image")
	errMissingImage = httpx.NewAPIError(fiber.StatusBadRequest, "upload_incomplete", "an image of the post was not uploaded")
)

// photoURLTTLSeconds is how long the image PUT URLs stay valid (storage.PresignPut).
const photoURLTTLSeconds = 3600

// InitPhotoUpload creates a photo post in `uploading` with its images and
// returns one presigned PUT per image (ADR-0020). The caption is optional.
func (s *Videos) InitPhotoUpload(ctx context.Context, userID uuid.UUID, req dto.InitPhotoRequest) (*dto.InitPhotoResponse, error) {
	desc, code := text.Description(req.Description)
	if code != "" {
		return nil, errDescription
	}
	if len(req.Images) < 1 || len(req.Images) > model.MaxImages {
		return nil, errImageCount
	}
	postID, err := uuid.NewV7()
	if err != nil {
		return nil, errInternal.WithCause(err)
	}
	images := make([]model.Image, len(req.Images))
	var total int64
	for i, spec := range req.Images {
		contentType := strings.ToLower(strings.TrimSpace(spec.ContentType))
		if !model.AllowedImageTypes[contentType] {
			return nil, errContentType
		}
		if spec.SizeBytes <= 0 {
			return nil, errSize
		}
		if spec.SizeBytes > model.MaxImageBytes {
			return nil, errTooLarge
		}
		images[i] = model.Image{
			Position: i + 1, ContentType: contentType, SizeBytes: spec.SizeBytes,
			SourceKey: model.ImageSourceKey(userID.String(), postID.String(), i+1),
		}
		total += spec.SizeBytes
	}
	now := s.now().UTC()
	row := &model.Video{
		ID: postID.String(), UserID: userID.String(), Description: desc, Status: model.StatusUploading,
		ContentType: model.PhotoContentType, SizeBytes: total,
		SourceKey: model.MediaPrefix(userID.String(), postID.String()), CreatedAt: now, UpdatedAt: now,
	}
	if err := s.repo.CreatePhoto(ctx, row, images); err != nil {
		return nil, errInternal.WithCause(err)
	}
	out := &dto.InitPhotoResponse{VideoID: postID.String(), ExpiresIn: photoURLTTLSeconds, Images: make([]dto.ImageUpload, len(images))}
	for i, img := range images {
		// A failure here leaves the post in `uploading`: the sweeper discards it.
		url, headers, err := s.store.PresignPut(ctx, img.SourceKey, img.ContentType, img.SizeBytes)
		if err != nil {
			return nil, errInternal.WithCause(err)
		}
		out.Images[i] = dto.ImageUpload{Position: img.Position, URL: url, Headers: headers}
	}
	return out, nil
}

// CompletePhotoUpload checks every image of the post, then sets `processing`
// and enqueues poro.video.uploaded in one transaction. Nothing is trusted but
// the stored bytes: each image must exist, keep its declared size and start
// with JPEG, PNG or WebP magic bytes matching its declared type.
//   - An image not uploaded yet: 400 upload_incomplete, nothing is deleted,
//     so the client sends the missing images and completes again.
//   - A storage failure: 500, nothing is deleted, the call can be repeated.
//   - An invalid image: the whole post and all its sources are deleted (no
//     partial post), 413 payload_too_large or 422 invalid_media.
//   - Called again once processing: 409 conflict, the post is untouched.
func (s *Videos) CompletePhotoUpload(ctx context.Context, userID, postID uuid.UUID) (*dto.VideoView, error) {
	row, err := s.ownedPost(ctx, userID, postID)
	if err != nil {
		return nil, err
	}
	if row.MediaType != model.MediaTypePhoto {
		return nil, errNotFound
	}
	if row.Status != model.StatusUploading {
		return nil, errConflict
	}
	images, err := s.repo.Images(ctx, row.ID)
	if err != nil {
		return nil, errInternal.WithCause(err)
	}
	if len(images) == 0 {
		return nil, errInternal.WithCause(errors.New("photo post without images"))
	}
	stored := make([]storage.Object, len(images))
	for i, img := range images {
		obj, err := s.store.HeadSource(ctx, img.SourceKey)
		if errors.Is(err, storage.ErrMissing) {
			return nil, errMissingImage
		}
		if err != nil {
			return nil, errInternal.WithCause(err)
		}
		stored[i] = obj
	}
	var total int64
	for i, img := range images {
		obj := stored[i]
		if obj.Size > model.MaxImageBytes {
			return nil, s.rejectPhoto(ctx, userID, postID, images, errTooLarge)
		}
		if obj.Size <= 0 || obj.Size != img.SizeBytes {
			return nil, s.rejectPhoto(ctx, userID, postID, images, errInvalidImage)
		}
		head, err := s.store.GetSourceRange(ctx, img.SourceKey, 0, 31)
		if err != nil {
			return nil, errInternal.WithCause(err)
		}
		if detected := media.DetectImageType(head); detected == "" || detected != img.ContentType {
			return nil, s.rejectPhoto(ctx, userID, postID, images, errInvalidImage)
		}
		total += obj.Size
	}

	now := s.now().UTC()
	tx, err := s.repo.Pool().Begin(ctx)
	if err != nil {
		return nil, errInternal.WithCause(err)
	}
	defer func() { _ = tx.Rollback(ctx) }()
	if err := s.repo.MarkProcessing(ctx, tx, postID, total); err != nil {
		if errors.Is(err, repository.ErrNotFound) {
			return nil, errConflict
		}
		return nil, errInternal.WithCause(err)
	}
	env, err := events.New(events.TypeVideoUploaded, 1, "video", postID.String(), events.VideoUploadedV1{
		VideoID: postID.String(), UserID: userID.String(), SourceKey: row.SourceKey,
		ContentType: model.PhotoContentType, SizeBytes: total, UploadedAt: now, MediaType: model.MediaTypePhoto,
	}, now)
	if err != nil {
		return nil, errInternal.WithCause(err)
	}
	if err := outbox.Enqueue(ctx, tx, env); err != nil {
		return nil, errInternal.WithCause(err)
	}
	if err := tx.Commit(ctx); err != nil {
		return nil, errInternal.WithCause(err)
	}
	row, err = s.repo.Get(ctx, postID)
	if err != nil {
		return nil, errInternal.WithCause(err)
	}
	return s.view(ctx, row), nil
}

// rejectPhoto deletes every source of a refused post, then the post, and
// returns why. A source that cannot be deleted leaves the post in
// `uploading`, so the sweeper retries the cleanup later.
func (s *Videos) rejectPhoto(ctx context.Context, userID, postID uuid.UUID, images []model.Image, why error) error {
	if err := s.deleteImageSources(ctx, images); err != nil {
		s.log.Warn("refused photo post kept for the sweeper", zap.String("video_id", postID.String()), zap.Error(err))
		return why
	}
	if err := s.dropRow(ctx, userID, postID); err != nil {
		s.log.Warn("drop refused photo post", zap.String("video_id", postID.String()), zap.Error(err))
	}
	return why
}

// deleteImageSources deletes the source of each image; a missing one counts
// as deleted. It stops at the first failure.
func (s *Videos) deleteImageSources(ctx context.Context, images []model.Image) error {
	for _, img := range images {
		if err := s.store.DeleteSource(ctx, img.SourceKey); err != nil {
			return err
		}
	}
	return nil
}

// ownedPost returns a live post of userID; any other is not found.
func (s *Videos) ownedPost(ctx context.Context, userID, postID uuid.UUID) (*model.Video, error) {
	row, err := s.repo.Get(ctx, postID)
	if err != nil {
		if errors.Is(err, repository.ErrNotFound) {
			return nil, errNotFound
		}
		return nil, errInternal.WithCause(err)
	}
	if row.UserID != userID.String() {
		return nil, errNotFound
	}
	return row, nil
}

// MediaTypes are the publications a client may receive: videos only, unless
// it declared the photo-posts capability.
func MediaTypes(photoPosts bool) []string {
	if photoPosts {
		return []string{model.MediaTypeVideo, model.MediaTypePhoto}
	}
	return []string{model.MediaTypeVideo}
}
