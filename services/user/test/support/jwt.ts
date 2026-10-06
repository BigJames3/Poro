import { KeyObject, createHash, generateKeyPairSync } from 'node:crypto';
import { Server, createServer } from 'node:http';
import { AddressInfo } from 'node:net';

import { SignJWT } from 'jose';

import { uuidv7 } from '../../src/common/uuid';

export interface SigningKey {
  kid: string;
  privateKey: KeyObject;
  publicJwk: Record<string, unknown>;
}

export function signingKey(bits = 2048): SigningKey {
  const { privateKey, publicKey } = generateKeyPairSync('rsa', { modulusLength: bits });
  const jwk = publicKey.export({ format: 'jwk' });
  const kid = createHash('sha256').update(`${jwk.n}.${jwk.e}`).digest('base64url');
  return { kid, privateKey, publicJwk: { ...jwk, kid, use: 'sig', alg: 'RS256' } };
}

export interface TokenOptions {
  sub?: string;
  roles?: unknown;
  issuer?: string;
  audience?: string;
  expiresIn?: string | number;
  kid?: string | null;
  sid?: string | null;
}

export async function signToken(key: SigningKey, opts: TokenOptions = {}): Promise<string> {
  const claims: Record<string, unknown> = { roles: opts.roles ?? ['PERSONAL'] };
  if (opts.sid !== null) {
    claims.sid = opts.sid ?? uuidv7();
  }
  const jwt = new SignJWT(claims)
    .setProtectedHeader(
      opts.kid === null ? { alg: 'RS256' } : { alg: 'RS256', kid: opts.kid ?? key.kid },
    )
    .setSubject(opts.sub ?? uuidv7())
    .setIssuer(opts.issuer ?? 'poro-auth')
    .setAudience(opts.audience ?? 'poro-api')
    .setJti(uuidv7())
    .setIssuedAt()
    .setExpirationTime(opts.expiresIn ?? '15m');
  return jwt.sign(key.privateKey);
}

/** Serves a mutable JWKS document, like the auth service. */
export class JwksServer {
  keys: Record<string, unknown>[] = [];
  status = 200;
  requests = 0;
  private server: Server | undefined;

  async start(): Promise<string> {
    this.server = createServer((_req, res) => {
      this.requests++;
      res.statusCode = this.status;
      res.setHeader('content-type', 'application/json');
      res.end(JSON.stringify({ keys: this.keys }));
    });
    await new Promise<void>((resolve) => this.server?.listen(0, '127.0.0.1', resolve));
    const { port } = this.server.address() as AddressInfo;
    return `http://127.0.0.1:${port}/.well-known/jwks.json`;
  }

  async stop(): Promise<void> {
    await new Promise((resolve) => this.server?.close(resolve));
  }
}
