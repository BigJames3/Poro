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
import { EnvelopeInterceptor } from './common/envelope.interceptor';
import { HttpExceptionFilter } from './common/http-exception.filter';
import { PoroRequest, requestIdOf } from './common/request';
import { appConfig, type AppConfigType } from './config/app.config';
import { HealthController } from './health/health.controller';
import { MetricsController, MetricsService } from './metrics/metrics';
import { PrismaService } from './prisma/prisma.service';
import { NotificationDispatcher } from './notifications/dispatcher';
import { NotificationsController } from './notifications/notifications.controller';
import { NotificationsService } from './notifications/notifications.service';
import { PurgeService } from './notifications/purge.service';
import { PUSH_SENDER, pushSenderFor } from './push/push.sender';
import { PushService } from './push/push.service';
import { REDIS_CLIENT, RedisModule } from './redis.module';
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
  controllers: [HealthController, MetricsController, NotificationsController],
  providers: [
    PrismaService,
    MetricsService,
    TokenVerifier,
    NotificationsService,
    NotificationDispatcher,
    PushService,
    PurgeService,
    {
      provide: PUSH_SENDER,
      inject: [appConfig.KEY],
      useFactory: (config: AppConfigType) => pushSenderFor(config.fcm),
    },
    { provide: APP_GUARD, useClass: AuthGuard },
    { provide: APP_GUARD, useClass: AccountThrottlerGuard },
    { provide: APP_INTERCEPTOR, useClass: EnvelopeInterceptor },
    { provide: APP_FILTER, useClass: HttpExceptionFilter },
  ],
})
export class AppModule {}
