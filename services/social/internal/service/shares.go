package service

import (
	"context"

	"github.com/google/uuid"
	"github.com/jackc/pgx/v5"

	"github.com/poro/shared-go/events"

	"github.com/poro/social/internal/dto"
	"github.com/poro/social/internal/repository"
)

var shareChannels = map[string]bool{
	events.ShareChannelWhatsApp: true,
	events.ShareChannelCopyLink: true,
	events.ShareChannelOther:    true,
}

// Share records that the user shared a ready video. Every share counts.
func (s *Social) Share(ctx context.Context, user, videoID uuid.UUID, req dto.ShareRequest) (*dto.ShareResult, error) {
	if !shareChannels[req.Channel] {
		return nil, errChannelInvalid
	}
	share := repository.Share{ID: newID(), UserID: user, VideoID: videoID, Channel: req.Channel}
	err := s.inTx(ctx, func(tx pgx.Tx) error {
		owner, err := readyVideo(ctx, tx, videoID)
		if err != nil {
			return err
		}
		if err := repository.EnsureUser(ctx, tx, user); err != nil {
			return err
		}
		share.CreatedAt = s.clock()
		if err := repository.InsertShare(ctx, tx, share); err != nil {
			return err
		}
		if err := repository.AddVideoCounters(ctx, tx, videoID, 0, 0, 1); err != nil {
			return err
		}
		return emit(ctx, tx, events.TypeSocialShareCreated, videoID.String(), events.SocialShareCreatedV1{
			ShareID: share.ID.String(), UserID: user.String(), VideoID: videoID.String(),
			VideoOwnerID: owner.String(), Channel: share.Channel, CreatedAt: share.CreatedAt,
		}, share.CreatedAt)
	})
	if err != nil {
		return nil, err
	}
	counters, err := repository.GetVideoCounters(ctx, s.pool, videoID)
	if err != nil {
		return nil, errInternal.WithCause(err)
	}
	return &dto.ShareResult{
		ShareID: share.ID.String(), VideoID: videoID.String(), Channel: share.Channel, SharesCount: counters.Shares,
	}, nil
}
