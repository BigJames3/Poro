import {
  Inject,
  Injectable,
  Logger,
  OnApplicationBootstrap,
  OnApplicationShutdown,
} from '@nestjs/common';

import { appConfig, type AppConfigType } from '../config/app.config';
import { SagaService } from './saga.service';

/**
 * Runs the timed transitions every SCHEDULER_INTERVAL_MS. Rows are claimed
 * with SKIP LOCKED, so every replica can run it.
 */
@Injectable()
export class Scheduler implements OnApplicationBootstrap, OnApplicationShutdown {
  private readonly logger = new Logger(Scheduler.name);
  private timer: NodeJS.Timeout | undefined;
  private running: Promise<void> | undefined;

  constructor(
    private readonly saga: SagaService,
    @Inject(appConfig.KEY) private readonly config: AppConfigType,
  ) {}

  onApplicationBootstrap(): void {
    if (!this.config.schedulerEnabled) {
      this.logger.warn('scheduler disabled by SCHEDULER_ENABLED=false');
      return;
    }
    this.timer = setInterval(() => {
      this.running ??= this.tick().finally(() => {
        this.running = undefined;
      });
    }, this.config.schedulerIntervalMs);
  }

  async onApplicationShutdown(): Promise<void> {
    clearInterval(this.timer);
    await this.running;
  }

  /** One pass of every timed task; a failing task does not stop the others. */
  async tick(now: Date = new Date()): Promise<void> {
    const tasks: [string, () => Promise<number>][] = [
      ['expired', () => this.saga.expireUnpaid(now)],
      ['auto_completed', () => this.saga.autoComplete(now)],
      ['contacts_erased', () => this.saga.eraseContacts(now)],
    ];
    for (const [name, task] of tasks) {
      try {
        const count = await task();
        if (count > 0) {
          this.logger.log({ msg: 'scheduled task done', task: name, count });
        }
      } catch (err) {
        this.logger.error({ msg: 'scheduled task failed', task: name, err });
      }
    }
  }
}
