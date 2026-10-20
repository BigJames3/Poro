import {
  Inject,
  Injectable,
  Logger,
  OnApplicationBootstrap,
  OnApplicationShutdown,
} from '@nestjs/common';

import { appConfig, type AppConfigType } from '../config/app.config';
import { AvatarService } from './avatar.service';

const HOUR_MS = 3_600_000;

/**
 * Deletes, once an hour, the originals uploaded but never confirmed
 * (PORO-008). The first pass runs an hour after start, so a restart loop
 * never hammers storage. Several replicas may run it: deleting an object
 * twice is harmless.
 */
@Injectable()
export class UploadPurgeTask implements OnApplicationBootstrap, OnApplicationShutdown {
  private readonly logger = new Logger(UploadPurgeTask.name);
  private timer: NodeJS.Timeout | undefined;
  private running: Promise<number> | undefined;

  constructor(
    private readonly media: AvatarService,
    @Inject(appConfig.KEY) private readonly config: AppConfigType,
  ) {}

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

  /** One pass; a storage error is logged and the next pass tries again. */
  async run(now: Date = new Date()): Promise<number> {
    const before = new Date(now.getTime() - this.config.uploadPurgeAfterHours * HOUR_MS);
    try {
      const removed = await this.media.purgeStaleUploads(before);
      if (removed > 0) {
        this.logger.log({ msg: 'stale uploads purged', count: removed });
      }
      return removed;
    } catch (err) {
      this.logger.error({ msg: 'stale upload purge failed', err });
      return 0;
    }
  }
}
