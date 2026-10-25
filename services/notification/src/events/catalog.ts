// Event types this service consumes. Their JSON Schemas live in packages/contracts/events.
// It publishes no event: notifications are a terminal read model.
export const TYPE_AUTH_USER_CREATED = 'poro.auth.user.created';
export const TYPE_USER_PROFILE_UPDATED = 'poro.user.profile.updated';
export const TYPE_VIDEO_READY = 'poro.video.ready';
export const TYPE_VIDEO_DELETED = 'poro.video.deleted';
export const TYPE_SOCIAL_LIKE_CREATED = 'poro.social.like.created';
export const TYPE_SOCIAL_COMMENT_CREATED = 'poro.social.comment.created';
export const TYPE_SOCIAL_COMMENT_DELETED = 'poro.social.comment.deleted';
export const TYPE_SOCIAL_FOLLOW_CREATED = 'poro.social.follow.created';
export const TYPE_MODERATION_CONTENT_REMOVED = 'poro.moderation.content.removed';
export const TYPE_SHOP_UPDATED = 'poro.shop.shop.updated';
export const TYPE_ORDER_PLACED = 'poro.order.order.placed';
export const TYPE_ORDER_SHIPPED = 'poro.order.order.shipped';
export const TYPE_ORDER_CANCELLED = 'poro.order.order.cancelled';
export const TYPE_ORDER_COMPLETED = 'poro.order.order.completed';
// Consumed by its own group (account-erasure.consumer.ts), not by the dispatcher.
export const TYPE_AUTH_USER_DELETED = 'poro.auth.user.deleted';

export const CONSUMED_TYPES = [
  TYPE_AUTH_USER_CREATED,
  TYPE_USER_PROFILE_UPDATED,
  TYPE_VIDEO_READY,
  TYPE_VIDEO_DELETED,
  TYPE_SOCIAL_LIKE_CREATED,
  TYPE_SOCIAL_COMMENT_CREATED,
  TYPE_SOCIAL_COMMENT_DELETED,
  TYPE_SOCIAL_FOLLOW_CREATED,
  TYPE_MODERATION_CONTENT_REMOVED,
  TYPE_SHOP_UPDATED,
  TYPE_ORDER_PLACED,
  TYPE_ORDER_SHIPPED,
  TYPE_ORDER_CANCELLED,
  TYPE_ORDER_COMPLETED,
] as const;

export interface AuthUserCreatedV1 {
  user_id: string;
  created_at: string;
}

/** Published by auth when an account is erased (ADR-0018). */
export interface AuthUserDeletedV1 {
  user_id: string;
  deleted_at: string;
}

/** A full snapshot: the newest updated_at wins. */
export interface UserProfileUpdatedV1 {
  user_id: string;
  username: string | null;
  display_name: string | null;
  avatar_url: string | null;
  is_creator: boolean;
  updated_at: string;
}

export interface VideoReadyV1 {
  video_id: string;
  user_id: string;
  title?: string;
  description?: string;
  ready_at: string;
  /** "video" or "photo" (ADR-0020); absent on events published before photo posts. */
  media_type?: string;
}

export interface VideoDeletedV1 {
  video_id: string;
  user_id: string;
  deleted_at: string;
}

export interface SocialLikeCreatedV1 {
  like_id: string;
  user_id: string;
  video_id: string;
  video_owner_id: string;
  created_at: string;
}

export interface SocialCommentCreatedV1 {
  comment_id: string;
  user_id: string;
  video_id: string;
  video_owner_id: string;
  parent_id: string | null;
  parent_author_id: string | null;
  excerpt: string;
  created_at: string;
}

export interface SocialCommentDeletedV1 {
  comment_id: string;
  video_id: string;
  user_id: string;
  deleted_at: string;
}

export interface SocialFollowCreatedV1 {
  follower_id: string;
  following_id: string;
  created_at: string;
}

/** Removed comments also arrive as poro.social.comment.deleted from social. */
export interface ModerationContentRemovedV1 {
  case_id: string;
  target_type: 'video' | 'comment';
  target_id: string;
  owner_id: string;
  reason: string;
  decided_by: 'auto' | 'moderator';
  removed_at: string;
}

export type Currency = 'XOF' | 'XAF' | 'NGN';
export type PaymentMethod = 'cash_on_delivery' | 'wave' | 'simulated';
export type CancelReason =
  | 'stock_rejected'
  | 'payment_timeout'
  | 'buyer_cancelled'
  | 'seller_cancelled'
  | 'account_deleted'
  | 'shop_suspended';

/** A full snapshot of the public fields of a shop: the newest updated_at wins. */
export interface ShopUpdatedV1 {
  shop_id: string;
  owner_id: string;
  name: string;
  handle: string;
  description: string | null;
  logo_url: string | null;
  country_code: string;
  currency: Currency;
  status: 'active' | 'closed' | 'suspended';
  updated_at: string;
}

interface OrderParties {
  order_id: string;
  buyer_id: string;
  shop_id: string;
  seller_id: string;
}

/** Stock held: the order is real. Amounts are minor units (NGN in kobo). */
export interface OrderPlacedV1 extends OrderParties {
  currency: Currency;
  total: number;
  payment_method: PaymentMethod;
  placed_at: string;
  expires_at: string | null;
}

export interface OrderShippedV1 extends OrderParties {
  tracking: string | null;
  shipped_at: string;
}

export interface OrderCancelledV1 extends OrderParties {
  reason: CancelReason;
  paid: boolean;
  cancelled_at: string;
}

export interface OrderCompletedV1 extends OrderParties {
  currency: Currency;
  total: number;
  payment_method: PaymentMethod;
  completed_at: string;
}
