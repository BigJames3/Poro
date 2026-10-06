import { Controller, Get, Inject, Optional, Res } from '@nestjs/common';
import { SkipThrottle } from '@nestjs/throttler';
import type { Response } from 'express';
import type Redis from 'ioredis';

import { Public } from '../auth/auth.guard';
import { PrismaService } from '../prisma/prisma.service';
import { REDIS_CLIENT } from '../redis.module';
import { SERVICE_NAME, SERVICE_VERSION } from '../version';

const CHECK_TIMEOUT_MS = 2_000;

type Check = () => Promise<unknown>;

function withTimeout(check: Check): Promise<unknown> {
  let timer: NodeJS.Timeout | undefined;
  const timeout = new Promise((_, reject) => {
    timer = setTimeout(() => {
      reject(new Error('timeout'));
    }, CHECK_TIMEOUT_MS);
  });
  return Promise.race([check(), timeout]).finally(() => {
    clearTimeout(timer);
  });
}

/**
 * Liveness checks nothing, so a database outage does not restart every pod.
 * Readiness checks Postgres and Redis; Kafka is left out on purpose: profiles
 * keep being served while the broker is down.
 */
@Public()
@SkipThrottle()
@Controller('health')
export class HealthController {
  private readonly checks: Record<string, Check>;

  constructor(prisma: PrismaService, @Optional() @Inject(REDIS_CLIENT) redis?: Redis) {
    this.checks = { postgres: () => prisma.ping() };
    if (redis) {
      this.checks.redis = () => redis.ping();
    }
  }

  @Get('live')
  live(@Res() res: Response): void {
    res.json({ status: 'ok', service: SERVICE_NAME, version: SERVICE_VERSION });
  }

  @Get()
  async health(@Res() res: Response): Promise<void> {
    await this.ready(res);
  }

  @Get('ready')
  async ready(@Res() res: Response): Promise<void> {
    const names = Object.keys(this.checks).sort();
    const results = await Promise.allSettled(names.map((name) => withTimeout(this.checks[name])));
    const checks: Record<string, 'up' | 'down'> = {};
    names.forEach((name, i) => {
      checks[name] = results[i].status === 'fulfilled' ? 'up' : 'down';
    });
    const ok = Object.values(checks).every((state) => state === 'up');
    res.status(ok ? 200 : 503).json({
      status: ok ? 'ok' : 'degraded',
      service: SERVICE_NAME,
      version: SERVICE_VERSION,
      checks,
    });
  }
}
