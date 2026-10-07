package service

import (
	"context"
	"errors"

	"github.com/google/uuid"
	"github.com/jackc/pgx/v5"

	"github.com/poro/shared-go/events"

	"github.com/poro/social/internal/cursor"
	"github.com/poro/social/internal/dto"
	"github.com/poro/social/internal/model"
	"github.com/poro/social/internal/repository"
	"github.com/poro/social/internal/text"
)

// CreateComment posts a comment on a ready video, or a reply to one of its
// top-level comments. Replies to a reply are refused.
func (s *Social) CreateComment(ctx context.Context, user, videoID uuid.UUID, req dto.CreateCommentRequest) (*dto.Comment, error) {
	content, ok := text.Comment(req.Content)
	if !ok {
		return nil, errCommentInvalid
	}
	var parentID *uuid.UUID
	if req.ParentID != nil {
		id, err := uuid.Parse(*req.ParentID)
		if err != nil {
			return nil, errParentInvalid
		}
		parentID = &id
	}

	comment := repository.Comment{ID: newID(), UserID: user, VideoID: videoID, ParentID: parentID, Content: content}
	err := s.inTx(ctx, func(tx pgx.Tx) error {
		owner, err := readyVideo(ctx, tx, videoID)
		if err != nil {
			return err
		}
		var parentAuthor *string
		if parentID != nil {
			parent, err := repository.LockComment(ctx, tx, *parentID)
			if errors.Is(err, repository.ErrNotFound) {
				return errCommentNotFound
			}
			if err != nil {
				return err
			}
			switch {
			case parent.VideoID != videoID:
				return errParentInvalid
			case parent.ParentID != nil:
				return errReplyDepth
			case parent.DeletedAt != nil:
				return errCommentNotFound
			}
			author := parent.UserID.String()
			parentAuthor = &author
			if err := repository.AddReplies(ctx, tx, parent.ID, 1); err != nil {
				return err
			}
		}
		if err := repository.EnsureUser(ctx, tx, user); err != nil {
			return err
		}
		now := s.clock()
		comment.CreatedAt, comment.UpdatedAt = now, now
		if err := repository.InsertComment(ctx, tx, comment); err != nil {
			return err
		}
		if err := repository.AddVideoCounters(ctx, tx, videoID, 0, 1, 0); err != nil {
			return err
		}
		return emit(ctx, tx, events.TypeSocialCommentCreated, videoID.String(), events.SocialCommentCreatedV1{
			CommentID: comment.ID.String(), UserID: user.String(), VideoID: videoID.String(),
			VideoOwnerID: owner.String(), ParentID: uuidString(parentID), ParentAuthorID: parentAuthor,
			Excerpt: text.Excerpt(content), CreatedAt: now,
		}, now)
	})
	if err != nil {
		return nil, err
	}
	return commentView(comment), nil
}

// EditComment replaces the text of the caller's comment within 15 minutes of
// posting. Editing publishes no event: consumers only see creations.
func (s *Social) EditComment(ctx context.Context, user, commentID uuid.UUID, req dto.EditCommentRequest) (*dto.Comment, error) {
	content, ok := text.Comment(req.Content)
	if !ok {
		return nil, errCommentInvalid
	}
	var edited *repository.Comment
	err := s.inTx(ctx, func(tx pgx.Tx) error {
		c, err := lockVisibleComment(ctx, tx, commentID)
		if err != nil {
			return err
		}
		if c.UserID != user {
			return errNotAuthor
		}
		now := s.clock()
		if now.Sub(c.CreatedAt) > model.EditWindow {
			return errEditWindow
		}
		if c.Content != content {
			if err := repository.UpdateCommentContent(ctx, tx, c.ID, content, now); err != nil {
				return err
			}
			c.Content, c.UpdatedAt = content, now
		}
		edited = c
		return nil
	})
	if err != nil {
		return nil, err
	}
	return s.comment(ctx, edited.ID, &user)
}

// DeleteComment soft-deletes a comment. Its author or the video owner may do
// it. Deleting an already deleted comment is accepted and changes nothing.
func (s *Social) DeleteComment(ctx context.Context, user, commentID uuid.UUID) error {
	return s.inTx(ctx, func(tx pgx.Tx) error {
		c, err := repository.LockComment(ctx, tx, commentID)
		if errors.Is(err, repository.ErrNotFound) {
			return errCommentNotFound
		}
		if err != nil {
			return err
		}
		owner, err := readyVideo(ctx, tx, c.VideoID)
		if err != nil {
			return err
		}
		if c.UserID != user && owner != user {
			if c.DeletedAt != nil {
				return errCommentNotFound
			}
			return errNotAuthor
		}
		if c.DeletedAt != nil {
			return nil
		}
		now := s.clock()
		if err := repository.SoftDeleteComment(ctx, tx, c.ID, now); err != nil {
			return err
		}
		if c.ParentID != nil {
			if err := repository.AddReplies(ctx, tx, *c.ParentID, -1); err != nil {
				return err
			}
		}
		if err := repository.AddVideoCounters(ctx, tx, c.VideoID, 0, -1, 0); err != nil {
			return err
		}
		return emit(ctx, tx, events.TypeSocialCommentDeleted, c.VideoID.String(), events.SocialCommentDeletedV1{
			CommentID: c.ID.String(), VideoID: c.VideoID.String(), UserID: c.UserID.String(), DeletedAt: now,
		}, now)
	})
}

// LikeComment likes a visible comment. Comment likes publish no event.
func (s *Social) LikeComment(ctx context.Context, user, commentID uuid.UUID) (*dto.CommentLikeState, error) {
	return s.toggleCommentLike(ctx, user, commentID, true)
}

// UnlikeComment removes a comment like.
func (s *Social) UnlikeComment(ctx context.Context, user, commentID uuid.UUID) (*dto.CommentLikeState, error) {
	return s.toggleCommentLike(ctx, user, commentID, false)
}

func (s *Social) toggleCommentLike(ctx context.Context, user, commentID uuid.UUID, like bool) (*dto.CommentLikeState, error) {
	out := &dto.CommentLikeState{CommentID: commentID.String(), LikedByMe: like}
	err := s.inTx(ctx, func(tx pgx.Tx) error {
		c, err := lockVisibleComment(ctx, tx, commentID)
		if err != nil {
			return err
		}
		if _, err := readyVideo(ctx, tx, c.VideoID); err != nil {
			return err
		}
		out.LikesCount = c.LikesCount
		var changed bool
		delta := -1
		if like {
			if err := repository.EnsureUser(ctx, tx, user); err != nil {
				return err
			}
			changed, err = repository.InsertCommentLike(ctx, tx, newID(), user, commentID, s.clock())
			delta = 1
		} else {
			changed, err = repository.DeleteCommentLike(ctx, tx, user, commentID)
		}
		if err != nil || !changed {
			return err
		}
		out.LikesCount, err = repository.AddCommentLikes(ctx, tx, commentID, delta)
		return err
	})
	if err != nil {
		return nil, err
	}
	return out, nil
}

// ListComments lists the top-level comments of a ready video, newest first.
func (s *Social) ListComments(ctx context.Context, videoID uuid.UUID, viewer *uuid.UUID, rawCursor string, limit int) (*dto.Page[dto.Comment], error) {
	after, err := decodeCursor(rawCursor)
	if err != nil {
		return nil, err
	}
	if _, err := readyVideo(ctx, s.pool, videoID); err != nil {
		return nil, asAPIError(err)
	}
	limit = model.PageSize(limit)
	rows, err := repository.ListTopComments(ctx, s.pool, videoID, viewerID(viewer), after, limit+1)
	if err != nil {
		return nil, errInternal.WithCause(err)
	}
	p := page(rows, limit, commentPosition, func(c repository.Comment) dto.Comment { return *commentView(c) })
	return &p, nil
}

// ListReplies lists the visible replies of a top-level comment, oldest first.
func (s *Social) ListReplies(ctx context.Context, commentID uuid.UUID, viewer *uuid.UUID, rawCursor string, limit int) (*dto.Page[dto.Comment], error) {
	after, err := decodeCursor(rawCursor)
	if err != nil {
		return nil, err
	}
	parent, err := repository.GetComment(ctx, s.pool, commentID, uuid.Nil)
	if errors.Is(err, repository.ErrNotFound) || (err == nil && parent.ParentID != nil) {
		return nil, errCommentNotFound
	}
	if err != nil {
		return nil, errInternal.WithCause(err)
	}
	if _, err := readyVideo(ctx, s.pool, parent.VideoID); err != nil {
		return nil, asAPIError(err)
	}
	limit = model.PageSize(limit)
	rows, err := repository.ListReplies(ctx, s.pool, commentID, viewerID(viewer), after, limit+1)
	if err != nil {
		return nil, errInternal.WithCause(err)
	}
	p := page(rows, limit, commentPosition, func(c repository.Comment) dto.Comment { return *commentView(c) })
	return &p, nil
}

func (s *Social) comment(ctx context.Context, id uuid.UUID, viewer *uuid.UUID) (*dto.Comment, error) {
	c, err := repository.GetComment(ctx, s.pool, id, viewerID(viewer))
	if err != nil {
		return nil, errInternal.WithCause(err)
	}
	return commentView(*c), nil
}

func lockVisibleComment(ctx context.Context, tx pgx.Tx, id uuid.UUID) (*repository.Comment, error) {
	c, err := repository.LockComment(ctx, tx, id)
	if errors.Is(err, repository.ErrNotFound) || (err == nil && c.DeletedAt != nil) {
		return nil, errCommentNotFound
	}
	return c, err
}

func commentView(c repository.Comment) *dto.Comment {
	v := &dto.Comment{
		ID: c.ID.String(), VideoID: c.VideoID.String(), ParentID: uuidString(c.ParentID),
		LikesCount: c.LikesCount, RepliesCount: c.RepliesCount, LikedByMe: c.LikedByMe,
		Edited: c.UpdatedAt.After(c.CreatedAt) && c.DeletedAt == nil, Deleted: c.DeletedAt != nil,
		CreatedAt: c.CreatedAt,
	}
	if c.DeletedAt == nil {
		author, content := c.UserID.String(), c.Content
		v.UserID, v.Content = &author, &content
	}
	return v
}

func commentPosition(c repository.Comment) cursor.Position {
	return cursor.Position{CreatedAt: c.CreatedAt, ID: c.ID}
}

func uuidString(id *uuid.UUID) *string {
	if id == nil {
		return nil
	}
	s := id.String()
	return &s
}
