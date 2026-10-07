import { signingKey } from '../../test/support/jwt';
import { JwksKeyStore, KeysUnavailableError, UnknownKeyError } from './jwks';

function jwksFetch(state: {
  keys: unknown[];
  status: number;
  calls: number;
  body?: string;
}): typeof fetch {
  return () => {
    state.calls++;
    const body = state.body ?? JSON.stringify({ keys: state.keys });
    return Promise.resolve(new Response(body, { status: state.status }));
  };
}

describe('JwksKeyStore', () => {
  const current = signingKey();
  const next = signingKey();

  it('caches keys and follows rotation at most once per gap', async () => {
    let now = 0;
    const state = { keys: [current.publicJwk], status: 200, calls: 0 };
    const store = new JwksKeyStore('http://auth/jwks', {
      fetchFn: jwksFetch(state),
      now: () => now,
    });

    await expect(store.key(current.kid)).resolves.toBeDefined();
    await store.key(current.kid);
    expect(state.calls).toBe(1);

    state.keys = [current.publicJwk, next.publicJwk];
    now = 10_000;
    await expect(store.key(next.kid)).rejects.toBeInstanceOf(UnknownKeyError);
    expect(state.calls).toBe(1);
    now = 31_000;
    await expect(store.key(next.kid)).resolves.toBeDefined();
    expect(state.calls).toBe(2);
  });

  it('keeps stale keys while auth is down and reports missing keys as unavailable', async () => {
    let now = 0;
    const state = { keys: [current.publicJwk], status: 200, calls: 0 };
    const store = new JwksKeyStore('http://auth/jwks', {
      fetchFn: jwksFetch(state),
      now: () => now,
    });
    await store.key(current.kid);

    state.status = 503;
    now = 6 * 60_000;
    await expect(store.key(current.kid)).resolves.toBeDefined();
    expect(state.calls).toBe(2);
    now += 31_000;
    await expect(store.key(next.kid)).rejects.toBeInstanceOf(KeysUnavailableError);

    state.status = 200;
    state.keys = [next.publicJwk];
    now += 31_000;
    await expect(store.key(next.kid)).resolves.toBeDefined();
    await expect(store.key(current.kid)).rejects.toBeInstanceOf(UnknownKeyError);
  });

  it('shares one request between concurrent lookups', async () => {
    const state = { keys: [current.publicJwk], status: 200, calls: 0 };
    const store = new JwksKeyStore('http://auth/jwks', {
      fetchFn: jwksFetch(state),
    });
    await Promise.all([store.key(current.kid), store.key(current.kid), store.key(current.kid)]);
    expect(state.calls).toBe(1);
  });

  it.each([
    ['weak RSA key', { keys: [signingKey(1024).publicJwk] }],
    ['wrong algorithm', { keys: [{ ...current.publicJwk, alg: 'RS512' }] }],
    ['encryption key', { keys: [{ ...current.publicJwk, use: 'enc' }] }],
    ['not RSA', { keys: [{ kty: 'EC', kid: 'ec', crv: 'P-256', x: 'a', y: 'b' }] }],
    ['garbage modulus', { keys: [{ ...current.publicJwk, n: '!!' }] }],
    ['missing keys', {}],
  ])('rejects a JWKS with %s', async (_name, doc) => {
    const state = {
      keys: [],
      status: 200,
      calls: 0,
      body: JSON.stringify(doc),
    };
    const store = new JwksKeyStore('http://auth/jwks', {
      fetchFn: jwksFetch(state),
    });
    await expect(store.key(current.kid)).rejects.toBeInstanceOf(KeysUnavailableError);
  });

  it('rejects oversized documents', async () => {
    const state = {
      keys: [],
      status: 200,
      calls: 0,
      body: 'x'.repeat(70 * 1024),
    };
    const store = new JwksKeyStore('http://auth/jwks', {
      fetchFn: jwksFetch(state),
    });
    await expect(store.key(current.kid)).rejects.toBeInstanceOf(KeysUnavailableError);
  });
});
