export const NOTIFICATION_TYPES = ['like', 'comment', 'reply', 'follow', 'video_ready'] as const;
export type NotificationType = (typeof NOTIFICATION_TYPES)[number];

export interface Preferences {
  pushEnabled: boolean;
  likes: boolean;
  comments: boolean;
  follows: boolean;
  videoReady: boolean;
}

export type TypePreference = Exclude<keyof Preferences, 'pushEnabled'>;

/** Without a row, an account gets every notification and every push. */
export const DEFAULT_PREFERENCES: Preferences = {
  pushEnabled: true,
  likes: true,
  comments: true,
  follows: true,
  videoReady: true,
};

/** The switch that governs each type; replies follow the comments switch. */
export const PREFERENCE_OF: Record<NotificationType, TypePreference> = {
  like: 'likes',
  comment: 'comments',
  reply: 'comments',
  follow: 'follows',
  video_ready: 'videoReady',
};
