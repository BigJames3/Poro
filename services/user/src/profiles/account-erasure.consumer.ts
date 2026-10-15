import {
  Inject,
  Injectable,
  Logger,
  OnApplicationBootstrap,
  OnApplicationShutdown,
} from '@nestjs/common';

import { AvatarService } from '../avatars/avatar.service';
import { isUuid } from '../common/uuid';
import { appConfig, type AppConfigType } from '../config/app.config';
import { AuthUserDeletedV1, TYPE_AUTH_USER_DELETED } from '../events/catalog';
import { Envelope } from '../events/envelope';
import { KafkaEventConsumer, PermanentError } from '../events/event-consumer';
import { claim } from '../events/outbox';
import { PrismaService } from '../prisma/prisma.service';

export const ACCOUNT_ERASURE_GROUP = 'poro-user-account-erasure';

/**
 * Anonymizes the profile of an erased auth account (ADR-0018). The row stays
 * with its id and deleted_at so reads answer 404 and a replayed
 * poro.auth.user.created cannot recreate it; the username becomes free again.
 *
 * No poro.user.profile.updated is published: every service consumes
 * poro.auth.user.deleted itself, and a snapshot could recreate the projections
 * they have just dropped.
 *
 * Order, as in the video moderation handler: the profile is anonymized and
 * committed first (idempotent), then the avatar objects are deleted, then the
 * inbox is claimed. A storage failure leaves the event unclaimed, so the retry
 * finds the objects again by their prefix and deletes them.
 */
@Injectable()
export class AccountErasureConsumer implements OnApplicationBootstrap, OnApplicationShutdown {
  private readonly logger = new Logger(AccountErasureConsumer.name);
  private consumer: KafkaEventConsumer | undefined;

  constructor(
    private readonly prisma: PrismaService,
    private readonly avatars: AvatarService,
    @Inject(appConfig.KEY) private readonly config: AppConfigType,
  ) {}

  onApplicationBootstrap(): void {
    if (!this.config.kafkaConsumerEnabled) {
      return;
    }
    this.consumer = new KafkaEventConsumer(
      this.config.kafkaBrokers,
      { group: ACCOUNT_ERASURE_GROUP, topics: [TYPE_AUTH_USER_DELETED] },
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
    const deletedAt = typeof data.deleted_at === 'string' ? new Date(data.deleted_at) : null;
    if (deletedAt === null || Number.isNaN(deletedAt.getTime())) {
      throw new PermanentError('deleted_at must be a date-time');
    }
    const userId = data.user_id.toLowerCase();

    const alreadyApplied = await this.prisma.$transaction(async (tx) => {
      const seen = await tx.processedEvent.findUnique({
        where: { consumer_eventId: { consumer: ACCOUNT_ERASURE_GROUP, eventId: env.id } },
      });
      if (seen) {
        return true;
      }
      // The profile may not exist yet when poro.auth.user.created is late:
      // the tombstone row then keeps that event from creating it.
      await tx.$executeRaw`
        INSERT INTO profiles (user_id, deleted_at, created_at, updated_at)
        VALUES (${userId}::uuid, ${deletedAt}, now(), now())
        ON CONFLICT (user_id) DO UPDATE
          SET username = NULL,
              display_name = NULL,
              bio = NULL,
              avatar_key = NULL,
              country_code = NULL,
              is_creator = false,
              creator_since = NULL,
              deleted_at = COALESCE(profiles.deleted_at, EXCLUDED.deleted_at),
              updated_at = now()`;
      return false;
    });
    if (alreadyApplied) {
      return;
    }

    const removed = await this.avatars.removeAllOf(userId);
    await this.prisma.$transaction(async (tx) => {
      await claim(tx, ACCOUNT_ERASURE_GROUP, env.id);
    });
    this.logger.log({ msg: 'profile erased', user_id: userId, avatar_objects: removed });
  }
}
