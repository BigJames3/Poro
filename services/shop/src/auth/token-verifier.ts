import { Inject, Injectable } from '@nestjs/common';
import { JWTPayload, jwtVerify } from 'jose';

import { AuthUser } from '../common/request';
import { isUuid } from '../common/uuid';
import { appConfig, type AppConfigType } from '../config/app.config';
import { JwksKeyStore, KeysUnavailableError } from './jwks';

export const TOKEN_ISSUER = 'poro-auth';
export const TOKEN_AUDIENCE = 'poro-api';

/** The token is not acceptable. The reason is for logs only. */
export class InvalidTokenError extends Error {
  constructor(reason: string, cause?: unknown) {
    super(reason, { cause });
    this.name = 'InvalidTokenError';
  }
}

export { KeysUnavailableError };

/**
 * Verifies auth access tokens offline against the auth JWKS. A token revoked by
 * logout stays accepted here until it expires (15 minutes at most).
 */
@Injectable()
export class TokenVerifier {
  private readonly keys: JwksKeyStore;

  constructor(@Inject(appConfig.KEY) config: Pick<AppConfigType, 'jwksUrl'>) {
    this.keys = new JwksKeyStore(config.jwksUrl);
  }

  async verify(token: string): Promise<AuthUser> {
    let payload: JWTPayload;
    try {
      ({ payload } = await jwtVerify(
        token,
        async (header) => {
          if (typeof header.kid !== 'string' || header.kid === '') {
            throw new InvalidTokenError('missing kid');
          }
          return this.keys.key(header.kid);
        },
        {
          algorithms: ['RS256'],
          issuer: TOKEN_ISSUER,
          audience: TOKEN_AUDIENCE,
          requiredClaims: ['sub', 'exp', 'jti', 'sid'],
          clockTolerance: 5,
        },
      ));
    } catch (err) {
      if (err instanceof KeysUnavailableError || err instanceof InvalidTokenError) {
        throw err;
      }
      throw new InvalidTokenError(err instanceof Error ? err.message : 'invalid token', err);
    }

    const roles: unknown = payload.roles;
    if (
      !isUuid(payload.sub) ||
      !Array.isArray(roles) ||
      !roles.every((r) => typeof r === 'string')
    ) {
      throw new InvalidTokenError('malformed claims');
    }
    if (typeof payload.sid !== 'string' || typeof payload.jti !== 'string') {
      throw new InvalidTokenError('malformed claims');
    }
    return {
      userId: payload.sub.toLowerCase(),
      roles,
      sessionId: payload.sid,
      tokenId: payload.jti,
    };
  }
}
