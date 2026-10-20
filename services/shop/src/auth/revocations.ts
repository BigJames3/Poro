import { Global, Inject, Logger, Module, OnApplicationShutdown, Optional } from '@nestjs/common';
import Redis from 'ioredis';

import { appConfig, type AppConfigType } from '../config/app.config';

/** Shared with the Go and Python services: auth writes these keys (ADR-0016). */
export const REVOCATION_KEY_PREFIX = 'poro:revoked:sid:';
export const REVOCATIONS = Symbol('REVOCATIONS');

/** The registry could not be read: the request is refused with 503. */
export class RevocationUnavailableError extends Error {
  constructor(cause: unknown) {
    super('revocation registry unavailable', { cause });
    this.name = 'RevocationUnavailableError';
  }
}

/** Tells whether auth revoked a device session. */
export interface Revocations {
  isRevoked(sessionId: string): Promise<boolean>;
}

/** Reads the registry of revoked sessions that auth writes in Redis. */
export class RedisRevocations implements Revocations {
  constructor(private readonly redis: Redis) {}

  /**
   * Every authenticated request reads the registry, so an outage must fail
   * fast: short timeouts and a single retry.
   */
  static connect(url: string): RedisRevocations {
    return new RedisRevocations(
      new Redis(url, { maxRetriesPerRequest: 1, connectTimeout: 1_000, commandTimeout: 500 }),
    );
  }

  async isRevoked(sessionId: string): Promise<boolean> {
    try {
      return (await this.redis.exists(REVOCATION_KEY_PREFIX + sessionId.toLowerCase())) > 0;
    } catch (err) {
      throw new RevocationUnavailableError(err);
    }
  }

  /** Quits cleanly when connected; otherwise stops reconnecting at once. */
  async close(): Promise<void> {
    if (this.redis.status === 'ready') {
      await this.redis.quit().catch(() => undefined);
    } else {
      this.redis.disconnect();
    }
  }
}

/**
 * Provides the registry. Without REVOCATION_REDIS_URL (allowed in dev only)
 * there is none: revoked sessions stay accepted until their tokens expire.
 */
@Global()
@Module({
  providers: [
    {
      provide: REVOCATIONS,
      inject: [appConfig.KEY],
      useFactory: (config: AppConfigType): RedisRevocations | undefined => {
        if (config.revocationRedisUrl === undefined) {
          new Logger('Revocations').warn(
            'REVOCATION_REDIS_URL is not set: revoked sessions stay accepted until their tokens expire',
          );
          return undefined;
        }
        return RedisRevocations.connect(config.revocationRedisUrl);
      },
    },
  ],
  exports: [REVOCATIONS],
})
export class RevocationModule implements OnApplicationShutdown {
  constructor(@Optional() @Inject(REVOCATIONS) private readonly revocations?: RedisRevocations) {}

  async onApplicationShutdown(): Promise<void> {
    await this.revocations?.close();
  }
}
