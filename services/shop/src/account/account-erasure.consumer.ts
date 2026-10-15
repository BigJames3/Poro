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
import { TYPE_AUTH_USER_DELETED } from '../events/catalog';
import { Envelope } from '../events/envelope';
import { KafkaEventConsumer, PermanentError } from '../events/event-consumer';
import { ACCOUNT_ERASURE_GROUP, AccountErasureService } from './account-erasure.service';
import { MetricsService } from '../metrics/metrics';

export const ACCOUNT_ERASURE_TOPICS = [TYPE_AUTH_USER_DELETED];

/** Erases the shop of each deleted account, once per event. Malformed events are dead-lettered. */
@Injectable()
export class AccountErasureConsumer implements OnApplicationBootstrap, OnApplicationShutdown {
  private readonly logger = new Logger(AccountErasureConsumer.name);
  private consumer: KafkaEventConsumer | undefined;

  constructor(
    private readonly erasure: AccountErasureService,
    @Inject(appConfig.KEY) private readonly config: AppConfigType,
    @Optional() private readonly metrics?: MetricsService,
  ) {}

  onApplicationBootstrap(): void {
    if (!this.config.kafkaConsumerEnabled) {
      this.logger.warn('kafka consumer disabled by KAFKA_CONSUMER_ENABLED=false');
      return;
    }
    this.consumer = new KafkaEventConsumer(
      this.config.kafkaBrokers,
      {
        group: ACCOUNT_ERASURE_GROUP,
        topics: ACCOUNT_ERASURE_TOPICS,
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
    const userId = (env.data as Record<string, unknown>).user_id;
    if (!isUuid(userId)) {
      throw new PermanentError('user_id must be a UUID');
    }
    await this.erasure.erase(env.id, userId.toLowerCase());
  }
}
