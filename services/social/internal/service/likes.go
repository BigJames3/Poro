package service

import (
	"context"

	"github.com/google/uuid"
	"github.com/jackc/pgx/v5"

	"github.com/poro/shared-go/events"

	"github.com/poro/social/internal/cursor"
	"github.com/poro/social/internal/dto"
	"github.com/poro/social/internal/model"
	"github.com/poro/social/internal/repository"
)

// LikeVideo likes a ready video. A second call changes nothing and publishes nothing.
func (s *Social) LikeVideo(ctx context.Context, user, videoID uuid.UUID) (*dto.VideoStats, error) {
	err := s.inTx(ctx, func(tx pgx.Tx) error {
		owner, err := readyVideo(ctx, tx, videoID)
		if err != nil {
			return err
		}
		if err := repository.EnsureUser(ctx, tx, user); err != nil {
			return err
		}
		now := s.clock()
		like := repository.Like{ID: newID(), UserID: user, VideoID: videoID, CreatedAt: now}
		added, err := repository.InsertLike(ctx, tx, like)
		if err != nil || !added {
			return err
		}
		if err := repository.AddVideoCounters(ctx, tx, videoID, 1, 0, 0); err != nil {
			return err
		}
		return emit(ctx, tx, events.TypeSocialLikeCreated, videoID.String(), events.SocialLikeCreatedV1{
			LikeID: like.ID.String(), UserID: user.String(), VideoID: videoID.String(),
			VideoOwnerID: owner.String(), CreatedAt: now,
		}, now)
	})
	if err != nil {
		return nil, err
	}
	return s.VideoStats(ctx, videoID, &user)
}

// UnlikeVideo removes a like. Removing a missing like changes nothing.
func (s *Social) UnlikeVideo(ctx context.Context, user, videoID uuid.UUID) (*dto.VideoStats, error) {
	err := s.inTx(ctx, func(tx pgx.Tx) error {
		owner, err := readyVideo(ctx, tx, videoID)
		if err != nil {
			return err
		}
		removed, err := repository.DeleteLike(ctx, tx, user, videoID)
		if err != nil || !removed {
			return err
		}
		if err := repository.AddVideoCounters(ctx, tx, videoID, -1, 0, 0); err != nil {
			return err
		}
		now := s.clock()
		return emit(ctx, tx, events.TypeSocialLikeDeleted, videoID.String(), events.SocialLikeDeletedV1{
			UserID: user.String(), VideoID: videoID.String(), VideoOwnerID: owner.String(), DeletedAt: now,
		}, now)
	})
	if err != nil {
		return nil, err
	}
	return s.VideoStats(ctx, videoID, &user)
}

// VideoStats returns the counters of a ready video. viewer may be nil.
func (s *Social) VideoStats(ctx context.Context, videoID uuid.UUID, viewer *uuid.UUID) (*dto.VideoStats, error) {
	if _, err := readyVideo(ctx, s.pool, videoID); err != nil {
		return nil, asAPIError(err)
	}
	c, err := repository.GetVideoCounters(ctx, s.pool, videoID)
	if err != nil {
		return nil, errInternal.WithCause(err)
	}
	out := &dto.VideoStats{VideoID: videoID.String(), Likes: c.Likes, Comments: c.Comments, Shares: c.Shares}
	if viewer != nil {
		if out.LikedByMe, err = repository.HasLiked(ctx, s.pool, *viewer, videoID); err != nil {
			return nil, errInternal.WithCause(err)
		}
	}
	return out, nil
}

// ListVideoLikes lists who likes a ready video, newest first.
func (s *Social) ListVideoLikes(ctx context.Context, videoID uuid.UUID, rawCursor string, limit int) (*dto.Page[dto.VideoLike], error) {
	after, err := decodeCursor(rawCursor)
	if err != nil {
		return nil, err
	}
	if _, err := readyVideo(ctx, s.pool, videoID); err != nil {
		return nil, asAPIError(err)
	}
	limit = model.PageSize(limit)
	rows, err := repository.ListVideoLikes(ctx, s.pool, videoID, after, limit+1)
	if err != nil {
		return nil, errInternal.WithCause(err)
	}
	p := page(rows, limit, likePosition, func(l repository.Like) dto.VideoLike {
		return dto.VideoLike{UserID: l.UserID.String(), LikedAt: l.CreatedAt}
	})
	return &p, nil
}

// ListUserLikes lists the ready videos a user likes, newest like first.
func (s *Social) ListUserLikes(ctx context.Context, userID uuid.UUID, rawCursor string, limit int) (*dto.Page[dto.LikedVideo], error) {
	after, err := decodeCursor(rawCursor)
	if err != nil {
		return nil, err
	}
	if err := knownUser(ctx, s.pool, userID); err != nil {
		return nil, asAPIError(err)
	}
	limit = model.PageSize(limit)
	rows, err := repository.ListUserLikes(ctx, s.pool, userID, after, limit+1)
	if err != nil {
		return nil, errInternal.WithCause(err)
	}
	p := page(rows, limit, likePosition, func(l repository.Like) dto.LikedVideo {
		return dto.LikedVideo{VideoID: l.VideoID.String(), LikedAt: l.CreatedAt}
	})
	return &p, nil
}

func likePosition(l repository.Like) cursor.Position {
	return cursor.Position{CreatedAt: l.CreatedAt, ID: l.ID}
}
