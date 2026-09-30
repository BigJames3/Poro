import { Global, Inject, Module, OnApplicationShutdown, Optional } from '@nestjs/common';
import Redis from 'ioredis';

import { appConfig, type AppConfigType } from './config/app.config';

export const REDIS_CLIENT = Symbol('REDIS_CLIENT');

/**
 * Redis only holds rate-limit counters, never a source of truth. Without
 * REDIS_URL (dev only) the client is undefined and counters stay in memory.
 */
@Global()
@Module({
  providers: [
    {
      provide: REDIS_CLIENT,
      inject: [appConfig.KEY],
      useFactory: (config: AppConfigType): Redis | undefined =>
        config.redisUrl === undefined
          ? undefined
          : new Redis(config.redisUrl, { maxRetriesPerRequest: 2, lazyConnect: false }),
    },
  ],
  exports: [REDIS_CLIENT],
})
export class RedisModule implements OnApplicationShutdown {
  constructor(@Optional() @Inject(REDIS_CLIENT) private readonly redis?: Redis) {}

  async onApplicationShutdown(): Promise<void> {
    await this.redis?.quit().catch(() => undefined);
  }
}
