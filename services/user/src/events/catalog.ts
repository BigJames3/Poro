// Event types. The JSON Schema of each lives in packages/contracts/events.
export const TYPE_AUTH_USER_CREATED = 'poro.auth.user.created';
export const TYPE_USER_CREATOR_ACTIVATED = 'poro.user.creator.activated';

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
