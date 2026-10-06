export const USERNAME_MIN = 3;
export const USERNAME_MAX = 30;

const ALLOWED = /^[a-z0-9._]+$/;

// Handles that would let someone impersonate PORO or a system route.
const RESERVED = new Set([
  'about',
  'account',
  'admin',
  'administrator',
  'api',
  'app',
  'auth',
  'billing',
  'creator',
  'creators',
  'explore',
  'help',
  'home',
  'login',
  'logout',
  'me',
  'moderator',
  'null',
  'official',
  'payment',
  'payments',
  'poro',
  'poro_africa',
  'poro_official',
  'poroafrica',
  'porosupport',
  'privacy',
  'root',
  'security',
  'settings',
  'shop',
  'signup',
  'staff',
  'support',
  'system',
  'terms',
  'undefined',
  'user',
  'users',
  'wallet',
]);

const withoutSeparators = (name: string): string => name.replace(/[._]/g, '');
const RESERVED_COMPACT = new Set([...RESERVED].map(withoutSeparators));

export type UsernameProblem = 'invalid' | 'reserved';

export function normalizeUsername(raw: string): string {
  return raw.trim().replace(/^@/, '').toLowerCase();
}

/**
 * Returns why a normalized username cannot be used, or undefined when it can.
 * Rules: 3 to 30 of a-z, 0-9, "." and "_"; no leading, trailing or doubled dot;
 * not only digits, so a handle never looks like a phone number.
 */
export function usernameProblem(username: string): UsernameProblem | undefined {
  if (
    username.length < USERNAME_MIN ||
    username.length > USERNAME_MAX ||
    !ALLOWED.test(username) ||
    username.startsWith('.') ||
    username.endsWith('.') ||
    username.includes('..') ||
    /^\d+$/.test(username)
  ) {
    return 'invalid';
  }
  if (RESERVED_COMPACT.has(withoutSeparators(username))) {
    return 'reserved';
  }
  return undefined;
}
