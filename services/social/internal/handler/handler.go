// Package handler adapts HTTP requests to the social service.
package handler

import (
	"github.com/gofiber/fiber/v2"
	"github.com/google/uuid"

	"github.com/poro/shared-go/httpx"
	"github.com/poro/shared-go/jwtauth"

	"github.com/poro/social/internal/dto"
	"github.com/poro/social/internal/service"
)

var (
	errUnauthorized = httpx.NewAPIError(fiber.StatusUnauthorized, "unauthorized", "unauthorized")
	errBadRequest   = httpx.NewAPIError(fiber.StatusBadRequest, "invalid_request", "invalid request")
	errBadID        = httpx.NewAPIError(fiber.StatusBadRequest, "invalid_id", "id must be a UUID")
)

// Social serves the /api/v1 social routes.
type Social struct {
	svc *service.Social
}

// New builds the HTTP handler.
func New(svc *service.Social) *Social {
	return &Social{svc: svc}
}

func (h *Social) LikeVideo(c *fiber.Ctx) error {
	return h.withUserAndID(c, "videoId", func(user, id uuid.UUID) error {
		out, err := h.svc.LikeVideo(c.UserContext(), user, id)
		return reply(c, fiber.StatusOK, out, err)
	})
}

func (h *Social) UnlikeVideo(c *fiber.Ctx) error {
	return h.withUserAndID(c, "videoId", func(user, id uuid.UUID) error {
		out, err := h.svc.UnlikeVideo(c.UserContext(), user, id)
		return reply(c, fiber.StatusOK, out, err)
	})
}

func (h *Social) VideoStats(c *fiber.Ctx) error {
	id, err := pathID(c, "videoId")
	if err != nil {
		return err
	}
	out, err := h.svc.VideoStats(c.UserContext(), id, viewer(c))
	return reply(c, fiber.StatusOK, out, err)
}

func (h *Social) ListVideoLikes(c *fiber.Ctx) error {
	id, err := pathID(c, "videoId")
	if err != nil {
		return err
	}
	out, err := h.svc.ListVideoLikes(c.UserContext(), id, c.Query("cursor"), c.QueryInt("limit"))
	return reply(c, fiber.StatusOK, out, err)
}

func (h *Social) ListUserLikes(c *fiber.Ctx) error {
	id, err := pathID(c, "userId")
	if err != nil {
		return err
	}
	out, err := h.svc.ListUserLikes(c.UserContext(), id, c.Query("cursor"), c.QueryInt("limit"))
	return reply(c, fiber.StatusOK, out, err)
}

func (h *Social) CreateComment(c *fiber.Ctx) error {
	return h.withUserAndID(c, "videoId", func(user, id uuid.UUID) error {
		var req dto.CreateCommentRequest
		if err := c.BodyParser(&req); err != nil {
			return errBadRequest
		}
		out, err := h.svc.CreateComment(c.UserContext(), user, id, req)
		return reply(c, fiber.StatusCreated, out, err)
	})
}

func (h *Social) ListComments(c *fiber.Ctx) error {
	id, err := pathID(c, "videoId")
	if err != nil {
		return err
	}
	out, err := h.svc.ListComments(c.UserContext(), id, viewer(c), c.Query("cursor"), c.QueryInt("limit"))
	return reply(c, fiber.StatusOK, out, err)
}

func (h *Social) ListReplies(c *fiber.Ctx) error {
	id, err := pathID(c, "commentId")
	if err != nil {
		return err
	}
	out, err := h.svc.ListReplies(c.UserContext(), id, viewer(c), c.Query("cursor"), c.QueryInt("limit"))
	return reply(c, fiber.StatusOK, out, err)
}

func (h *Social) EditComment(c *fiber.Ctx) error {
	return h.withUserAndID(c, "commentId", func(user, id uuid.UUID) error {
		var req dto.EditCommentRequest
		if err := c.BodyParser(&req); err != nil {
			return errBadRequest
		}
		out, err := h.svc.EditComment(c.UserContext(), user, id, req)
		return reply(c, fiber.StatusOK, out, err)
	})
}

func (h *Social) DeleteComment(c *fiber.Ctx) error {
	return h.withUserAndID(c, "commentId", func(user, id uuid.UUID) error {
		if err := h.svc.DeleteComment(c.UserContext(), user, id); err != nil {
			return err
		}
		return c.SendStatus(fiber.StatusNoContent)
	})
}

func (h *Social) LikeComment(c *fiber.Ctx) error {
	return h.withUserAndID(c, "commentId", func(user, id uuid.UUID) error {
		out, err := h.svc.LikeComment(c.UserContext(), user, id)
		return reply(c, fiber.StatusOK, out, err)
	})
}

func (h *Social) UnlikeComment(c *fiber.Ctx) error {
	return h.withUserAndID(c, "commentId", func(user, id uuid.UUID) error {
		out, err := h.svc.UnlikeComment(c.UserContext(), user, id)
		return reply(c, fiber.StatusOK, out, err)
	})
}

func (h *Social) Follow(c *fiber.Ctx) error {
	return h.withUserAndID(c, "userId", func(user, id uuid.UUID) error {
		out, err := h.svc.Follow(c.UserContext(), user, id)
		return reply(c, fiber.StatusOK, out, err)
	})
}

func (h *Social) Unfollow(c *fiber.Ctx) error {
	return h.withUserAndID(c, "userId", func(user, id uuid.UUID) error {
		out, err := h.svc.Unfollow(c.UserContext(), user, id)
		return reply(c, fiber.StatusOK, out, err)
	})
}

func (h *Social) UserStats(c *fiber.Ctx) error {
	id, err := pathID(c, "userId")
	if err != nil {
		return err
	}
	out, err := h.svc.UserStats(c.UserContext(), id, viewer(c))
	return reply(c, fiber.StatusOK, out, err)
}

func (h *Social) ListFollowers(c *fiber.Ctx) error {
	id, err := pathID(c, "userId")
	if err != nil {
		return err
	}
	out, err := h.svc.ListFollowers(c.UserContext(), id, c.Query("cursor"), c.QueryInt("limit"))
	return reply(c, fiber.StatusOK, out, err)
}

func (h *Social) ListFollowing(c *fiber.Ctx) error {
	id, err := pathID(c, "userId")
	if err != nil {
		return err
	}
	out, err := h.svc.ListFollowing(c.UserContext(), id, c.Query("cursor"), c.QueryInt("limit"))
	return reply(c, fiber.StatusOK, out, err)
}

func (h *Social) Share(c *fiber.Ctx) error {
	return h.withUserAndID(c, "videoId", func(user, id uuid.UUID) error {
		var req dto.ShareRequest
		if err := c.BodyParser(&req); err != nil {
			return errBadRequest
		}
		out, err := h.svc.Share(c.UserContext(), user, id, req)
		return reply(c, fiber.StatusCreated, out, err)
	})
}

// withUserAndID requires a verified caller and a UUID path parameter.
func (h *Social) withUserAndID(c *fiber.Ctx, param string, fn func(user, id uuid.UUID) error) error {
	claims, ok := jwtauth.ClaimsFrom(c)
	if !ok {
		return errUnauthorized
	}
	id, err := pathID(c, param)
	if err != nil {
		return err
	}
	return fn(claims.UserID, id)
}

func pathID(c *fiber.Ctx, param string) (uuid.UUID, error) {
	id, err := uuid.Parse(c.Params(param))
	if err != nil {
		return uuid.Nil, errBadID
	}
	return id, nil
}

func viewer(c *fiber.Ctx) *uuid.UUID {
	if claims, ok := jwtauth.ClaimsFrom(c); ok {
		return &claims.UserID
	}
	return nil
}

func reply(c *fiber.Ctx, status int, data any, err error) error {
	if err != nil {
		return err
	}
	return httpx.WriteData(c, status, data)
}
