import { normalizeUsername, usernameProblem } from './username';

describe('usernames', () => {
  it('normalizes case, spaces and a leading @', () => {
    expect(normalizeUsername('  @Awa.Kone ')).toBe('awa.kone');
  });

  it.each(['awa', 'awa.kone', 'moussa_d', 'dj_2026', 'a'.repeat(30), '_underscore_'])(
    'accepts %s',
    (name) => {
      expect(usernameProblem(name)).toBeUndefined();
    },
  );

  it.each([
    ['ab', 'invalid'],
    ['a'.repeat(31), 'invalid'],
    ['awa kone', 'invalid'],
    ['awa-kone', 'invalid'],
    ['.awa', 'invalid'],
    ['awa.', 'invalid'],
    ['awa..kone', 'invalid'],
    ['2250701020304', 'invalid'],
    ['kôné', 'invalid'],
    ['admin', 'reserved'],
    ['poro', 'reserved'],
    ['poro.official', 'reserved'],
    ['p.o.r.o', 'reserved'],
  ])('rejects %s as %s', (name, problem) => {
    expect(usernameProblem(name)).toBe(problem);
  });
});
