import {
  Inject,
  Injectable,
  Logger,
  OnApplicationBootstrap,
  OnApplicationShutdown,
} from '@nestjs/common';

import { appConfig, type AppConfigType } from '../config/app.config';
import { PrismaService } from '../prisma/prisma.service';

const DAY_MS = 24 * 60 * 60 * 1000;
const FIRST_RUN_DELAY_MS = 60_000;
const BATCH_SIZE = 5_000;
/** Kafka keeps events 7 days: an older inbox row can no longer dedupe anything. */
const INBOX_RETENTION_DAYS = 30;

/**
 * Deletes notifications without activity for retentionDays, and old inbox
 * rows, once a day. Every replica runs it; the deletes are idempotent.
 */
@Injectable()
export class PurgeService implements OnApplicationBootstrap, OnApplicationShutdown {
  private readonly logger = new Logger(PurgeService.name);
  private timer: NodeJS.Timeout | undefined;

  constructor(
    private readonly prisma: PrismaService,
    @Inject(appConfig.KEY) private readonly config: AppConfigType,
  ) {}

  onApplicationBootstrap(): void {
    if (!this.config.purgeEnabled) {
      return;
    }
    const run = (): void => {
      this.purge().catch((err: unknown) => {
        this.logger.error({ msg: 'purge failed', err });
      });
    };
    this.timer = setTimeout(() => {
      run();
      this.timer = setInterval(run, DAY_MS);
      this.timer.unref();
    }, FIRST_RUN_DELAY_MS);
    this.timer.unref();
  }

  onApplicationShutdown(): void {
    clearTimeout(this.timer);
  }

  async purge(now: Date = new Date()): Promise<{ notifications: number; inbox: number }> {
    const notificationsBefore = new Date(now.getTime() - this.config.retentionDays * DAY_MS);
    const inboxBefore = new Date(now.getTime() - INBOX_RETENTION_DAYS * DAY_MS);
    const notifications = await this.inBatches(
      (limit) => this.prisma.$executeRaw`
        DELETE FROM notifications WHERE id IN (
          SELECT id FROM notifications WHERE last_activity_at < ${notificationsBefore} LIMIT ${limit})`,
    );
    const inbox = await this.inBatches(
      (limit) => this.prisma.$executeRaw`
        DELETE FROM inbox_events WHERE (consumer, event_id) IN (
          SELECT consumer, event_id FROM inbox_events WHERE processed_at < ${inboxBefore} LIMIT ${limit})`,
    );
    if (notifications + inbox > 0) {
      this.logger.log({ msg: 'purge done', notifications, inbox });
    }
    return { notifications, inbox };
  }

  private async inBatches(batch: (limit: number) => Promise<number>): Promise<number> {
    let total = 0;
    for (;;) {
      const deleted = await batch(BATCH_SIZE);
      total += deleted;
      if (deleted < BATCH_SIZE) {
        return total;
      }
    }
  }
}
