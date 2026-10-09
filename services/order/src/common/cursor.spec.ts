import { ApiError } from './api-error';
import { DEFAULT_PAGE_SIZE, MAX_PAGE_SIZE, decodeCursor, encodeCursor, pageSize } from './cursor';
import { uuidv7 } from './uuid';

describe('cursor', () => {
  it('round-trips a position', () => {
    const cursor = { createdAt: new Date('2026-10-09T10:00:00.123Z'), id: uuidv7() };
    expect(decodeCursor(encodeCursor(cursor))).toEqual(cursor);
    expect(decodeCursor(undefined)).toBeUndefined();
    expect(decodeCursor('')).toBeUndefined();
  });

  it.each([
    Buffer.from('nope').toString('base64url'),
    Buffer.from(`not-a-date|${uuidv7()}`).toString('base64url'),
    Buffer.from('2026-10-09T10:00:00Z|x').toString('base64url'),
    Buffer.from(`2026-10-09T10:00:00Z|${uuidv7()}|extra`).toString('base64url'),
  ])('rejects %s', (raw) => {
    expect(() => decodeCursor(raw)).toThrow(ApiError);
  });

  it('bounds the page size', () => {
    expect(pageSize(undefined)).toBe(DEFAULT_PAGE_SIZE);
    expect(pageSize(0)).toBe(1);
    expect(pageSize(500)).toBe(MAX_PAGE_SIZE);
  });
});
