export const NOTIFICATION_TYPES = [
  'like',
  'comment',
  'reply',
  'follow',
  'video_ready',
  'order_new',
  'order_confirmed',
  'order_shipped',
  'order_cancelled',
  'order_completed',
] as const;
export type NotificationType = (typeof NOTIFICATION_TYPES)[number];

export interface Preferences {
  pushEnabled: boolean;
  likes: boolean;
  comments: boolean;
  follows: boolean;
  videoReady: boolean;
  orders: boolean;
}

export type TypePreference = Exclude<keyof Preferences, 'pushEnabled'>;

/** Without a row, an account gets every notification and every push. */
export const DEFAULT_PREFERENCES: Preferences = {
  pushEnabled: true,
  likes: true,
  comments: true,
  follows: true,
  videoReady: true,
  orders: true,
};

/** The switch that governs each type; replies follow the comments switch. */
export const PREFERENCE_OF: Record<NotificationType, TypePreference> = {
  like: 'likes',
  comment: 'comments',
  reply: 'comments',
  follow: 'follows',
  video_ready: 'videoReady',
  order_new: 'orders',
  order_confirmed: 'orders',
  order_shipped: 'orders',
  order_cancelled: 'orders',
  order_completed: 'orders',
};

/**
 * Switches that only mute the push: the in-app notification of a purchase or
 * a sale is always written, so neither side misses an order.
 */
export const PUSH_ONLY_PREFERENCES: ReadonlySet<TypePreference> = new Set(['orders']);
