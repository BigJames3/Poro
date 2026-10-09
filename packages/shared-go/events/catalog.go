package events

import "time"

// Event types. The JSON Schema of each lives in packages/contracts/events.
const (
	TypeAuthUserCreated      = "poro.auth.user.created"
	TypeUserCreatorActivated = "poro.user.creator.activated"
	TypeUserProfileUpdated   = "poro.user.profile.updated"
	TypeVideoUploaded        = "poro.video.uploaded"
	TypeVideoReady           = "poro.video.ready"
	TypeVideoFailed          = "poro.video.failed"
	TypeVideoDeleted         = "poro.video.deleted"

	TypeSocialLikeCreated    = "poro.social.like.created"
	TypeSocialLikeDeleted    = "poro.social.like.deleted"
	TypeSocialCommentCreated = "poro.social.comment.created"
	TypeSocialCommentUpdated = "poro.social.comment.updated"
	TypeSocialCommentDeleted = "poro.social.comment.deleted"
	TypeSocialFollowCreated  = "poro.social.follow.created"
	TypeSocialFollowDeleted  = "poro.social.follow.deleted"
	TypeSocialShareCreated   = "poro.social.share.created"

	TypeModerationContentRemoved  = "poro.moderation.content.removed"
	TypeModerationContentRestored = "poro.moderation.content.restored"

	TypeShopShopCreated        = "poro.shop.shop.created"
	TypeShopShopUpdated        = "poro.shop.shop.updated"
	TypeShopProductUpdated     = "poro.shop.product.updated"
	TypeShopProductDeleted     = "poro.shop.product.deleted"
	TypeShopStockReserved      = "poro.shop.stock.reserved"
	TypeShopStockRejected      = "poro.shop.stock.rejected"
	TypeOrderOrderCreated      = "poro.order.order.created"
	TypeOrderOrderPlaced       = "poro.order.order.placed"
	TypeOrderOrderCancelled    = "poro.order.order.cancelled"
	TypeOrderOrderShipped      = "poro.order.order.shipped"
	TypeOrderOrderCompleted    = "poro.order.order.completed"
	TypePaymentSucceeded       = "poro.payment.payment.succeeded"
	TypePaymentFailed          = "poro.payment.payment.failed"
	TypePaymentRefundSucceeded = "poro.payment.refund.succeeded"
)

// AuthUserCreatedV1 is published by auth when an account is created.
// It carries no phone number or email: consumers never need them.
type AuthUserCreatedV1 struct {
	UserID       string    `json:"user_id"`
	SignupMethod string    `json:"signup_method"` // phone, email
	CountryCode  *string   `json:"country_code"`
	Language     string    `json:"language"`
	CreatedAt    time.Time `json:"created_at"`
}

// UserCreatorActivatedV1 is published by the user service when a profile becomes a creator.
type UserCreatorActivatedV1 struct {
	UserID      string    `json:"user_id"`
	Username    string    `json:"username"`
	ActivatedAt time.Time `json:"activated_at"`
}

// UserProfileUpdatedV1 is published by the user service whenever a public
// profile field changes. It is a full snapshot: keep the latest UpdatedAt.
type UserProfileUpdatedV1 struct {
	UserID      string    `json:"user_id"`
	Username    *string   `json:"username"`
	DisplayName *string   `json:"display_name"`
	AvatarURL   *string   `json:"avatar_url"`
	IsCreator   bool      `json:"is_creator"`
	UpdatedAt   time.Time `json:"updated_at"`
}

// VideoUploadedV1 is published by the video API after a multipart upload completes.
// The media-worker transcodes the object at SourceKey.
type VideoUploadedV1 struct {
	VideoID     string    `json:"video_id"`
	UserID      string    `json:"user_id"`
	SourceKey   string    `json:"source_key"`
	ContentType string    `json:"content_type"`
	SizeBytes   int64     `json:"size_bytes"`
	UploadedAt  time.Time `json:"uploaded_at"`
}

// VideoRenditionV1 is one HLS ladder rung published when a video is ready.
type VideoRenditionV1 struct {
	Name        string `json:"name"`
	Bandwidth   int    `json:"bandwidth"`
	Width       int    `json:"width"`
	Height      int    `json:"height"`
	PlaylistKey string `json:"playlist_key"`
}

// VideoReadyV1 is published by the media-worker after HLS and a thumbnail exist.
// Title, Description, Hashtags and PublishedAt were added after the first
// release: consumers must accept events without them.
type VideoReadyV1 struct {
	VideoID      string             `json:"video_id"`
	UserID       string             `json:"user_id"`
	DurationMs   int                `json:"duration_ms"`
	Width        int                `json:"width"`
	Height       int                `json:"height"`
	HLSKey       string             `json:"hls_key"`
	ThumbnailKey string             `json:"thumbnail_key"`
	Renditions   []VideoRenditionV1 `json:"renditions"`
	ReadyAt      time.Time          `json:"ready_at"`
	Title        string             `json:"title"`
	Description  string             `json:"description"`
	Hashtags     []string           `json:"hashtags"`
	PublishedAt  time.Time          `json:"published_at"`
}

// Failure codes on VideoFailedV1. Never include FFmpeg stderr or file paths.
const (
	VideoFailTooLong   = "too_long"
	VideoFailInvalid   = "invalid_media"
	VideoFailTranscode = "transcode_failed"
)

// VideoFailedV1 is published when transcode cannot produce playable HLS.
type VideoFailedV1 struct {
	VideoID  string    `json:"video_id"`
	UserID   string    `json:"user_id"`
	Code     string    `json:"code"`
	FailedAt time.Time `json:"failed_at"`
}

// VideoDeletedV1 is published by the video API when the owner deletes a video.
// Projections must drop the video, whatever status they last saw.
type VideoDeletedV1 struct {
	VideoID   string    `json:"video_id"`
	UserID    string    `json:"user_id"`
	DeletedAt time.Time `json:"deleted_at"`
}

// Share channels accepted on SocialShareCreatedV1.
const (
	ShareChannelWhatsApp = "whatsapp"
	ShareChannelCopyLink = "copy_link"
	ShareChannelOther    = "other"
)

// SocialLikeCreatedV1 is published by social when a user starts liking a video.
type SocialLikeCreatedV1 struct {
	LikeID       string    `json:"like_id"`
	UserID       string    `json:"user_id"`
	VideoID      string    `json:"video_id"`
	VideoOwnerID string    `json:"video_owner_id"`
	CreatedAt    time.Time `json:"created_at"`
}

// SocialLikeDeletedV1 is published by social when a user removes a like.
type SocialLikeDeletedV1 struct {
	UserID       string    `json:"user_id"`
	VideoID      string    `json:"video_id"`
	VideoOwnerID string    `json:"video_owner_id"`
	DeletedAt    time.Time `json:"deleted_at"`
}

// SocialCommentCreatedV1 is published by social for a comment or a reply.
// ParentID and ParentAuthorID are nil for a top-level comment.
type SocialCommentCreatedV1 struct {
	CommentID      string  `json:"comment_id"`
	UserID         string  `json:"user_id"`
	VideoID        string  `json:"video_id"`
	VideoOwnerID   string  `json:"video_owner_id"`
	ParentID       *string `json:"parent_id"`
	ParentAuthorID *string `json:"parent_author_id"`
	Excerpt        string  `json:"excerpt"`
	// Text is the full comment. Events published before it existed lack it.
	Text      string    `json:"text,omitempty"`
	CreatedAt time.Time `json:"created_at"`
}

// SocialCommentUpdatedV1 is published by social when the author edits a
// comment. Text is the full text after the edit.
type SocialCommentUpdatedV1 struct {
	CommentID string    `json:"comment_id"`
	UserID    string    `json:"user_id"`
	VideoID   string    `json:"video_id"`
	Text      string    `json:"text"`
	UpdatedAt time.Time `json:"updated_at"`
}

// SocialCommentDeletedV1 is published by social when a comment is soft-deleted.
// UserID is the comment author, whoever deleted it.
type SocialCommentDeletedV1 struct {
	CommentID string    `json:"comment_id"`
	VideoID   string    `json:"video_id"`
	UserID    string    `json:"user_id"`
	DeletedAt time.Time `json:"deleted_at"`
}

// SocialFollowCreatedV1 is published by social when a user follows another.
type SocialFollowCreatedV1 struct {
	FollowerID  string    `json:"follower_id"`
	FollowingID string    `json:"following_id"`
	CreatedAt   time.Time `json:"created_at"`
}

// SocialFollowDeletedV1 is published by social when a user unfollows another.
type SocialFollowDeletedV1 struct {
	FollowerID  string    `json:"follower_id"`
	FollowingID string    `json:"following_id"`
	DeletedAt   time.Time `json:"deleted_at"`
}

// SocialShareCreatedV1 is published by social each time a video is shared.
type SocialShareCreatedV1 struct {
	ShareID      string    `json:"share_id"`
	UserID       string    `json:"user_id"`
	VideoID      string    `json:"video_id"`
	VideoOwnerID string    `json:"video_owner_id"`
	Channel      string    `json:"channel"`
	CreatedAt    time.Time `json:"created_at"`
}

// Moderation targets, reasons and deciders.
const (
	ModerationTargetVideo   = "video"
	ModerationTargetComment = "comment"

	ModerationReasonSpam       = "spam"
	ModerationReasonNudity     = "nudity"
	ModerationReasonViolence   = "violence"
	ModerationReasonHarassment = "harassment"
	ModerationReasonHate       = "hate"
	ModerationReasonFraud      = "fraud"
	ModerationReasonOther      = "other"

	ModerationDecidedByAuto      = "auto"
	ModerationDecidedByModerator = "moderator"
)

// ModerationContentRemovedV1 is published by moderation when a video or a
// comment is removed. The subject is TargetID.
type ModerationContentRemovedV1 struct {
	CaseID     string    `json:"case_id"`
	TargetType string    `json:"target_type"` // video, comment
	TargetID   string    `json:"target_id"`
	OwnerID    string    `json:"owner_id"`
	Reason     string    `json:"reason"`
	DecidedBy  string    `json:"decided_by"` // auto, moderator
	RemovedAt  time.Time `json:"removed_at"`
}

// ModerationContentRestoredV1 is published by moderation when a moderator
// restores a removed video. The subject is TargetID.
type ModerationContentRestoredV1 struct {
	CaseID     string    `json:"case_id"`
	TargetType string    `json:"target_type"` // video
	TargetID   string    `json:"target_id"`
	OwnerID    string    `json:"owner_id"`
	RestoredAt time.Time `json:"restored_at"`
}

// Marketplace currencies, countries, payment methods and statuses. Amounts are
// int64 in minor units: XOF and XAF have none, NGN is in kobo.
const (
	CurrencyXOF = "XOF"
	CurrencyXAF = "XAF"
	CurrencyNGN = "NGN"

	PaymentMethodCashOnDelivery = "cash_on_delivery"
	PaymentMethodWave           = "wave"
	PaymentMethodSimulated      = "simulated"

	ShopStatusActive    = "active"
	ShopStatusSuspended = "suspended"
	ShopStatusClosed    = "closed"

	ProductStatusDraft    = "draft"
	ProductStatusActive   = "active"
	ProductStatusArchived = "archived"

	StockRejectedOutOfStock         = "out_of_stock"
	StockRejectedProductUnavailable = "product_unavailable"
	StockRejectedShopUnavailable    = "shop_unavailable"
	StockRejectedCurrencyMismatch   = "currency_mismatch"

	OrderCancelledStockRejected   = "stock_rejected"
	OrderCancelledPaymentTimeout  = "payment_timeout"
	OrderCancelledBuyerCancelled  = "buyer_cancelled"
	OrderCancelledSellerCancelled = "seller_cancelled"

	PaymentFailedDeclined      = "declined"
	PaymentFailedExpired       = "expired"
	PaymentFailedCancelled     = "cancelled"
	PaymentFailedProviderError = "provider_error"
)

// ShopShopCreatedV1 is published by shop when an account opens its shop. A
// shop always belongs to one auth account (OwnerID), which holds at most one
// shop. Auth grants BUSINESS to OwnerID. The subject is ShopID.
type ShopShopCreatedV1 struct {
	ShopID      string    `json:"shop_id"`
	OwnerID     string    `json:"owner_id"`
	Name        string    `json:"name"`
	Handle      string    `json:"handle"`
	CountryCode string    `json:"country_code"` // CI, SN, CM, NG
	Currency    string    `json:"currency"`
	CreatedAt   time.Time `json:"created_at"`
}

// ShopShopUpdatedV1 is a full snapshot of a shop: keep the latest UpdatedAt.
type ShopShopUpdatedV1 struct {
	ShopID      string    `json:"shop_id"`
	OwnerID     string    `json:"owner_id"`
	Name        string    `json:"name"`
	Handle      string    `json:"handle"`
	Description *string   `json:"description"`
	LogoURL     *string   `json:"logo_url"`
	CountryCode string    `json:"country_code"`
	Currency    string    `json:"currency"`
	Status      string    `json:"status"`
	UpdatedAt   time.Time `json:"updated_at"`
}

// ShopVariantV1 is one purchasable variant of a product. InStock is a hint:
// the stock reservation decides.
type ShopVariantV1 struct {
	VariantID string `json:"variant_id"`
	Title     string `json:"title"`
	Price     int64  `json:"price"`
	InStock   bool   `json:"in_stock"`
}

// ShopProductUpdatedV1 is a full snapshot of a product without its stock:
// keep the latest UpdatedAt. The subject is ProductID.
type ShopProductUpdatedV1 struct {
	ProductID   string          `json:"product_id"`
	ShopID      string          `json:"shop_id"`
	OwnerID     string          `json:"owner_id"`
	Title       string          `json:"title"`
	Description *string         `json:"description"`
	Currency    string          `json:"currency"`
	Status      string          `json:"status"`
	ImageURLs   []string        `json:"image_urls"`
	Variants    []ShopVariantV1 `json:"variants"`
	UpdatedAt   time.Time       `json:"updated_at"`
}

// ShopProductDeletedV1 is published by shop when a product is deleted for good.
type ShopProductDeletedV1 struct {
	ProductID string    `json:"product_id"`
	ShopID    string    `json:"shop_id"`
	OwnerID   string    `json:"owner_id"`
	DeletedAt time.Time `json:"deleted_at"`
}

// OrderLineV1 is one line of an order at the price the buyer saw.
type OrderLineV1 struct {
	ProductID string `json:"product_id"`
	VariantID string `json:"variant_id"`
	Quantity  int    `json:"quantity"`
	UnitPrice int64  `json:"unit_price"`
}

// OrderOrderCreatedV1 is published by order at checkout, one order per shop.
// Shop answers with stock.reserved or stock.rejected. The subject is OrderID.
type OrderOrderCreatedV1 struct {
	OrderID       string        `json:"order_id"`
	BuyerID       string        `json:"buyer_id"`
	ShopID        string        `json:"shop_id"`
	SellerID      string        `json:"seller_id"`
	Currency      string        `json:"currency"`
	PaymentMethod string        `json:"payment_method"`
	Items         []OrderLineV1 `json:"items"`
	CreatedAt     time.Time     `json:"created_at"`
}

// ShopReservedLineV1 is a reserved line with the authoritative price and title.
type ShopReservedLineV1 struct {
	ProductID string `json:"product_id"`
	VariantID string `json:"variant_id"`
	Quantity  int    `json:"quantity"`
	UnitPrice int64  `json:"unit_price"`
	Title     string `json:"title"`
}

// ShopStockReservedV1 is published by shop once every line of an order is held.
type ShopStockReservedV1 struct {
	OrderID    string               `json:"order_id"`
	ShopID     string               `json:"shop_id"`
	Currency   string               `json:"currency"`
	Items      []ShopReservedLineV1 `json:"items"`
	Subtotal   int64                `json:"subtotal"`
	ReservedAt time.Time            `json:"reserved_at"`
}

// ShopStockRejectedV1 is published by shop when an order cannot be reserved.
// VariantIDs lists the offending variants, empty when the whole shop is at fault.
type ShopStockRejectedV1 struct {
	OrderID    string    `json:"order_id"`
	ShopID     string    `json:"shop_id"`
	Reason     string    `json:"reason"`
	VariantIDs []string  `json:"variant_ids"`
	RejectedAt time.Time `json:"rejected_at"`
}

// OrderOrderPlacedV1 is published by order after the reservation. Payment
// charges Total. ExpiresAt is nil for cash on delivery.
type OrderOrderPlacedV1 struct {
	OrderID       string     `json:"order_id"`
	BuyerID       string     `json:"buyer_id"`
	ShopID        string     `json:"shop_id"`
	SellerID      string     `json:"seller_id"`
	Currency      string     `json:"currency"`
	Total         int64      `json:"total"`
	PaymentMethod string     `json:"payment_method"`
	PlacedAt      time.Time  `json:"placed_at"`
	ExpiresAt     *time.Time `json:"expires_at"`
}

// OrderOrderCancelledV1 ends an order before completion. Shop releases the
// stock it holds; payment refunds when Paid is true.
type OrderOrderCancelledV1 struct {
	OrderID     string    `json:"order_id"`
	BuyerID     string    `json:"buyer_id"`
	ShopID      string    `json:"shop_id"`
	SellerID    string    `json:"seller_id"`
	Reason      string    `json:"reason"`
	Paid        bool      `json:"paid"`
	CancelledAt time.Time `json:"cancelled_at"`
}

// OrderOrderShippedV1 is published when the seller marks the order shipped.
type OrderOrderShippedV1 struct {
	OrderID   string    `json:"order_id"`
	BuyerID   string    `json:"buyer_id"`
	ShopID    string    `json:"shop_id"`
	SellerID  string    `json:"seller_id"`
	Tracking  *string   `json:"tracking"`
	ShippedAt time.Time `json:"shipped_at"`
}

// OrderOrderCompletedV1 is published when the buyer confirms receipt or the
// confirmation delay ends. Total becomes payable to the seller.
type OrderOrderCompletedV1 struct {
	OrderID       string    `json:"order_id"`
	BuyerID       string    `json:"buyer_id"`
	ShopID        string    `json:"shop_id"`
	SellerID      string    `json:"seller_id"`
	Currency      string    `json:"currency"`
	Total         int64     `json:"total"`
	PaymentMethod string    `json:"payment_method"`
	CompletedAt   time.Time `json:"completed_at"`
}

// PaymentSucceededV1 is published once the provider confirmed a payment by a
// verified webhook or a server-side status check. The subject is PaymentID.
type PaymentSucceededV1 struct {
	PaymentID   string    `json:"payment_id"`
	OrderID     string    `json:"order_id"`
	BuyerID     string    `json:"buyer_id"`
	Amount      int64     `json:"amount"`
	Currency    string    `json:"currency"`
	Provider    string    `json:"provider"` // wave, simulated
	SucceededAt time.Time `json:"succeeded_at"`
}

// PaymentFailedV1 is published when a payment attempt fails. The buyer may
// retry until the order expires.
type PaymentFailedV1 struct {
	PaymentID string    `json:"payment_id"`
	OrderID   string    `json:"order_id"`
	BuyerID   string    `json:"buyer_id"`
	Provider  string    `json:"provider"`
	Reason    string    `json:"reason"`
	FailedAt  time.Time `json:"failed_at"`
}

// PaymentRefundSucceededV1 is published once the provider confirmed a refund.
// The subject is RefundID.
type PaymentRefundSucceededV1 struct {
	RefundID   string    `json:"refund_id"`
	PaymentID  string    `json:"payment_id"`
	OrderID    string    `json:"order_id"`
	BuyerID    string    `json:"buyer_id"`
	Amount     int64     `json:"amount"`
	Currency   string    `json:"currency"`
	Provider   string    `json:"provider"`
	RefundedAt time.Time `json:"refunded_at"`
}
