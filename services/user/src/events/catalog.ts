// Event types. The JSON Schema of each lives in packages/contracts/events.
export const TYPE_AUTH_USER_CREATED = 'poro.auth.user.created';
export const TYPE_AUTH_USER_DELETED = 'poro.auth.user.deleted';
export const TYPE_AUTH_USER_DELETION_SCHEDULED = 'poro.auth.user.deletion_scheduled';
export const TYPE_AUTH_USER_DELETION_CANCELLED = 'poro.auth.user.deletion_cancelled';
export const TYPE_USER_CREATOR_ACTIVATED = 'poro.user.creator.activated';
export const TYPE_USER_PROFILE_UPDATED = 'poro.user.profile.updated';

/** Published by auth when an account is created. Carries no contact data. */
export interface AuthUserCreatedV1 {
  user_id: string;
  signup_method: 'phone' | 'email';
  country_code: string | null;
  language: string;
  created_at: string;
}

/** Published by auth when an account is erased (ADR-0018). Carries no contact data. */
/** Published by auth when an owner asks to delete their account: hide it until erase_at. */
export interface AuthUserDeletionScheduledV1 {
  user_id: string;
  erase_at: string;
  changed_at: string;
}

/** Published by auth when a scheduled deletion is cancelled: show the account again. */
export interface AuthUserDeletionCancelledV1 {
  user_id: string;
  changed_at: string;
}

export interface AuthUserDeletedV1 {
  user_id: string;
  deleted_at: string;
}

/** Published by this service when a profile becomes a creator. */
export interface UserCreatorActivatedV1 {
  user_id: string;
  username: string;
  activated_at: string;
}

/**
 * Published by this service, in the transaction of the change, whenever a
 * public field of a profile changes. It is a full snapshot: consumers replace
 * their copy and ignore a snapshot older than the one they hold.
 */
export interface UserProfileUpdatedV1 {
  user_id: string;
  username: string | null;
  display_name: string | null;
  avatar_url: string | null;
  is_creator: boolean;
  updated_at: string;
}
