// Event types. The JSON Schema of each lives in packages/contracts/events.
export const TYPE_AUTH_USER_CREATED = 'poro.auth.user.created';
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
