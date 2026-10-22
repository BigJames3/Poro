import { Injectable, Logger, OnApplicationBootstrap, OnApplicationShutdown } from '@nestjs/common';

import { MediaService } from './media.service';

/** Originals left unconfirmed for longer than this are deleted (PORO-008). */
export const ABANDONED_UPLOAD_AGE_MS = 24 * 60 * 60 * 1000;
export const UPLOAD_SWEEP_INTERVAL_MS = 60 * 60 * 1000;

/**
 * Deletes, every hour, the logo and product photo originals that were uploaded but never
 * confirmed. Every replica may run it: deleting a deleted object succeeds.
 */
@Injectable()
export class UploadSweeper implements OnApplicationBootstrap, OnApplicationShutdown {
  private readonly logger = new Logger(UploadSweeper.name);
  private timer: NodeJS.Timeout | undefined;
  private running: Promise<void> | undefined;

  constructor(private readonly media: MediaService) {}

  onApplicationBootstrap(): void {
    this.timer = setInterval(() => {
      this.running ??= this.tick().finally(() => {
        this.running = undefined;
      });
    }, UPLOAD_SWEEP_INTERVAL_MS);
    this.timer.unref();
  }

  async onApplicationShutdown(): Promise<void> {
    clearInterval(this.timer);
    await this.running;
  }

  async tick(now: Date = new Date()): Promise<void> {
    try {
      const count = await this.media.purgeAbandonedUploads(
        new Date(now.getTime() - ABANDONED_UPLOAD_AGE_MS),
      );
      if (count > 0) {
        this.logger.log({ msg: 'abandoned image uploads deleted', count });
      }
    } catch (err) {
      this.logger.error({ msg: 'abandoned image uploads not deleted', err });
    }
  }
}
