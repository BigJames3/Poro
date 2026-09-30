import { isUuid, uuidv7 } from './uuid';

describe('uuidv7', () => {
  it('encodes the timestamp, version and variant', () => {
    const at = Date.UTC(2026, 8, 30, 12, 0, 0, 123);
    const id = uuidv7(at);
    expect(isUuid(id)).toBe(true);
    expect(id[14]).toBe('7');
    expect(['8', '9', 'a', 'b']).toContain(id[19]);
    expect(parseInt(id.replace(/-/g, '').slice(0, 12), 16)).toBe(at);
  });

  it('sorts by creation time', () => {
    const ids = [uuidv7(1_000), uuidv7(2_000), uuidv7(3_000)];
    expect([...ids].sort()).toEqual(ids);
    expect(new Set(Array.from({ length: 1000 }, () => uuidv7())).size).toBe(1000);
  });

  it('recognizes UUIDs only', () => {
    expect(isUuid('0192f3a1-7b2c-7d3e-8f40-123456789abc')).toBe(true);
    expect(isUuid('not-a-uuid')).toBe(false);
    expect(isUuid(42)).toBe(false);
  });
});
