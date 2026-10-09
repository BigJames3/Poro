import {
  Inject,
  Injectable,
  Logger,
  OnApplicationBootstrap,
  OnApplicationShutdown,
} from '@nestjs/common';

import { appConfig, type AppConfigType } from '../config/app.config';
import {
  TYPE_PAYMENT_FAILED,
  TYPE_PAYMENT_SUCCEEDED,
  TYPE_REFUND_SUCCEEDED,
  TYPE_STOCK_REJECTED,
  TYPE_STOCK_RESERVED,
} from '../events/catalog';
import { Envelope } from '../events/envelope';
import { KafkaEventConsumer, PermanentError } from '../events/event-consumer';
import { Data } from '../events/fields';
import { SAGA_CONSUMER_GROUP, SagaService } from './saga.service';

export const SAGA_TOPICS = [
  TYPE_STOCK_RESERVED,
  TYPE_STOCK_REJECTED,
  TYPE_PAYMENT_SUCCEEDED,
  TYPE_PAYMENT_FAILED,
  TYPE_REFUND_SUCCEEDED,
];

/** Feeds the answers of shop and payment to the saga, once per event. */
@Injectable()
export class SagaConsumer implements OnApplicationBootstrap, OnApplicationShutdown {
  private readonly logger = new Logger(SagaConsumer.name);
  private consumer: KafkaEventConsumer | undefined;

  constructor(
    private readonly saga: SagaService,
    @Inject(appConfig.KEY) private readonly config: AppConfigType,
  ) {}

  onApplicationBootstrap(): void {
    if (!this.config.kafkaConsumerEnabled) {
      this.logger.warn('kafka consumer disabled by KAFKA_CONSUMER_ENABLED=false');
      return;
    }
    this.consumer = new KafkaEventConsumer(
      this.config.kafkaBrokers,
      { group: SAGA_CONSUMER_GROUP, topics: SAGA_TOPICS },
      (env) => this.handle(env),
    );
    this.consumer.start();
  }

  async onApplicationShutdown(): Promise<void> {
    await this.consumer?.stop();
  }

  async handle(env: Envelope): Promise<void> {
    if (env.version !== 1) {
      throw new PermanentError(`unsupported event ${env.type} v${env.version}`);
    }
    const data = env.data as Data;
    switch (env.type) {
      case TYPE_STOCK_RESERVED:
        await this.saga.stockReserved(env.id, data);
        return;
      case TYPE_STOCK_REJECTED:
        await this.saga.stockRejected(env.id, data);
        return;
      case TYPE_PAYMENT_SUCCEEDED:
        await this.saga.paymentSucceeded(env.id, data);
        return;
      case TYPE_PAYMENT_FAILED:
        await this.saga.paymentFailed(env.id, data);
        return;
      case TYPE_REFUND_SUCCEEDED:
        await this.saga.refundSucceeded(env.id, data);
        return;
      default:
        throw new PermanentError(`unsupported event ${env.type} v${env.version}`);
    }
  }
}
