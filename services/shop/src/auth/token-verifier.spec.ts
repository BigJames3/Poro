import { JwksServer, signToken, signingKey } from '../../test/support/jwt';
import { uuidv7 } from '../common/uuid';
import { InvalidTokenError, KeysUnavailableError, TokenVerifier } from './token-verifier';

describe('TokenVerifier', () => {
  const key = signingKey();
  const attacker = signingKey();
  const jwks = new JwksServer();
  let verifier: TokenVerifier;

  beforeAll(async () => {
    jwks.keys = [key.publicJwk];
    verifier = new TokenVerifier({ jwksUrl: await jwks.start() });
  });

  afterAll(async () => {
    await jwks.stop();
  });

  it('accepts auth access tokens', async () => {
    const userId = uuidv7();
    const user = await verifier.verify(
      await signToken(key, { sub: userId, roles: ['PERSONAL', 'CREATOR'] }),
    );
    expect(user).toEqual({
      userId,
      roles: ['PERSONAL', 'CREATOR'],
      sessionId: expect.any(String),
      tokenId: expect.any(String),
    });
  });

  it.each([
    ['expired', { expiresIn: Math.floor(Date.now() / 1000) - 60 }],
    ['wrong issuer', { issuer: 'someone-else' }],
    ['wrong audience', { audience: 'poro-admin' }],
    ['non-uuid subject', { sub: 'admin' }],
    ['roles not a list', { roles: 'ADMIN' }],
    ['no session', { sid: null }],
    ['no kid', { kid: null }],
    ['unknown kid', { kid: 'unknown' }],
  ])('rejects a token with %s', async (_name, opts) => {
    await expect(verifier.verify(await signToken(key, opts))).rejects.toBeInstanceOf(
      InvalidTokenError,
    );
  });

  it('rejects forged signatures and non-RS256 tokens', async () => {
    const forged = await signToken(attacker, { kid: key.kid });
    await expect(verifier.verify(forged)).rejects.toBeInstanceOf(InvalidTokenError);

    const [, payload] = (await signToken(key)).split('.');
    const none = `${Buffer.from(JSON.stringify({ alg: 'none', kid: key.kid })).toString('base64url')}.${payload}.`;
    await expect(verifier.verify(none)).rejects.toBeInstanceOf(InvalidTokenError);
    await expect(verifier.verify('garbage')).rejects.toBeInstanceOf(InvalidTokenError);
  });

  it('reports unreachable keys separately from bad tokens', async () => {
    const down = new TokenVerifier({ jwksUrl: 'http://127.0.0.1:9/.well-known/jwks.json' });
    await expect(down.verify(await signToken(key))).rejects.toBeInstanceOf(KeysUnavailableError);
  });
});
