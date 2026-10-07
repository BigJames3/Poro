package service

import (
	"context"
	"errors"
	"strings"
	"time"

	"github.com/gofiber/fiber/v2"
	"github.com/google/uuid"
	"go.uber.org/zap"

	"github.com/poro/shared-go/events"
	"github.com/poro/shared-go/httpx"
	"github.com/poro/shared-go/outbox"

	"github.com/poro/video/internal/config"
	"github.com/poro/video/internal/cursor"
	"github.com/poro/video/internal/dto"
	"github.com/poro/video/internal/media"
	"github.com/poro/video/internal/model"
	"github.com/poro/video/internal/repository"
	"github.com/poro/video/internal/storage"
	"github.com/poro/video/internal/text"
)

var (
	errTitle        = httpx.NewAPIError(fiber.StatusUnprocessableEntity, "title_invalid", "title is invalid")
	errDescription  = httpx.NewAPIError(fiber.StatusUnprocessableEntity, "description_invalid", "description is invalid")
	errContentType  = httpx.NewAPIError(fiber.StatusUnprocessableEntity, "content_type_invalid", "content type is not supported")
	errSize         = httpx.NewAPIError(fiber.StatusUnprocessableEntity, "size_invalid", "file size is invalid")
	errTooLarge     = httpx.NewAPIError(fiber.StatusRequestEntityTooLarge, "payload_too_large", "file is too large")
	errInvalidMedia = httpx.NewAPIError(fiber.StatusUnprocessableEntity, "invalid_media", "file is not a supported video")
	errNotFound     = httpx.NewAPIError(fiber.StatusNotFound, "not_found", "not found")
	errConflict     = httpx.NewAPIError(fiber.StatusConflict, "conflict", "upload is not in a completable state")
	errIncomplete   = httpx.NewAPIError(fiber.StatusBadRequest, "upload_incomplete", "multipart parts are incomplete")
	errCursor       = httpx.NewAPIError(fiber.StatusBadRequest, "invalid_request", "cursor is invalid")
	errInternal     = httpx.NewAPIError(fiber.StatusInternalServerError, "internal_error", "internal server error")
)

// Videos is the video API use-cases.
type Videos struct {
	cfg    *config.Config
	repo   *repository.Videos
	store  storage.Storage
	public func(string) string
	log    *zap.Logger
	now    func() time.Time
}

// NewVideos builds the service.
func NewVideos(cfg *config.Config, repo *repository.Videos, store storage.Storage, log *zap.Logger) *Videos {
	return &Videos{cfg: cfg, repo: repo, store: store, public: cfg.PublicObjectURL, log: log, now: time.Now}
}

// InitUpload creates the row and returns presigned part URLs.
func (s *Videos) InitUpload(ctx context.Context, userID uuid.UUID, req dto.InitRequest) (*dto.InitResponse, error) {
	title, code := text.Title(req.Title)
	if code != "" {
		return nil, errTitle
	}
	desc, code := text.Description(req.Description)
	if code != "" {
		return nil, errDescription
	}
	ext, ok := model.AllowedContentTypes[strings.ToLower(strings.TrimSpace(req.ContentType))]
	if !ok {
		return nil, errContentType
	}
	if req.SizeBytes <= 0 {
		return nil, errSize
	}
	if req.SizeBytes > model.MaxBytes {
		return nil, errTooLarge
	}
	videoID, err := uuid.NewV7()
	if err != nil {
		return nil, errInternal.WithCause(err)
	}
	key := model.SourceKey(userID.String(), videoID.String(), ext)
	uploadID, err := s.store.CreateMultipart(ctx, key, strings.ToLower(strings.TrimSpace(req.ContentType)))
	if err != nil {
		return nil, errInternal.WithCause(err)
	}
	now := s.now().UTC()
	uid := uploadID
	row := &model.Video{
		ID: videoID.String(), UserID: userID.String(), Title: title, Description: desc,
		Status: model.StatusUploading, ContentType: strings.ToLower(strings.TrimSpace(req.ContentType)),
		SizeBytes: req.SizeBytes, SourceKey: key, S3UploadID: &uid, CreatedAt: now, UpdatedAt: now,
	}
	if err := s.repo.Create(ctx, row); err != nil {
		_ = s.store.AbortMultipart(ctx, key, uploadID)
		return nil, errInternal.WithCause(err)
	}
	n := model.PartCount(req.SizeBytes)
	parts := make([]dto.PartURL, n)
	for i := 0; i < n; i++ {
		num := int32(i + 1)
		url, err := s.store.PresignPart(ctx, key, uploadID, num)
		if err != nil {
			return nil, errInternal.WithCause(err)
		}
		parts[i] = dto.PartURL{PartNumber: num, URL: url}
	}
	return &dto.InitResponse{
		VideoID: videoID.String(), UploadID: uploadID, Key: key, PartSize: model.PartSize, Parts: parts,
	}, nil
}

// Complete finishes multipart upload, checks magic bytes, and enqueues poro.video.uploaded.
func (s *Videos) Complete(ctx context.Context, userID, videoID uuid.UUID, req dto.CompleteRequest) (*dto.VideoView, error) {
	row, err := s.repo.Get(ctx, videoID)
	if err != nil {
		if errors.Is(err, repository.ErrNotFound) {
			return nil, errNotFound
		}
		return nil, errInternal.WithCause(err)
	}
	if row.UserID != userID.String() {
		return nil, errNotFound
	}
	if row.Status != model.StatusUploading || row.S3UploadID == nil {
		return nil, errConflict
	}
	want := model.PartCount(row.SizeBytes)
	if len(req.Parts) != want {
		return nil, errIncomplete
	}
	seen := map[int32]struct{}{}
	parts := make([]storage.CompletedPart, 0, len(req.Parts))
	for _, p := range req.Parts {
		if p.PartNumber < 1 || p.PartNumber > int32(want) || strings.TrimSpace(p.ETag) == "" {
			return nil, errIncomplete
		}
		if _, dup := seen[p.PartNumber]; dup {
			return nil, errIncomplete
		}
		seen[p.PartNumber] = struct{}{}
		parts = append(parts, storage.CompletedPart{Number: p.PartNumber, ETag: strings.TrimSpace(p.ETag)})
	}
	if err := s.store.CompleteMultipart(ctx, row.SourceKey, *row.S3UploadID, parts); err != nil {
		return nil, errInternal.WithCause(err)
	}
	obj, err := s.store.Head(ctx, row.SourceKey)
	if err != nil {
		return nil, errInternal.WithCause(err)
	}
	if obj.Size <= 0 {
		_ = s.store.Delete(ctx, row.SourceKey)
		_ = s.dropRow(ctx, userID, videoID)
		return nil, errInvalidMedia
	}
	if obj.Size > model.MaxBytes {
		_ = s.store.Delete(ctx, row.SourceKey)
		_ = s.dropRow(ctx, userID, videoID)
		return nil, errTooLarge
	}
	head, err := s.store.GetRange(ctx, row.SourceKey, 0, 31)
	if err != nil || !media.LooksLikeVideo(head) {
		_ = s.store.Delete(ctx, row.SourceKey)
		_ = s.dropRow(ctx, userID, videoID)
		return nil, errInvalidMedia
	}

	now := s.now().UTC()
	tx, err := s.repo.Pool().Begin(ctx)
	if err != nil {
		return nil, errInternal.WithCause(err)
	}
	defer func() { _ = tx.Rollback(ctx) }()
	if err := s.repo.MarkProcessing(ctx, tx, videoID, obj.Size); err != nil {
		if errors.Is(err, repository.ErrNotFound) {
			return nil, errConflict
		}
		return nil, errInternal.WithCause(err)
	}
	env, err := events.New(events.TypeVideoUploaded, 1, "video", videoID.String(), events.VideoUploadedV1{
		VideoID: videoID.String(), UserID: userID.String(), SourceKey: row.SourceKey,
		ContentType: row.ContentType, SizeBytes: obj.Size, UploadedAt: now,
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
	row, err = s.repo.Get(ctx, videoID)
	if err != nil {
		return nil, errInternal.WithCause(err)
	}
	return s.view(row, true), nil
}

// Abort cancels an in-progress multipart upload.
func (s *Videos) Abort(ctx context.Context, userID, videoID uuid.UUID) error {
	row, err := s.repo.Get(ctx, videoID)
	if err != nil {
		if errors.Is(err, repository.ErrNotFound) {
			return errNotFound
		}
		return errInternal.WithCause(err)
	}
	if row.UserID != userID.String() {
		return errNotFound
	}
	if row.Status != model.StatusUploading {
		return errConflict
	}
	if row.S3UploadID != nil {
		_ = s.store.AbortMultipart(ctx, row.SourceKey, *row.S3UploadID)
	}
	tx, err := s.repo.Pool().Begin(ctx)
	if err != nil {
		return errInternal.WithCause(err)
	}
	defer func() { _ = tx.Rollback(ctx) }()
	if _, err := s.repo.SoftDelete(ctx, tx, videoID, userID); err != nil {
		if errors.Is(err, repository.ErrNotFound) {
			return errNotFound
		}
		return errInternal.WithCause(err)
	}
	return tx.Commit(ctx)
}

// Get returns a video. Non-ready videos are only visible to the owner.
func (s *Videos) Get(ctx context.Context, id uuid.UUID, viewer *uuid.UUID) (*dto.VideoView, error) {
	row, err := s.repo.Get(ctx, id)
	if err != nil {
		if errors.Is(err, repository.ErrNotFound) {
			return nil, errNotFound
		}
		return nil, errInternal.WithCause(err)
	}
	owner := viewer != nil && viewer.String() == row.UserID
	if row.Status != model.StatusReady && !owner {
		return nil, errNotFound
	}
	return s.view(row, owner), nil
}

// List returns the caller's videos.
func (s *Videos) List(ctx context.Context, userID uuid.UUID, rawCursor string, limit int) (*dto.ListResponse, error) {
	if limit <= 0 {
		limit = model.DefaultListLimit
	}
	if limit > model.MaxListLimit {
		limit = model.MaxListLimit
	}
	var ts *time.Time
	var cid *uuid.UUID
	if rawCursor != "" {
		t, id, err := cursor.Decode(rawCursor)
		if err != nil {
			return nil, errCursor
		}
		ts, cid = &t, &id
	}
	rows, err := s.repo.ListByUser(ctx, userID, ts, cid, limit+1)
	if err != nil {
		return nil, errInternal.WithCause(err)
	}
	out := &dto.ListResponse{Items: make([]dto.VideoView, 0, len(rows))}
	for i, row := range rows {
		if i == limit {
			c := cursor.Encode(rows[limit-1].CreatedAt, uuid.MustParse(rows[limit-1].ID))
			out.NextCursor = &c
			break
		}
		out.Items = append(out.Items, *s.view(&row, true))
	}
	return out, nil
}

// Delete soft-deletes the owner's video and aborts an unfinished upload.
func (s *Videos) Delete(ctx context.Context, userID, videoID uuid.UUID) error {
	row, err := s.repo.Get(ctx, videoID)
	if err != nil {
		if errors.Is(err, repository.ErrNotFound) {
			return errNotFound
		}
		return errInternal.WithCause(err)
	}
	if row.UserID != userID.String() {
		return errNotFound
	}
	if row.Status == model.StatusUploading && row.S3UploadID != nil {
		_ = s.store.AbortMultipart(ctx, row.SourceKey, *row.S3UploadID)
	}
	tx, err := s.repo.Pool().Begin(ctx)
	if err != nil {
		return errInternal.WithCause(err)
	}
	defer func() { _ = tx.Rollback(ctx) }()
	deleted, err := s.repo.SoftDelete(ctx, tx, videoID, userID)
	if err != nil {
		if errors.Is(err, repository.ErrNotFound) {
			return errNotFound
		}
		return errInternal.WithCause(err)
	}
	deletedAt := s.now().UTC()
	if deleted.DeletedAt != nil {
		deletedAt = deleted.DeletedAt.UTC()
	}
	env, err := events.New(events.TypeVideoDeleted, 1, "video", videoID.String(), events.VideoDeletedV1{
		VideoID: videoID.String(), UserID: userID.String(), DeletedAt: deletedAt,
	}, deletedAt)
	if err != nil {
		return errInternal.WithCause(err)
	}
	if err := outbox.Enqueue(ctx, tx, env); err != nil {
		return errInternal.WithCause(err)
	}
	return tx.Commit(ctx)
}

func (s *Videos) view(row *model.Video, owner bool) *dto.VideoView {
	v := &dto.VideoView{
		ID: row.ID, UserID: row.UserID, Title: row.Title, Description: row.Description,
		Status: row.Status, CreatedAt: row.CreatedAt,
	}
	if row.Status == model.StatusReady {
		v.DurationMs, v.Width, v.Height = row.DurationMs, row.Width, row.Height
		if row.HLSKey != nil {
			u := s.public(*row.HLSKey)
			v.HLSURL = &u
		}
		if row.ThumbnailKey != nil {
			u := s.public(*row.ThumbnailKey)
			v.ThumbnailURL = &u
		}
		for _, r := range row.Renditions {
			v.Renditions = append(v.Renditions, dto.RenditionView{
				Name: r.Name, Bandwidth: r.Bandwidth, Width: r.Width, Height: r.Height,
			})
		}
	}
	_ = owner
	return v
}

func (s *Videos) dropRow(ctx context.Context, userID, videoID uuid.UUID) error {
	tx, err := s.repo.Pool().Begin(ctx)
	if err != nil {
		return err
	}
	defer func() { _ = tx.Rollback(ctx) }()
	if _, err := s.repo.SoftDelete(ctx, tx, videoID, userID); err != nil {
		return err
	}
	return tx.Commit(ctx)
}
