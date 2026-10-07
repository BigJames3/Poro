import { ApiError } from '../common/api-error';
import { isUuid } from '../common/uuid';

/** Position after the last item of a page: its last_activity_at and id. */
export interface Cursor {
  at: Date;
  id: string;
}

export function encodeCursor(cursor: Cursor): string {
  return Buffer.from(`${cursor.at.getTime()}|${cursor.id}`).toString('base64url');
}

export function decodeCursor(raw: string): Cursor {
  const [ms, id, ...rest] = Buffer.from(raw, 'base64url').toString('utf8').split('|');
  const at = new Date(Number(ms));
  if (rest.length > 0 || !/^\d{1,15}$/.test(ms) || Number.isNaN(at.getTime()) || !isUuid(id)) {
    throw new ApiError(400, 'invalid_cursor', 'invalid cursor');
  }
  return { at, id: id.toLowerCase() };
}
