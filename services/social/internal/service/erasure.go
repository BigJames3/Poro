package service

import (
	"context"
	"time"

	"github.com/google/uuid"
	"github.com/jackc/pgx/v5"

	"github.com/poro/social/internal/repository"
)

// EraseAccount forgets an erased account in the caller's transaction
// (ADR-0018). Its likes, comment likes, shares and follows in both directions
// are deleted with their counters and *.deleted events, its comments are
// soft-deleted (comment.deleted) and lose their text, and its projection and
// counters are deleted behind a tombstone. Only what still exists is touched,
// so a second call changes and publishes nothing.
func EraseAccount(ctx context.Context, tx pgx.Tx, userID uuid.UUID, at time.Time) error {
	now := at.UTC().Truncate(time.Microsecond)
	if err := repository.TombstoneUser(ctx, tx, userID, now); err != nil {
		return err
	}

	likes, err := repository.ListAllUserLikes(ctx, tx, userID)
	if err != nil {
		return err
	}
	for _, l := range likes {
		if err := removeLike(ctx, tx, userID, l.VideoID, l.OwnerID, now); err != nil {
			return err
		}
	}

	// Shares publish no deletion event: feed and search keep their totals.
	shares, err := repository.DeleteUserShares(ctx, tx, userID)
	if err != nil {
		return err
	}
	for _, s := range shares {
		if err := repository.AddVideoCounters(ctx, tx, s.VideoID, 0, 0, -s.Count); err != nil {
			return err
		}
	}

	// Comment likes publish no event, as when they are removed one by one.
	liked, err := repository.ListUserCommentLikes(ctx, tx, userID)
	if err != nil {
		return err
	}
	for _, commentID := range liked {
		removed, err := repository.DeleteCommentLike(ctx, tx, userID, commentID)
		if err != nil {
			return err
		}
		if removed {
			if _, err := repository.AddCommentLikes(ctx, tx, commentID, -1); err != nil {
				return err
			}
		}
	}

	// AddFollowCounters locks the two accounts of each follow in id order.
	follows, err := repository.ListAllUserFollows(ctx, tx, userID)
	if err != nil {
		return err
	}
	for _, f := range follows {
		if err := removeFollow(ctx, tx, f.FollowerID, f.FollowingID, now); err != nil {
			return err
		}
	}

	comments, err := repository.ListUserCommentIDs(ctx, tx, userID)
	if err != nil {
		return err
	}
	for _, commentID := range comments {
		if err := RemoveComment(ctx, tx, commentID, now); err != nil {
			return err
		}
	}
	if err := repository.ClearUserComments(ctx, tx, userID); err != nil {
		return err
	}
	return repository.DeleteUser(ctx, tx, userID)
}
