import { ApiError } from './api-error';
import { isUuid } from './uuid';

/** Position after the last item of a page sorted by (created_at DESC, id DESC). */
export interface Cursor {
  createdAt: Date;
  id: string;
}

export const DEFAULT_PAGE_SIZE = 20;
export const MAX_PAGE_SIZE = 50;

export function encodeCursor(cursor: Cursor): string {
  return Buffer.from(`${cursor.createdAt.toISOString()}|${cursor.id}`, 'utf8').toString(
    'base64url',
  );
}

/** Undefined for page 1; a malformed cursor is a client error, never a crash. */
export function decodeCursor(raw: string | undefined): Cursor | undefined {
  if (raw === undefined || raw === '') {
    return undefined;
  }
  const parts = Buffer.from(raw, 'base64url').toString('utf8').split('|');
  const createdAt = new Date(parts[0]);
  if (parts.length !== 2 || !isUuid(parts[1]) || Number.isNaN(createdAt.getTime())) {
    throw new ApiError(400, 'invalid_cursor', 'invalid cursor');
  }
  return { createdAt, id: parts[1].toLowerCase() };
}

export function pageSize(raw: number | undefined): number {
  return raw === undefined ? DEFAULT_PAGE_SIZE : Math.min(Math.max(raw, 1), MAX_PAGE_SIZE);
}
