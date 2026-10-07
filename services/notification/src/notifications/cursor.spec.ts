import { ApiError } from '../common/api-error';
import { uuidv7 } from '../common/uuid';
import { decodeCursor, encodeCursor } from './cursor';

describe('cursor', () => {
  it('round-trips a position', () => {
    const cursor = { at: new Date('2026-10-07T10:00:00.123Z'), id: uuidv7() };
    expect(decodeCursor(encodeCursor(cursor))).toEqual(cursor);
  });

  it.each([
    'bad',
    Buffer.from(`123|not-a-uuid`).toString('base64url'),
    Buffer.from(`abc|${uuidv7()}`).toString('base64url'),
    Buffer.from(`123|${uuidv7()}|extra`).toString('base64url'),
  ])('rejects %s', (raw) => {
    expect(() => decodeCursor(raw)).toThrow(ApiError);
  });
});
