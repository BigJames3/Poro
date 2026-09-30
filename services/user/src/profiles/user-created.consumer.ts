import {
  Inject,
  Injectable,
  Logger,
  OnApplicationBootstrap,
  OnApplicationShutdown,
} from '@nestjs/common';

import { isUuid } from '../common/uuid';
import { appConfig, type AppConfigType } from '../config/app.config';
import { AuthUserCreatedV1, TYPE_AUTH_USER_CREATED } from '../events/catalog';
import { Envelope } from '../events/envelope';
import { KafkaEventConsumer, PermanentError } from '../events/event-consumer';
import { claim } from '../events/outbox';
import { PrismaService } from '../prisma/prisma.service';

export const PROFILES_CONSUMER_GROUP = 'poro-user-profiles';
const COUNTRY_CODE = /^[A-Z]{2}$/;

/** Creates the profile of each new auth account, once per event. */
@Injectable()
export class UserCreatedConsumer implements OnApplicationBootstrap, OnApplicationShutdown {
  private readonly logger = new Logger(UserCreatedConsumer.name);
  private consumer: KafkaEventConsumer | undefined;

  constructor(
    private readonly prisma: PrismaService,
    @Inject(appConfig.KEY) private readonly config: AppConfigType,
  ) {}

  onApplicationBootstrap(): void {
    if (!this.config.kafkaConsumerEnabled) {
      this.logger.warn('kafka consumer disabled by KAFKA_CONSUMER_ENABLED=false');
      return;
    }
    this.consumer = new KafkaEventConsumer(
      this.config.kafkaBrokers,
      { group: PROFILES_CONSUMER_GROUP, topics: [TYPE_AUTH_USER_CREATED] },
      (env) => this.handle(env),
    );
    this.consumer.start();
  }

  async onApplicationShutdown(): Promise<void> {
    await this.consumer?.stop();
  }

  async handle(env: Envelope): Promise<void> {
    if (env.type !== TYPE_AUTH_USER_CREATED || env.version !== 1) {
      throw new PermanentError(`unsupported event ${env.type} v${env.version}`);
    }
    const data = env.data as Partial<AuthUserCreatedV1>;
    if (!isUuid(data.user_id)) {
      throw new PermanentError('user_id must be a UUID');
    }
    const userId = data.user_id.toLowerCase();
    const countryCode =
      typeof data.country_code === 'string' && COUNTRY_CODE.test(data.country_code)
        ? data.country_code
        : null;

    await this.prisma.$transaction(async (tx) => {
      if (!(await claim(tx, PROFILES_CONSUMER_GROUP, env.id))) {
        return;
      }
      // A profile may already exist: GET /me creates it when this event is late.
      await tx.$executeRaw`
        INSERT INTO profiles (user_id, country_code, created_at, updated_at)
        VALUES (${userId}::uuid, ${countryCode}, now(), now())
        ON CONFLICT (user_id) DO UPDATE
          SET country_code = COALESCE(profiles.country_code, EXCLUDED.country_code),
              updated_at = CASE
                WHEN profiles.country_code IS NULL AND EXCLUDED.country_code IS NOT NULL THEN now()
                ELSE profiles.updated_at
              END`;
    });
  }
}
