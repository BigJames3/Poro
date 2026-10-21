import {
  Inject,
  Injectable,
  Logger,
  OnApplicationBootstrap,
  OnApplicationShutdown,
  Optional,
} from '@nestjs/common';

import { isUuid } from '../common/uuid';
import { appConfig, type AppConfigType } from '../config/app.config';
import { AuthUserDeletedV1, TYPE_AUTH_USER_DELETED } from '../events/catalog';
import { Envelope } from '../events/envelope';
import { KafkaEventConsumer, PermanentError } from '../events/event-consumer';
import { claim } from '../events/inbox';
import { PrismaService } from '../prisma/prisma.service';
import { MetricsService } from '../metrics/metrics';

export const ACCOUNT_ERASURE_GROUP = 'poro-notification-account-erasure';

/**
 * Forgets an erased auth account (ADR-0018): its devices, preferences,
 * projection, the notifications it received and those it acted in. The
 * erasure and the inbox row commit together. erased_users keeps the id so the
 * dispatcher never writes them back from a late or replayed event.
 */
@Injectable()
export class AccountErasureConsumer implements OnApplicationBootstrap, OnApplicationShutdown {
  private readonly logger = new Logger(AccountErasureConsumer.name);
  private consumer: KafkaEventConsumer | undefined;

  constructor(
    private readonly prisma: PrismaService,
    @Inject(appConfig.KEY) private readonly config: AppConfigType,
    @Optional() private readonly metrics?: MetricsService,
  ) {}

  onApplicationBootstrap(): void {
    if (!this.config.kafkaConsumerEnabled) {
      return;
    }
    this.consumer = new KafkaEventConsumer(
      this.config.kafkaBrokers,
      {
        group: ACCOUNT_ERASURE_GROUP,
        topics: [TYPE_AUTH_USER_DELETED],
        records: this.metrics?.kafkaRecords,
      },
      (env) => this.handle(env),
    );
    this.consumer.start();
  }

  async onApplicationShutdown(): Promise<void> {
    await this.consumer?.stop();
  }

  async handle(env: Envelope): Promise<void> {
    if (env.type !== TYPE_AUTH_USER_DELETED || env.version !== 1) {
      throw new PermanentError(`unsupported event ${env.type} v${env.version}`);
    }
    const data = env.data as Partial<AuthUserDeletedV1>;
    if (!isUuid(data.user_id)) {
      throw new PermanentError('user_id must be a UUID');
    }
    const erasedAt = typeof data.deleted_at === 'string' ? new Date(data.deleted_at) : null;
    if (erasedAt === null || Number.isNaN(erasedAt.getTime())) {
      throw new PermanentError('deleted_at must be a date-time');
    }
    const userId = data.user_id.toLowerCase();

    await this.prisma.$transaction(async (tx) => {
      if (!(await claim(tx, ACCOUNT_ERASURE_GROUP, env.id))) {
        return;
      }
      await tx.erasedUser.createMany({ data: [{ userId, erasedAt }], skipDuplicates: true });
      await tx.device.deleteMany({ where: { userId } });
      await tx.preference.deleteMany({ where: { userId } });
      await tx.userProjection.deleteMany({ where: { userId } });
      // A grouped notification of another recipient names only its newest
      // actor but keeps every actor id: it is deleted whole as soon as the
      // account is one of them, rather than re-rendered without it. Its
      // notification_actors rows go with it (ON DELETE CASCADE).
      const notifications = await tx.notification.deleteMany({
        where: {
          OR: [{ userId }, { actorId: userId }, { actors: { some: { actorId: userId } } }],
        },
      });
      this.logger.log({
        msg: 'account erased',
        user_id: userId,
        notifications: notifications.count,
      });
    });
  }
}
