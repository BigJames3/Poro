import { ThrottlerStorageRedisService } from '@nest-lab/throttler-storage-redis';
import { Module } from '@nestjs/common';
import { ConfigModule } from '@nestjs/config';
import { APP_FILTER, APP_GUARD, APP_INTERCEPTOR } from '@nestjs/core';
import { ThrottlerGuard, ThrottlerModule } from '@nestjs/throttler';
import { trace } from '@opentelemetry/api';
import type Redis from 'ioredis';
import { LoggerModule } from 'nestjs-pino';

import { AuthGuard } from './auth/auth.guard';
import { TokenVerifier } from './auth/token-verifier';
import { AvatarService, S3_CLIENT, S3_PRESIGN_CLIENT, s3ClientFor } from './avatars/avatar.service';
import { EnvelopeInterceptor } from './common/envelope.interceptor';
import { HttpExceptionFilter } from './common/http-exception.filter';
import { PoroRequest, requestIdOf } from './common/request';
import { appConfig, type AppConfigType } from './config/app.config';
import { HealthController } from './health/health.controller';
import { MetricsController, MetricsService } from './metrics/metrics';
import { PrismaService } from './prisma/prisma.service';
import { ProfilesController } from './profiles/profiles.controller';
import { ProfilesService } from './profiles/profiles.service';
import { REDIS_CLIENT, RedisModule } from './redis.module';
import { UserCreatedConsumer } from './profiles/user-created.consumer';
import { SERVICE_NAME, SERVICE_VERSION } from './version';

/** Rate limits per account when authenticated, per client IP otherwise. */
class AccountThrottlerGuard extends ThrottlerGuard {
  protected override getTracker(req: Record<string, unknown>): Promise<string> {
    const request = req as unknown as PoroRequest;
    return Promise.resolve(request.user ? `user:${request.user.userId}` : `ip:${request.ip ?? ''}`);
  }
}

@Module({
  imports: [
    ConfigModule.forRoot({ isGlobal: true, cache: true, load: [appConfig] }),
    LoggerModule.forRootAsync({
      inject: [appConfig.KEY],
      useFactory: (config: AppConfigType) => ({
        pinoHttp: {
          level: config.logLevel,
          base: { service: SERVICE_NAME, version: SERVICE_VERSION },
          genReqId: (req) => requestIdOf(req as PoroRequest),
          customAttributeKeys: { reqId: 'request_id' },
          redact: ['req.headers.authorization', 'req.headers.cookie'],
          serializers: {
            req: (req: { id: string; method: string; url: string }) => ({
              id: req.id,
              method: req.method,
              url: req.url.split('?')[0],
            }),
          },
          customLogLevel: (req, res, err) => {
            if (res.statusCode >= 500 || err) {
              return 'error';
            }
            return req.url?.startsWith('/health') || req.url === '/metrics' ? 'debug' : 'info';
          },
          mixin: () => {
            const span = trace.getActiveSpan();
            return span ? { trace_id: span.spanContext().traceId } : {};
          },
        },
      }),
    }),
    RedisModule,
    ThrottlerModule.forRootAsync({
      inject: [{ token: REDIS_CLIENT, optional: true }],
      useFactory: (redis?: Redis) => ({
        throttlers: [{ name: 'default', ttl: 60_000, limit: 120 }],
        storage: redis ? new ThrottlerStorageRedisService(redis) : undefined,
        errorMessage: 'too many requests',
      }),
    }),
  ],
  controllers: [HealthController, MetricsController, ProfilesController],
  providers: [
    PrismaService,
    MetricsService,
    TokenVerifier,
    AvatarService,
    ProfilesService,
    UserCreatedConsumer,

    {
      provide: S3_CLIENT,
      inject: [appConfig.KEY],
      useFactory: (config: AppConfigType) => s3ClientFor(config, config.s3.endpoint),
    },
    {
      provide: S3_PRESIGN_CLIENT,
      inject: [appConfig.KEY],
      useFactory: (config: AppConfigType) => s3ClientFor(config, config.s3.publicEndpoint),
    },
    { provide: APP_GUARD, useClass: AuthGuard },
    { provide: APP_GUARD, useClass: AccountThrottlerGuard },
    { provide: APP_INTERCEPTOR, useClass: EnvelopeInterceptor },
    { provide: APP_FILTER, useClass: HttpExceptionFilter },
  ],
})
export class AppModule {}
