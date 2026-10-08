import { Inject, Injectable, Logger, OnApplicationShutdown } from '@nestjs/common';

import { MetricsService } from '../metrics/metrics';
import { PrismaService } from '../prisma/prisma.service';
import { PUSH_SENDER, type PushMessage, type PushSender } from './push.sender';

/**
 * Sends a push to every device of an account. It runs after the notification
 * is committed and never throws: a failed push must not undo nor retry the
 * in-app notification.
 */
@Injectable()
export class PushService implements OnApplicationShutdown {
  private readonly logger = new Logger(PushService.name);

  constructor(
    private readonly prisma: PrismaService,
    @Inject(PUSH_SENDER) private readonly sender: PushSender,
    private readonly metrics: MetricsService,
  ) {}

  get enabled(): boolean {
    return this.sender.enabled;
  }

  async deliver(userId: string, message: PushMessage): Promise<void> {
    if (!this.sender.enabled) {
      return;
    }
    try {
      const devices = await this.prisma.device.findMany({
        where: { userId },
        select: { fcmToken: true },
      });
      if (devices.length === 0) {
        return;
      }
      const result = await this.sender.send(
        devices.map((device) => device.fcmToken),
        message,
      );
      this.metrics.push('sent', result.sent);
      this.metrics.push('failed', result.failed - result.invalidTokens.length);
      this.metrics.push('invalid_token', result.invalidTokens.length);
      if (result.invalidTokens.length > 0) {
        await this.prisma.device.deleteMany({
          where: { userId, fcmToken: { in: result.invalidTokens } },
        });
        this.logger.log({
          msg: 'invalid fcm tokens removed',
          user_id: userId,
          count: result.invalidTokens.length,
        });
      }
    } catch (err) {
      this.metrics.push('error');
      this.logger.warn({ msg: 'push failed', user_id: userId, err });
    }
  }

  async onApplicationShutdown(): Promise<void> {
    await this.sender.close().catch(() => undefined);
  }
}
