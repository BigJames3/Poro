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

// Follow makes follower follow a known account. A second call changes nothing.
func (s *Social) Follow(ctx context.Context, follower, following uuid.UUID) (*dto.UserStats, error) {
	if follower == following {
		return nil, errFollowSelf
	}
	err := s.inTx(ctx, func(tx pgx.Tx) error {
		if err := knownUser(ctx, tx, following); err != nil {
			return err
		}
		if err := repository.EnsureUser(ctx, tx, follower); err != nil {
			return err
		}
		now := s.clock()
		added, err := repository.InsertFollow(ctx, tx, repository.Follow{
			ID: newID(), FollowerID: follower, FollowingID: following, CreatedAt: now,
		})
		if err != nil || !added {
			return err
		}
		if err := repository.AddFollowCounters(ctx, tx, follower, following, 1); err != nil {
			return err
		}
		return emit(ctx, tx, events.TypeSocialFollowCreated, follower.String(), events.SocialFollowCreatedV1{
			FollowerID: follower.String(), FollowingID: following.String(), CreatedAt: now,
		}, now)
	})
	if err != nil {
		return nil, err
	}
	return s.UserStats(ctx, following, &follower)
}

// Unfollow stops following. Unfollowing an account not followed changes nothing.
func (s *Social) Unfollow(ctx context.Context, follower, following uuid.UUID) (*dto.UserStats, error) {
	if follower == following {
		return nil, errFollowSelf
	}
	err := s.inTx(ctx, func(tx pgx.Tx) error {
		if err := knownUser(ctx, tx, following); err != nil {
			return err
		}
		removed, err := repository.DeleteFollow(ctx, tx, follower, following)
		if err != nil || !removed {
			return err
		}
		if err := repository.AddFollowCounters(ctx, tx, follower, following, -1); err != nil {
			return err
		}
		now := s.clock()
		return emit(ctx, tx, events.TypeSocialFollowDeleted, follower.String(), events.SocialFollowDeletedV1{
			FollowerID: follower.String(), FollowingID: following.String(), DeletedAt: now,
		}, now)
	})
	if err != nil {
		return nil, err
	}
	return s.UserStats(ctx, following, &follower)
}

// UserStats returns the follow counters of a known account. viewer may be nil.
func (s *Social) UserStats(ctx context.Context, userID uuid.UUID, viewer *uuid.UUID) (*dto.UserStats, error) {
	if err := knownUser(ctx, s.pool, userID); err != nil {
		return nil, asAPIError(err)
	}
	c, err := repository.GetUserCounters(ctx, s.pool, userID)
	if err != nil {
		return nil, errInternal.WithCause(err)
	}
	out := &dto.UserStats{UserID: userID.String(), Followers: c.Followers, Following: c.Following}
	if viewer != nil && *viewer != userID {
		if out.FollowedByMe, err = repository.IsFollowing(ctx, s.pool, *viewer, userID); err != nil {
			return nil, errInternal.WithCause(err)
		}
	}
	return out, nil
}

// ListFollowers lists who follows a known account, newest first.
func (s *Social) ListFollowers(ctx context.Context, userID uuid.UUID, rawCursor string, limit int) (*dto.Page[dto.FollowEntry], error) {
	return s.listFollows(ctx, userID, rawCursor, limit, repository.ListFollowers, func(f repository.Follow) uuid.UUID { return f.FollowerID })
}

// ListFollowing lists who a known account follows, newest first.
func (s *Social) ListFollowing(ctx context.Context, userID uuid.UUID, rawCursor string, limit int) (*dto.Page[dto.FollowEntry], error) {
	return s.listFollows(ctx, userID, rawCursor, limit, repository.ListFollowing, func(f repository.Follow) uuid.UUID { return f.FollowingID })
}

type followLister func(context.Context, repository.DBTX, uuid.UUID, *cursor.Position, int) ([]repository.Follow, error)

func (s *Social) listFollows(ctx context.Context, userID uuid.UUID, rawCursor string, limit int, list followLister, other func(repository.Follow) uuid.UUID) (*dto.Page[dto.FollowEntry], error) {
	after, err := decodeCursor(rawCursor)
	if err != nil {
		return nil, err
	}
	if err := knownUser(ctx, s.pool, userID); err != nil {
		return nil, asAPIError(err)
	}
	limit = model.PageSize(limit)
	rows, err := list(ctx, s.pool, userID, after, limit+1)
	if err != nil {
		return nil, errInternal.WithCause(err)
	}
	p := page(rows, limit, func(f repository.Follow) cursor.Position {
		return cursor.Position{CreatedAt: f.CreatedAt, ID: f.ID}
	}, func(f repository.Follow) dto.FollowEntry {
		return dto.FollowEntry{UserID: other(f).String(), FollowedAt: f.CreatedAt}
	})
	return &p, nil
}
