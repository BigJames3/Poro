import { handleProblem, normalizeHandle } from './handle';

describe('shop handles', () => {
  it('normalizes case, spaces and a leading @', () => {
    expect(normalizeHandle('  @Pagnes.Awa ')).toBe('pagnes.awa');
  });

  it.each(['pagnes.awa', 'bazin_dakar', 'awa2026'])('accepts %s', (handle) => {
    expect(handleProblem(handle)).toBeUndefined();
  });

  it.each(['ab', 'a'.repeat(31), '.awa', 'awa.', 'a..b', '0700000000', 'awa-shop', 'awa shop'])(
    'rejects %s as invalid',
    (handle) => {
      expect(handleProblem(handle)).toBe('invalid');
    },
  );

  it.each(['shop', 'boutique', 'market_place', 'p.o.r.o', 'me'])('reserves %s', (handle) => {
    expect(handleProblem(handle)).toBe(handle === 'me' ? 'invalid' : 'reserved');
  });
});
