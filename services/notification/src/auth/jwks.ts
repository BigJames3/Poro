import { KeyObject, createPublicKey } from 'node:crypto';

/** The auth JWKS could not be fetched and no cached key matches. */
export class KeysUnavailableError extends Error {
  constructor(cause?: unknown) {
    super('jwks unavailable', { cause });
    this.name = 'KeysUnavailableError';
  }
}

/** The token names a key the auth service does not publish. */
export class UnknownKeyError extends Error {
  constructor(kid: string) {
    super(`unknown signing key "${kid}"`);
    this.name = 'UnknownKeyError';
  }
}

export interface JwksOptions {
  maxAgeMs?: number;
  minRefreshGapMs?: number;
  timeoutMs?: number;
  fetchFn?: typeof fetch;
  now?: () => number;
}

interface Jwk {
  kty?: unknown;
  kid?: unknown;
  use?: unknown;
  alg?: unknown;
  n?: unknown;
  e?: unknown;
}

const MAX_JWKS_BYTES = 64 * 1024;
const MIN_RSA_BITS = 2048;

/**
 * Caches the auth public keys. Keys are refreshed when older than maxAge, or
 * when a token names an unknown kid, at most once per minRefreshGap. When auth
 * is down the last good keys keep verifying tokens.
 */
export class JwksKeyStore {
  private keys = new Map<string, KeyObject>();
  private fetchedAt = 0;
  private lastAttempt = Number.NEGATIVE_INFINITY;
  private lastError: unknown = undefined;
  private inflight: Promise<void> | undefined;
  private readonly maxAgeMs: number;
  private readonly minRefreshGapMs: number;
  private readonly timeoutMs: number;
  private readonly fetchFn: typeof fetch;
  private readonly now: () => number;

  constructor(
    private readonly url: string,
    options: JwksOptions = {},
  ) {
    this.maxAgeMs = options.maxAgeMs ?? 5 * 60_000;
    this.minRefreshGapMs = options.minRefreshGapMs ?? 30_000;
    this.timeoutMs = options.timeoutMs ?? 3_000;
    this.fetchFn = options.fetchFn ?? fetch;
    this.now = options.now ?? Date.now;
  }

  async key(kid: string): Promise<KeyObject> {
    const stale = this.now() - this.fetchedAt > this.maxAgeMs;
    if ((stale || !this.keys.has(kid)) && this.now() - this.lastAttempt >= this.minRefreshGapMs) {
      await this.refresh();
    }
    const key = this.keys.get(kid);
    if (key) {
      return key;
    }
    if (this.lastError !== undefined) {
      throw new KeysUnavailableError(this.lastError);
    }
    throw new UnknownKeyError(kid);
  }

  private refresh(): Promise<void> {
    this.inflight ??= this.fetchKeys()
      .then((keys) => {
        this.keys = keys;
        this.fetchedAt = this.now();
        this.lastError = undefined;
      })
      .catch((err: unknown) => {
        this.lastError = err;
      })
      .finally(() => {
        this.lastAttempt = this.now();
        this.inflight = undefined;
      });
    return this.inflight;
  }

  private async fetchKeys(): Promise<Map<string, KeyObject>> {
    const res = await this.fetchFn(this.url, {
      headers: { accept: 'application/json' },
      signal: AbortSignal.timeout(this.timeoutMs),
    });
    if (!res.ok) {
      throw new Error(`jwks: status ${res.status}`);
    }
    const body = await res.text();
    if (body.length > MAX_JWKS_BYTES) {
      throw new Error('jwks: document too large');
    }
    const doc = JSON.parse(body) as { keys?: unknown };
    if (!Array.isArray(doc.keys)) {
      throw new Error('jwks: missing keys');
    }
    const keys = new Map<string, KeyObject>();
    for (const jwk of doc.keys as Jwk[]) {
      const parsed = rsaKey(jwk);
      if (parsed) {
        keys.set(parsed.kid, parsed.key);
      }
    }
    if (keys.size === 0) {
      throw new Error('jwks: no usable RS256 key');
    }
    return keys;
  }
}

function rsaKey(jwk: Jwk): { kid: string; key: KeyObject } | undefined {
  if (
    jwk.kty !== 'RSA' ||
    typeof jwk.kid !== 'string' ||
    jwk.kid === '' ||
    typeof jwk.n !== 'string' ||
    typeof jwk.e !== 'string' ||
    (jwk.use !== undefined && jwk.use !== 'sig') ||
    (jwk.alg !== undefined && jwk.alg !== 'RS256')
  ) {
    return undefined;
  }
  try {
    const key = createPublicKey({
      key: { kty: 'RSA', n: jwk.n, e: jwk.e },
      format: 'jwk',
    });
    const details = key.asymmetricKeyDetails;
    const exponent = details?.publicExponent ?? 0n;
    if ((details?.modulusLength ?? 0) < MIN_RSA_BITS || exponent < 3n || exponent % 2n === 0n) {
      return undefined;
    }
    return { kid: jwk.kid, key };
  } catch {
    return undefined;
  }
}
