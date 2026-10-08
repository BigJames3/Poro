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
] as const;

export interface AuthUserCreatedV1 {
  user_id: string;
  created_at: string;
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
  ready_at: string;
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
