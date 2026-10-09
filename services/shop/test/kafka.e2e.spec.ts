import { NestExpressApplication } from '@nestjs/platform-express';
import { RedpandaContainer, StartedRedpandaContainer } from '@testcontainers/redpanda';
import { Kafka, Producer, logLevel } from 'kafkajs';
import { Client } from 'pg';

import { uuidv7 } from '../src/common/uuid';
import { newEnvelope } from '../src/events/envelope';
import { HEADER_ERROR } from '../src/events/event-consumer';
import { ProductsService } from '../src/products/products.service';
import { ShopsService } from '../src/shops/shops.service';
import { STOCK_TOPICS } from '../src/stock/stock.consumer';
import { createApp } from './support/app';
import { outboxEvents } from './support/outbox';
import { TestDatabase, startDatabase } from './support/postgres';

async function eventually<T>(read: () => Promise<T | undefined>, timeoutMs = 60_000): Promise<T> {
  const deadline = Date.now() + timeoutMs;
  for (;;) {
    const value = await read();
    if (value !== undefined) {
      return value;
    }
    if (Date.now() > deadline) {
      throw new Error('condition not met in time');
    }
    await new Promise((resolve) => setTimeout(resolve, 250));
  }
}

describe('order saga over Kafka', () => {
  let redpanda: StartedRedpandaContainer;
  let db: TestDatabase;
  let pg: Client;
  let kafka: Kafka;
  let producer: Producer;
  let app: NestExpressApplication;

  beforeAll(async () => {
    [redpanda, db] = await Promise.all([
      new RedpandaContainer('redpandadata/redpanda:v24.2.7').start(),
      startDatabase(),
    ]);
    const broker = redpanda.getBootstrapServers().replace('localhost', '127.0.0.1');
    kafka = new Kafka({ clientId: 'test', brokers: [broker], logLevel: logLevel.NOTHING });
    const admin = kafka.admin();
    await admin.connect();
    await admin.createTopics({
      topics: STOCK_TOPICS.flatMap((topic) => [topic, `${topic}.dlq`]).map((topic) => ({
        topic,
        numPartitions: 3,
      })),
    });
    await admin.disconnect();
    producer = kafka.producer();
    await producer.connect();
    pg = new Client({ connectionString: db.url });
    await pg.connect();
    app = await createApp({
      DATABASE_URL: db.url,
      JWKS_URL: 'http://127.0.0.1:1/jwks.json',
      KAFKA_BROKERS: broker,
      KAFKA_CONSUMER_ENABLED: 'true',
    });
  });

  afterAll(async () => {
    await (app as NestExpressApplication | undefined)?.close();
    await (producer as Producer | undefined)?.disconnect();
    await (pg as Client | undefined)?.end();
    await Promise.all([
      (redpanda as StartedRedpandaContainer | undefined)?.stop(),
      (db as TestDatabase | undefined)?.container.stop(),
    ]);
  });

  it('reserves on order.created and releases on order.cancelled', async () => {
    const ownerId = uuidv7();
    const shop = await app
      .get(ShopsService)
      .create(ownerId, { name: 'Bazin Dakar', handle: 'bazin_dakar', country_code: 'SN' });
    const product = await app.get(ProductsService).create(ownerId, {
      title: 'Bazin riche',
      variants: [{ title: 'Blanc', price: 25000, stock: 2 }],
    });
    await pg.query(`UPDATE products SET status = 'active' WHERE id = $1`, [product.product_id]);
    const variantId = product.variants[0].variant_id;

    const orderId = uuidv7();
    const buyerId = uuidv7();
    const order = newEnvelope('poro.order.order.created', 1, orderId, {
      order_id: orderId,
      buyer_id: buyerId,
      shop_id: shop.shop_id,
      seller_id: ownerId,
      currency: 'XOF',
      payment_method: 'cash_on_delivery',
      items: [
        { product_id: product.product_id, variant_id: variantId, quantity: 2, unit_price: 25000 },
      ],
      created_at: new Date().toISOString(),
    });
    await producer.send({
      topic: order.type,
      messages: [{ key: orderId, value: JSON.stringify(order) }],
    });

    const reserved = await eventually(
      async () => (await outboxEvents(pg, 'poro.shop.stock.reserved', orderId))[0],
    );
    expect(reserved).toMatchObject({ order_id: orderId, subtotal: 50000, currency: 'XOF' });

    const cancel = newEnvelope('poro.order.order.cancelled', 1, orderId, {
      order_id: orderId,
      buyer_id: buyerId,
      shop_id: shop.shop_id,
      seller_id: ownerId,
      reason: 'payment_timeout',
      paid: false,
      cancelled_at: new Date().toISOString(),
    });
    await producer.send({
      topic: cancel.type,
      messages: [{ key: orderId, value: JSON.stringify(cancel) }],
    });
    const released = await eventually(async () => {
      const { rows } = await pg.query<{ status: string }>(
        'SELECT status FROM reservations WHERE order_id = $1',
        [orderId],
      );
      return rows[0]?.status === 'released' ? rows[0].status : undefined;
    });
    expect(released).toBe('released');
    const { rows } = await pg.query('SELECT stock_reserved FROM variants WHERE id = $1', [
      variantId,
    ]);
    expect(rows).toEqual([{ stock_reserved: 0 }]);
  });

  it('dead-letters events it cannot apply', async () => {
    const topic = 'poro.order.order.created';
    const reader = kafka.consumer({ groupId: `dlq-reader-${uuidv7()}` });
    await reader.connect();
    await reader.subscribe({ topic: `${topic}.dlq`, fromBeginning: true });
    const parked: { key: string; error: string }[] = [];
    await reader.run({
      eachMessage: ({ message }) => {
        parked.push({
          key: message.key?.toString() ?? '',
          error: message.headers?.[HEADER_ERROR]?.toString() ?? '',
        });
        return Promise.resolve();
      },
    });
    const noLines = newEnvelope(topic, 1, uuidv7(), {
      order_id: uuidv7(),
      shop_id: uuidv7(),
      buyer_id: uuidv7(),
      seller_id: uuidv7(),
      currency: 'XOF',
      payment_method: 'wave',
      created_at: new Date().toISOString(),
      items: [],
    });
    await producer.send({
      topic,
      messages: [
        { key: 'poison-json', value: '{not json' },
        { key: 'poison-lines', value: JSON.stringify(noLines) },
      ],
    });
    const found = await eventually(() =>
      Promise.resolve(
        parked.length >= 2 ? [...parked].sort((a, b) => a.key.localeCompare(b.key)) : undefined,
      ),
    );
    expect(found).toEqual([
      { key: 'poison-json', error: expect.stringContaining('invalid event envelope') },
      { key: 'poison-lines', error: 'items must hold 1 to 50 lines' },
    ]);
    await reader.disconnect();
  });
});
