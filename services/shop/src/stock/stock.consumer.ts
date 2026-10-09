import {
  Inject,
  Injectable,
  Logger,
  OnApplicationBootstrap,
  OnApplicationShutdown,
} from '@nestjs/common';

import { isUuid } from '../common/uuid';
import { appConfig, type AppConfigType } from '../config/app.config';
import {
  TYPE_ORDER_CANCELLED,
  TYPE_ORDER_COMPLETED,
  TYPE_ORDER_CREATED,
  type OrderCancelledV1,
  type OrderCompletedV1,
  type OrderCreatedV1,
  type OrderLineV1,
} from '../events/catalog';
import { Envelope } from '../events/envelope';
import { KafkaEventConsumer, PermanentError } from '../events/event-consumer';
import { STOCK_CONSUMER_GROUP, StockService } from './stock.service';

export const STOCK_TOPICS = [TYPE_ORDER_CREATED, TYPE_ORDER_CANCELLED, TYPE_ORDER_COMPLETED];
const MAX_LINES = 50;
const MAX_QUANTITY = 99;

function uuidField(data: Record<string, unknown>, field: string): string {
  const value = data[field];
  if (!isUuid(value)) {
    throw new PermanentError(`${field} must be a UUID`);
  }
  return value.toLowerCase();
}

function stringField(data: Record<string, unknown>, field: string): string {
  const value = data[field];
  if (typeof value !== 'string' || value === '') {
    throw new PermanentError(`${field} is required`);
  }
  return value;
}

function lines(raw: unknown): OrderLineV1[] {
  if (!Array.isArray(raw) || raw.length === 0 || raw.length > MAX_LINES) {
    throw new PermanentError(`items must hold 1 to ${MAX_LINES} lines`);
  }
  return raw.map((item: unknown) => {
    if (typeof item !== 'object' || item === null) {
      throw new PermanentError('items must be objects');
    }
    const line = item as Record<string, unknown>;
    const quantity = line.quantity;
    if (
      typeof quantity !== 'number' ||
      !Number.isInteger(quantity) ||
      quantity < 1 ||
      quantity > MAX_QUANTITY
    ) {
      throw new PermanentError(`quantity must be 1 to ${MAX_QUANTITY}`);
    }
    return {
      product_id: uuidField(line, 'product_id'),
      variant_id: uuidField(line, 'variant_id'),
      quantity,
      unit_price: typeof line.unit_price === 'number' ? line.unit_price : 0,
    };
  });
}

/** Applies the order saga to stock, once per event. Malformed events are dead-lettered. */
@Injectable()
export class StockConsumer implements OnApplicationBootstrap, OnApplicationShutdown {
  private readonly logger = new Logger(StockConsumer.name);
  private consumer: KafkaEventConsumer | undefined;

  constructor(
    private readonly stock: StockService,
    @Inject(appConfig.KEY) private readonly config: AppConfigType,
  ) {}

  onApplicationBootstrap(): void {
    if (!this.config.kafkaConsumerEnabled) {
      this.logger.warn('kafka consumer disabled by KAFKA_CONSUMER_ENABLED=false');
      return;
    }
    this.consumer = new KafkaEventConsumer(
      this.config.kafkaBrokers,
      { group: STOCK_CONSUMER_GROUP, topics: STOCK_TOPICS },
      (env) => this.handle(env),
    );
    this.consumer.start();
  }

  async onApplicationShutdown(): Promise<void> {
    await this.consumer?.stop();
  }

  async handle(env: Envelope): Promise<void> {
    if (!STOCK_TOPICS.includes(env.type) || env.version !== 1) {
      throw new PermanentError(`unsupported event ${env.type} v${env.version}`);
    }
    const data = env.data as Record<string, unknown>;
    const orderId = uuidField(data, 'order_id');
    const shopId = uuidField(data, 'shop_id');
    if (env.type === TYPE_ORDER_CREATED) {
      const order: OrderCreatedV1 = {
        order_id: orderId,
        buyer_id: uuidField(data, 'buyer_id'),
        shop_id: shopId,
        seller_id: uuidField(data, 'seller_id'),
        currency: stringField(data, 'currency'),
        payment_method: stringField(data, 'payment_method'),
        items: lines(data.items),
        created_at: stringField(data, 'created_at'),
      };
      return this.stock.reserve(env.id, order);
    }
    if (env.type === TYPE_ORDER_CANCELLED) {
      return this.stock.release(env.id, {
        ...(data as unknown as OrderCancelledV1),
        order_id: orderId,
        shop_id: shopId,
        reason: stringField(data, 'reason'),
      });
    }
    return this.stock.consume(env.id, {
      ...(data as unknown as OrderCompletedV1),
      order_id: orderId,
      shop_id: shopId,
    });
  }
}
