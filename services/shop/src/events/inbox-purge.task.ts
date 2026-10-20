import { Injectable, Logger, OnApplicationBootstrap, OnApplicationShutdown } from '@nestjs/common';

import { PrismaService } from '../prisma/prisma.service';
import { INBOX_RETENTION_MS, purgeProcessedEvents } from './outbox';

const HOUR_MS = 3_600_000;

/**
 * Deletes, once an hour, the inbox rows older than the Kafka retention
 * (PORO-019). Every replica may run it: deleting twice is harmless.
 */
@Injectable()
export class InboxPurgeTask implements OnApplicationBootstrap, OnApplicationShutdown {
  private readonly logger = new Logger(InboxPurgeTask.name);
  private timer: NodeJS.Timeout | undefined;
  private running: Promise<number> | undefined;

  constructor(private readonly prisma: PrismaService) {}

  onApplicationBootstrap(): void {
    this.timer = setInterval(() => {
      this.running ??= this.run().finally(() => {
        this.running = undefined;
      });
    }, HOUR_MS);
    this.timer.unref();
  }

  async onApplicationShutdown(): Promise<void> {
    clearInterval(this.timer);
    await this.running;
  }

  /** One pass; a failure is logged and the next pass tries again. */
  async run(now: Date = new Date()): Promise<number> {
    try {
      const deleted = await purgeProcessedEvents(
        this.prisma,
        new Date(now.getTime() - INBOX_RETENTION_MS),
      );
      if (deleted > 0) {
        this.logger.log({ msg: 'processed events purged', count: deleted });
      }
      return deleted;
    } catch (err) {
      this.logger.error({ msg: 'processed events purge failed', err });
      return 0;
    }
  }
}
