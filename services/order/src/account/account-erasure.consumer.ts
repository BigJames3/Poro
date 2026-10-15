import {
  Inject,
  Injectable,
  Logger,
  OnApplicationBootstrap,
  OnApplicationShutdown,
} from '@nestjs/common';

import { appConfig, type AppConfigType } from '../config/app.config';
import { TYPE_AUTH_USER_DELETED } from '../events/catalog';
import { Envelope } from '../events/envelope';
import { KafkaEventConsumer, PermanentError } from '../events/event-consumer';
import { Data, uuidField } from '../events/fields';
import { ACCOUNT_ERASURE_GROUP, AccountErasureService } from './account-erasure.service';

export const ACCOUNT_ERASURE_TOPICS = [TYPE_AUTH_USER_DELETED];

/** Erases the order data of each deleted account, once per event. */
@Injectable()
export class AccountErasureConsumer implements OnApplicationBootstrap, OnApplicationShutdown {
  private readonly logger = new Logger(AccountErasureConsumer.name);
  private consumer: KafkaEventConsumer | undefined;

  constructor(
    private readonly erasure: AccountErasureService,
    @Inject(appConfig.KEY) private readonly config: AppConfigType,
  ) {}

  onApplicationBootstrap(): void {
    if (!this.config.kafkaConsumerEnabled) {
      this.logger.warn('kafka consumer disabled by KAFKA_CONSUMER_ENABLED=false');
      return;
    }
    this.consumer = new KafkaEventConsumer(
      this.config.kafkaBrokers,
      { group: ACCOUNT_ERASURE_GROUP, topics: ACCOUNT_ERASURE_TOPICS },
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
    await this.erasure.erase(env.id, uuidField(env.data as Data, 'user_id'));
  }
}
