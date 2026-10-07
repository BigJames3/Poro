import { Controller, Get, Injectable, Res } from '@nestjs/common';
import { SkipThrottle } from '@nestjs/throttler';
import { Counter, Histogram, Registry, collectDefaultMetrics } from '@prometheus-io/client';
import type { NextFunction, Request, Response } from 'express';

import { Public } from '../auth/auth.guard';
import { SERVICE_NAME } from '../version';

export type PushOutcome = 'sent' | 'failed' | 'invalid_token' | 'error';

/**
 * HTTP metrics with the same names and labels as shared-go/metrics, plus the
 * notifications written and the pushes attempted.
 */
@Injectable()
export class MetricsService {
  readonly registry = new Registry();
  private readonly requests: Counter<'method' | 'route' | 'status'>;
  private readonly duration: Histogram<'method' | 'route'>;
  private readonly notifications: Counter<'type'>;
  private readonly pushes: Counter<'outcome'>;

  constructor() {
    this.registry.setDefaultLabels({ service: SERVICE_NAME });
    collectDefaultMetrics({ register: this.registry });
    this.requests = new Counter({
      name: 'http_requests_total',
      help: 'HTTP requests by method, route template and status.',
      labelNames: ['method', 'route', 'status'],
      registers: [this.registry],
    });
    this.duration = new Histogram({
      name: 'http_request_duration_seconds',
      help: 'HTTP request latency by method and route template.',
      labelNames: ['method', 'route'],
      buckets: [0.005, 0.01, 0.025, 0.05, 0.1, 0.25, 0.5, 1, 2.5, 5, 10],
      registers: [this.registry],
    });
    this.notifications = new Counter({
      name: 'notifications_created_total',
      help: 'In-app notifications created, by type. Grouped likes count once per group.',
      labelNames: ['type'],
      registers: [this.registry],
    });
    this.pushes = new Counter({
      name: 'notification_pushes_total',
      help: 'FCM push deliveries per device token, by outcome; error counts whole failed sends.',
      labelNames: ['outcome'],
      registers: [this.registry],
    });
  }

  notificationCreated(type: string): void {
    this.notifications.inc({ type });
  }

  push(outcome: PushOutcome, count = 1): void {
    if (count > 0) {
      this.pushes.inc({ outcome }, count);
    }
  }

  /** Labels by route template, never by raw path, to keep cardinality bounded. */
  middleware(): (req: Request, res: Response, next: NextFunction) => void {
    return (req, res, next) => {
      const start = process.hrtime.bigint();
      res.on('finish', () => {
        const route = (req.route as { path?: unknown } | undefined)?.path;
        const labels = {
          method: req.method,
          route: typeof route === 'string' ? route : 'unmatched',
        };
        this.requests.inc({ ...labels, status: String(res.statusCode) });
        this.duration.observe(labels, Number(process.hrtime.bigint() - start) / 1e9);
      });
      next();
    };
  }
}

/** /metrics must not be exposed by the public gateway. */
@Public()
@SkipThrottle()
@Controller('metrics')
export class MetricsController {
  constructor(private readonly metrics: MetricsService) {}

  @Get()
  async scrape(@Res() res: Response): Promise<void> {
    res.setHeader('Content-Type', this.metrics.registry.contentType);
    res.send(await this.metrics.registry.metrics());
  }
}
