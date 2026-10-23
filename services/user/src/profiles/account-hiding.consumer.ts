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
import {
  AuthUserDeletionCancelledV1,
  AuthUserDeletionScheduledV1,
  TYPE_AUTH_USER_DELETION_CANCELLED,
  TYPE_AUTH_USER_DELETION_SCHEDULED,
} from '../events/catalog';
import { Envelope } from '../events/envelope';
import { KafkaEventConsumer, PermanentError } from '../events/event-consumer';
import { MetricsService } from '../metrics/metrics';
import { PrismaService } from '../prisma/prisma.service';

export const ACCOUNT_HIDING_GROUP = 'poro-user-account-hiding';

/**
 * Hides the profile of an account whose deletion is pending, and shows it
 * again when the deletion is cancelled (same rule as shared-go/hiding). The
 * two events travel on different topics: the latest changed_at wins, which
 * also makes a replay harmless, so no inbox is needed.
 */
@Injectable()
export class AccountHidingConsumer implements OnApplicationBootstrap, OnApplicationShutdown {
  private readonly logger = new Logger(AccountHidingConsumer.name);
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
        group: ACCOUNT_HIDING_GROUP,
        topics: [TYPE_AUTH_USER_DELETION_SCHEDULED, TYPE_AUTH_USER_DELETION_CANCELLED],
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
    let hidden: boolean;
    if (env.type === TYPE_AUTH_USER_DELETION_SCHEDULED && env.version === 1) {
      hidden = true;
    } else if (env.type === TYPE_AUTH_USER_DELETION_CANCELLED && env.version === 1) {
      hidden = false;
    } else {
      throw new PermanentError(`unsupported event ${env.type} v${env.version}`);
    }
    const data = env.data as Partial<AuthUserDeletionScheduledV1 & AuthUserDeletionCancelledV1>;
    if (!isUuid(data.user_id)) {
      throw new PermanentError('user_id must be a UUID');
    }
    const changedAt = typeof data.changed_at === 'string' ? new Date(data.changed_at) : null;
    if (changedAt === null || Number.isNaN(changedAt.getTime())) {
      throw new PermanentError('changed_at must be a date-time');
    }
    const userId = data.user_id.toLowerCase();
    await this.prisma.$executeRaw`
      INSERT INTO hidden_accounts (user_id, hidden, changed_at)
      VALUES (${userId}::uuid, ${hidden}, ${changedAt})
      ON CONFLICT (user_id) DO UPDATE SET hidden = EXCLUDED.hidden, changed_at = EXCLUDED.changed_at
      WHERE hidden_accounts.changed_at < EXCLUDED.changed_at`;
    this.logger.log({ msg: hidden ? 'profile hidden' : 'profile shown', user_id: userId });
  }
}
