// Package repository writes the search read model from events. Every function
// runs in the caller's transaction, next to the inbox claim.
package repository

import (
	"context"
	"errors"
	"fmt"
	"slices"
	"time"

	"github.com/google/uuid"
	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgconn"
)

// DBTX is satisfied by pgx.Tx and *pgxpool.Pool.
type DBTX interface {
	Exec(ctx context.Context, sql string, args ...any) (pgconn.CommandTag, error)
	Query(ctx context.Context, sql string, args ...any) (pgx.Rows, error)
	QueryRow(ctx context.Context, sql string, args ...any) pgx.Row
}

// Video is what poro.video.ready says about a video.
type Video struct {
	VideoID      uuid.UUID
	AuthorID     uuid.UUID
	Title        string
	Description  string
	Hashtags     []string
	ThumbnailKey string
	DurationMs   int
	PublishedAt  time.Time
	// MediaType is "video" or "photo" (ADR-0020); empty means video.
	MediaType string
}

const visible = `published_at IS NOT NULL AND deleted_at IS NULL AND moderation_status = 'approved'`

// visibleTags locks the post row and returns its hashtags when it is
// searchable (nil otherwise), with the counter of its media type: photo
// posts are counted apart from videos (ADR-0020).
func visibleTags(ctx context.Context, db DBTX, videoID uuid.UUID) ([]string, string, error) {
	var tags []string
	var shown bool
	var mediaType string
	err := db.QueryRow(ctx, `SELECT hashtags, `+visible+`, media_type FROM videos WHERE video_id = $1 FOR UPDATE`, videoID).
		Scan(&tags, &shown, &mediaType)
	if errors.Is(err, pgx.ErrNoRows) || (err == nil && !shown) {
		return nil, "", nil
	}
	if err != nil {
		return nil, "", fmt.Errorf("visible tags: %w", err)
	}
	if mediaType == MediaPhoto {
		return tags, "photos_count", nil
	}
	return tags, "videos_count", nil
}

// Media types of a post.
const (
	MediaVideo = "video"
	MediaPhoto = "photo"
)

// changeVideo applies change and keeps hashtags.videos_count and
// hashtags.photos_count equal to the number of searchable videos and photo
// posts carrying each tag.
func changeVideo(ctx context.Context, db DBTX, videoID uuid.UUID, at time.Time, change func() error) error {
	before, beforeCounter, err := visibleTags(ctx, db, videoID)
	if err != nil {
		return err
	}
	if err := change(); err != nil {
		return err
	}
	after, afterCounter, err := visibleTags(ctx, db, videoID)
	if err != nil {
		return err
	}
	// The counter is a column name chosen above, never client input.
	for _, tag := range distinct(after) {
		if slices.Contains(before, tag) && beforeCounter == afterCounter {
			continue
		}
		if _, err := db.Exec(ctx, `INSERT INTO hashtags (tag, `+afterCounter+`, last_used_at) VALUES ($1, 1, $2)
			ON CONFLICT (tag) DO UPDATE SET `+afterCounter+` = hashtags.`+afterCounter+` + 1,
				last_used_at = GREATEST(hashtags.last_used_at, EXCLUDED.last_used_at)`, tag, at); err != nil {
			return fmt.Errorf("count hashtag: %w", err)
		}
	}
	for _, tag := range distinct(before) {
		if slices.Contains(after, tag) && beforeCounter == afterCounter {
			continue
		}
		if _, err := db.Exec(ctx, `UPDATE hashtags SET `+beforeCounter+` = GREATEST(`+beforeCounter+` - 1, 0) WHERE tag = $1`, tag); err != nil {
			return fmt.Errorf("uncount hashtag: %w", err)
		}
	}
	return nil
}

func distinct(tags []string) []string {
	out := make([]string, 0, len(tags))
	for _, tag := range tags {
		if !slices.Contains(out, tag) {
			out = append(out, tag)
		}
	}
	return out
}

// UpsertReadyVideo records a ready video. A deleted or rejected video keeps
// that state whatever order the events arrive in.
func UpsertReadyVideo(ctx context.Context, db DBTX, v Video) error {
	tags := v.Hashtags
	if tags == nil {
		tags = []string{}
	}
	mediaType := v.MediaType
	if mediaType == "" {
		mediaType = MediaVideo
	}
	return changeVideo(ctx, db, v.VideoID, v.PublishedAt, func() error {
		_, err := db.Exec(ctx, `INSERT INTO videos (video_id, author_id, title, description, hashtags, thumbnail_key, duration_ms, published_at,
				media_type)
			VALUES ($1, $2, $3, $4, $5, $6, $7, $8, $9)
			ON CONFLICT (video_id) DO UPDATE SET author_id = EXCLUDED.author_id, title = EXCLUDED.title,
				description = EXCLUDED.description, hashtags = EXCLUDED.hashtags, thumbnail_key = EXCLUDED.thumbnail_key,
				duration_ms = EXCLUDED.duration_ms, published_at = EXCLUDED.published_at, media_type = EXCLUDED.media_type`,
			v.VideoID, v.AuthorID, v.Title, v.Description, tags, v.ThumbnailKey, v.DurationMs, v.PublishedAt, mediaType)
		if err != nil {
			return fmt.Errorf("upsert video: %w", err)
		}
		return nil
	})
}

// DeleteVideo hides a video for good, or leaves a tombstone for an unknown one.
func DeleteVideo(ctx context.Context, db DBTX, videoID, authorID uuid.UUID, at time.Time) error {
	return changeVideo(ctx, db, videoID, at, func() error {
		_, err := db.Exec(ctx, `INSERT INTO videos (video_id, author_id, deleted_at) VALUES ($1, $2, $3)
			ON CONFLICT (video_id) DO UPDATE SET deleted_at = COALESCE(videos.deleted_at, EXCLUDED.deleted_at)`,
			videoID, authorID, at)
		if err != nil {
			return fmt.Errorf("delete video: %w", err)
		}
		return nil
	})
}

// SetModeration hides (rejected) or shows again (approved) a video. An
// unknown video gets a stub, so a late poro.video.ready keeps it hidden.
func SetModeration(ctx context.Context, db DBTX, videoID, authorID uuid.UUID, status string, at time.Time) error {
	return changeVideo(ctx, db, videoID, at, func() error {
		_, err := db.Exec(ctx, `INSERT INTO videos (video_id, author_id, moderation_status) VALUES ($1, $2, $3)
			ON CONFLICT (video_id) DO UPDATE SET moderation_status = EXCLUDED.moderation_status`,
			videoID, authorID, status)
		if err != nil {
			return fmt.Errorf("set moderation: %w", err)
		}
		return nil
	})
}

// AddVideoStats moves the counters used for ranking, never below zero. A
// video not projected yet gets a stub when its owner is known.
func AddVideoStats(ctx context.Context, db DBTX, videoID uuid.UUID, ownerID *uuid.UUID, likes, comments, shares int64) error {
	if ownerID != nil {
		if _, err := db.Exec(ctx, `INSERT INTO videos (video_id, author_id) VALUES ($1, $2) ON CONFLICT (video_id) DO NOTHING`,
			videoID, *ownerID); err != nil {
			return fmt.Errorf("stub video: %w", err)
		}
	}
	_, err := db.Exec(ctx, `UPDATE videos SET likes_count = GREATEST(likes_count + $2, 0),
		comments_count = GREATEST(comments_count + $3, 0), shares_count = GREATEST(shares_count + $4, 0)
		WHERE video_id = $1`, videoID, likes, comments, shares)
	if err != nil {
		return fmt.Errorf("video stats: %w", err)
	}
	return nil
}

// notErased guards every insert into users: an erased account is never
// indexed again, whatever event arrives late.
const notErased = `NOT EXISTS (SELECT 1 FROM erased_users WHERE user_id = $1)`

// EnsureUser records an account; it becomes searchable once it has a username.
func EnsureUser(ctx context.Context, db DBTX, userID uuid.UUID) error {
	if _, err := db.Exec(ctx, `INSERT INTO users (user_id) SELECT $1 WHERE `+notErased+`
		ON CONFLICT (user_id) DO NOTHING`, userID); err != nil {
		return fmt.Errorf("ensure user: %w", err)
	}
	return nil
}

// Profile is a poro.user.profile.updated snapshot.
type Profile struct {
	UserID      uuid.UUID
	Username    *string
	DisplayName *string
	AvatarURL   *string
	IsCreator   bool
	UpdatedAt   time.Time
}

// UpsertProfile stores the snapshot unless a newer one is already stored.
func UpsertProfile(ctx context.Context, db DBTX, p Profile) error {
	_, err := db.Exec(ctx, `INSERT INTO users (user_id, username, display_name, avatar_url, is_creator, profile_at)
		SELECT $1, $2, $3, $4, $5, $6 WHERE `+notErased+`
		ON CONFLICT (user_id) DO UPDATE SET username = EXCLUDED.username, display_name = EXCLUDED.display_name,
			avatar_url = EXCLUDED.avatar_url, is_creator = EXCLUDED.is_creator, profile_at = EXCLUDED.profile_at
		WHERE users.profile_at IS NULL OR users.profile_at < EXCLUDED.profile_at`,
		p.UserID, p.Username, p.DisplayName, p.AvatarURL, p.IsCreator, p.UpdatedAt)
	if err != nil {
		return fmt.Errorf("upsert profile: %w", err)
	}
	return nil
}

// AddFollowers moves the follower count used to rank accounts, never below zero.
func AddFollowers(ctx context.Context, db DBTX, userID uuid.UUID, delta int64) error {
	_, err := db.Exec(ctx, `INSERT INTO users (user_id, followers_count) SELECT $1, GREATEST($2::bigint, 0) WHERE `+notErased+`
		ON CONFLICT (user_id) DO UPDATE SET followers_count = GREATEST(users.followers_count + $2, 0)`, userID, delta)
	if err != nil {
		return fmt.Errorf("followers: %w", err)
	}
	return nil
}

// EraseUser de-indexes an erased account (ADR-0018) and records its id so it
// is never indexed again. Its videos leave through poro.video.deleted, and the
// follower counts of the accounts it followed through poro.social.follow.deleted.
func EraseUser(ctx context.Context, db DBTX, userID uuid.UUID, at time.Time) error {
	if _, err := db.Exec(ctx, `INSERT INTO erased_users (user_id, erased_at) VALUES ($1, $2) ON CONFLICT DO NOTHING`, userID, at); err != nil {
		return fmt.Errorf("record erased user: %w", err)
	}
	if _, err := db.Exec(ctx, `DELETE FROM users WHERE user_id = $1`, userID); err != nil {
		return fmt.Errorf("delete user: %w", err)
	}
	if _, err := db.Exec(ctx, `DELETE FROM products WHERE owner_id = $1`, userID); err != nil {
		return fmt.Errorf("delete products: %w", err)
	}
	if _, err := db.Exec(ctx, `DELETE FROM shops WHERE owner_id = $1`, userID); err != nil {
		return fmt.Errorf("delete shops: %w", err)
	}
	return nil
}

// Shop is what poro.shop.shop.updated says about a shop.
type Shop struct {
	ShopID      uuid.UUID
	OwnerID     uuid.UUID
	Name        string
	Handle      string
	Description *string
	LogoURL     *string
	CountryCode string
	Currency    string
	Status      string
	UpdatedAt   time.Time
}

// UpsertShop keeps the newest snapshot of a shop. A shop of an erased account
// is never indexed again.
func UpsertShop(ctx context.Context, db DBTX, s Shop) error {
	_, err := db.Exec(ctx, `
		INSERT INTO shops (shop_id, owner_id, name, handle, description, logo_url, country_code, currency, status, updated_at)
		SELECT $1, $2, $3, $4, $5, $6, $7, $8, $9, $10
		WHERE NOT EXISTS (SELECT 1 FROM erased_users WHERE user_id = $2)
		ON CONFLICT (shop_id) DO UPDATE SET
			owner_id = EXCLUDED.owner_id, name = EXCLUDED.name, handle = EXCLUDED.handle,
			description = EXCLUDED.description, logo_url = EXCLUDED.logo_url,
			country_code = EXCLUDED.country_code, currency = EXCLUDED.currency,
			status = EXCLUDED.status, updated_at = EXCLUDED.updated_at
		WHERE shops.updated_at < EXCLUDED.updated_at`,
		s.ShopID, s.OwnerID, s.Name, s.Handle, s.Description, s.LogoURL, s.CountryCode, s.Currency, s.Status, s.UpdatedAt)
	if err != nil {
		return fmt.Errorf("upsert shop: %w", err)
	}
	return nil
}

// Product is what poro.shop.product.updated says about a product.
type Product struct {
	ProductID   uuid.UUID
	ShopID      uuid.UUID
	OwnerID     uuid.UUID
	Title       string
	Description *string
	Currency    string
	Status      string
	ImageURL    *string
	MinPrice    *int64
	InStock     bool
	UpdatedAt   time.Time
}

// ProductDeleted marks a product removed for good: later snapshots never revive it.
const ProductDeleted = "deleted"

// UpsertProduct keeps the newest snapshot of a product, unless the product
// was deleted or its owner erased.
func UpsertProduct(ctx context.Context, db DBTX, p Product) error {
	_, err := db.Exec(ctx, `
		INSERT INTO products (product_id, shop_id, owner_id, title, description, currency, status,
			image_url, min_price, in_stock, updated_at)
		SELECT $1, $2, $3, $4, $5, $6, $7, $8, $9, $10, $11
		WHERE NOT EXISTS (SELECT 1 FROM erased_users WHERE user_id = $3)
		ON CONFLICT (product_id) DO UPDATE SET
			shop_id = EXCLUDED.shop_id, owner_id = EXCLUDED.owner_id, title = EXCLUDED.title,
			description = EXCLUDED.description, currency = EXCLUDED.currency, status = EXCLUDED.status,
			image_url = EXCLUDED.image_url, min_price = EXCLUDED.min_price, in_stock = EXCLUDED.in_stock,
			updated_at = EXCLUDED.updated_at
		WHERE products.updated_at < EXCLUDED.updated_at AND products.status <> '`+ProductDeleted+`'`,
		p.ProductID, p.ShopID, p.OwnerID, p.Title, p.Description, p.Currency, p.Status,
		p.ImageURL, p.MinPrice, p.InStock, p.UpdatedAt)
	if err != nil {
		return fmt.Errorf("upsert product: %w", err)
	}
	return nil
}

// DeleteProduct keeps a tombstone so that a late snapshot does not index the
// product again; the tombstone holds no text.
func DeleteProduct(ctx context.Context, db DBTX, productID, shopID, ownerID uuid.UUID, at time.Time) error {
	_, err := db.Exec(ctx, `
		INSERT INTO products (product_id, shop_id, owner_id, title, currency, status, updated_at)
		VALUES ($1, $2, $3, '', 'XOF', '`+ProductDeleted+`', $4)
		ON CONFLICT (product_id) DO UPDATE SET
			title = '', description = NULL, image_url = NULL, min_price = NULL, in_stock = false,
			status = '`+ProductDeleted+`', updated_at = greatest(products.updated_at, EXCLUDED.updated_at)`,
		productID, shopID, ownerID, at)
	if err != nil {
		return fmt.Errorf("delete product: %w", err)
	}
	return nil
}
