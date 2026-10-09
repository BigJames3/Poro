export const HANDLE_MIN = 3;
export const HANDLE_MAX = 30;

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
  'shops',
  'store',
  'boutique',
  'market',
  'marketplace',
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

export type HandleProblem = 'invalid' | 'reserved';

export function normalizeHandle(raw: string): string {
  return raw.trim().replace(/^@/, '').toLowerCase();
}

/**
 * Returns why a normalized shop handle cannot be used, or undefined when it can.
 * Rules: 3 to 30 of a-z, 0-9, "." and "_"; no leading, trailing or doubled dot;
 * not only digits, so a handle never looks like a phone number.
 * Same rules as usernames, in a separate namespace.
 */
export function handleProblem(handle: string): HandleProblem | undefined {
  if (
    handle.length < HANDLE_MIN ||
    handle.length > HANDLE_MAX ||
    !ALLOWED.test(handle) ||
    handle.startsWith('.') ||
    handle.endsWith('.') ||
    handle.includes('..') ||
    /^\d+$/.test(handle)
  ) {
    return 'invalid';
  }
  if (RESERVED_COMPACT.has(withoutSeparators(handle))) {
    return 'reserved';
  }
  return undefined;
}
