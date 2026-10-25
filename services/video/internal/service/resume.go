package service

import (
	"context"
	"errors"

	"github.com/google/uuid"

	"github.com/poro/video/internal/dto"
	"github.com/poro/video/internal/model"
	"github.com/poro/video/internal/repository"
	"github.com/poro/video/internal/storage"
)

// ResumeUpload lets the owner finish an interrupted video upload. Storage is
// the source of truth: the parts it holds at their exact length are returned
// with their ETag, every other part gets a fresh presigned URL. The row says
// whether the upload is still open: once completed, aborted or swept, the
// client has to start over.
func (s *Videos) ResumeUpload(ctx context.Context, userID, videoID uuid.UUID) (*dto.ResumeResponse, error) {
	row, err := s.repo.Get(ctx, videoID)
	if err != nil {
		if errors.Is(err, repository.ErrNotFound) {
			return nil, errNotFound
		}
		return nil, errInternal.WithCause(err)
	}
	if row.UserID != userID.String() || row.MediaType != model.MediaTypeVideo {
		return nil, errNotFound
	}
	if row.Status != model.StatusUploading || row.S3UploadID == nil {
		return nil, errConflict
	}
	uploaded, err := s.store.ListParts(ctx, row.SourceKey, *row.S3UploadID)
	if errors.Is(err, storage.ErrMissing) {
		return nil, errConflict
	}
	if err != nil {
		return nil, errInternal.WithCause(err)
	}
	n := model.PartCount(row.SizeBytes)
	held := make(map[int32]string, len(uploaded))
	for _, p := range uploaded {
		// A part of another length cannot have come through a URL we signed;
		// it is sent again, which replaces it.
		if p.Number >= 1 && int(p.Number) <= n && p.Size == model.PartLength(row.SizeBytes, int(p.Number)) {
			held[p.Number] = p.ETag
		}
	}
	out := &dto.ResumeResponse{
		VideoID: row.ID, UploadID: *row.S3UploadID, Key: row.SourceKey, PartSize: model.PartSize,
		UploadedParts: make([]dto.CompletePart, 0, len(held)), Parts: make([]dto.PartURL, 0, n-len(held)),
	}
	for i := 1; i <= n; i++ {
		num := int32(i)
		if etag, ok := held[num]; ok {
			out.UploadedParts = append(out.UploadedParts, dto.CompletePart{PartNumber: num, ETag: etag})
			continue
		}
		url, err := s.store.PresignPart(ctx, row.SourceKey, *row.S3UploadID, num, model.PartLength(row.SizeBytes, i))
		if err != nil {
			return nil, errInternal.WithCause(err)
		}
		out.Parts = append(out.Parts, dto.PartURL{PartNumber: num, URL: url})
	}
	return out, nil
}
