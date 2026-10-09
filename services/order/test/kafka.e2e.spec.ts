import { NestExpressApplication } from '@nestjs/platform-express';
import { RedpandaContainer, StartedRedpandaContainer } from '@testcontainers/redpanda';
import { Kafka, Producer, logLevel } from 'kafkajs';
import { Client } from 'pg';
import request from 'supertest';

import { CATALOG_TOPICS } from '../src/catalog/catalog.consumer';
import { uuidv7 } from '../src/common/uuid';
import { Envelope, newEnvelope } from '../src/events/envelope';
import { HEADER_ERROR } from '../src/events/event-consumer';
import { SAGA_TOPICS } from '../src/orders/saga.consumer';
import { newListing, productUpdated, shopUpdated } from './support/catalog';
import { createApp } from './support/app';
import { JwksServer, signToken, signingKey } from './support/jwt';
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
  const jwks = new JwksServer();
  const key = signingKey();

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
      topics: [...CATALOG_TOPICS, ...SAGA_TOPICS]
        .flatMap((topic) => [topic, `${topic}.dlq`])
        .map((topic) => ({ topic, numPartitions: 3 })),
    });
    await admin.disconnect();
    producer = kafka.producer();
    await producer.connect();
    pg = new Client({ connectionString: db.url });
    await pg.connect();
    jwks.keys = [key.publicJwk];
    app = await createApp({
      DATABASE_URL: db.url,
      JWKS_URL: await jwks.start(),
      KAFKA_BROKERS: broker,
      KAFKA_CONSUMER_ENABLED: 'true',
      SCHEDULER_ENABLED: 'false',
    });
  });

  afterAll(async () => {
    await (app as NestExpressApplication | undefined)?.close();
    await (producer as Producer | undefined)?.disconnect();
    await (pg as Client | undefined)?.end();
    await jwks.stop();
    await Promise.all([
      (redpanda as StartedRedpandaContainer | undefined)?.stop(),
      (db as TestDatabase | undefined)?.container.stop(),
    ]);
  });

  async function publish(env: Envelope): Promise<void> {
    await producer.send({
      topic: env.type,
      messages: [{ key: env.subject, value: JSON.stringify(env) }],
    });
  }

  it('builds the catalogue, places a cash order once the stock is reserved', async () => {
    const listing = newListing(1);
    await publish(shopUpdated(listing));
    await publish(productUpdated(listing, [12000]));
    const auth = `Bearer ${await signToken(key, { sub: uuidv7() })}`;
    const server = app.getHttpServer();
    await eventually(async () => {
      const res = await request(server)
        .put(`/api/v1/cart/items/${listing.variants[0]}`)
        .set('Authorization', auth)
        .send({ quantity: 3 });
      return res.status === 200 ? res : undefined;
    });

    const recap = await request(server)
      .post('/api/v1/checkout/preview')
      .set('Authorization', auth)
      .send({
        delivery: {
          full_name: 'Moussa',
          phone: '+221770000000',
          city: 'Dakar',
          address: 'Plateau',
        },
      });
    expect(recap.status).toBe(201);
    const checkout = await request(server)
      .post('/api/v1/checkout/confirm')
      .set('Authorization', auth)
      .send({ preview_id: recap.body.data.preview_id as string, confirmed: true });
    expect(checkout.status).toBe(201);
    const orderId = checkout.body.data.orders[0].order_id as string;

    await publish(
      newEnvelope('poro.shop.stock.reserved', 1, orderId, {
        order_id: orderId,
        shop_id: listing.shopId,
        currency: 'XOF',
        items: [
          {
            product_id: listing.productId,
            variant_id: listing.variants[0],
            quantity: 3,
            unit_price: 12000,
            title: 'Pagne wax — V0',
          },
        ],
        subtotal: 36000,
        reserved_at: new Date().toISOString(),
      }),
    );
    const placed = await eventually(
      async () => (await outboxEvents(pg, 'poro.order.order.placed', orderId))[0],
    );
    expect(placed).toMatchObject({
      total: 36000,
      payment_method: 'cash_on_delivery',
      expires_at: null,
    });
  });

  it('dead-letters events it cannot apply', async () => {
    const topic = 'poro.shop.stock.reserved';
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
    await producer.send({
      topic,
      messages: [
        { key: 'poison-json', value: '{not json' },
        {
          key: 'poison-order',
          value: JSON.stringify(newEnvelope(topic, 1, uuidv7(), { order_id: 'x' })),
        },
      ],
    });
    const found = await eventually(() =>
      Promise.resolve(
        parked.length >= 2 ? [...parked].sort((a, b) => a.key.localeCompare(b.key)) : undefined,
      ),
    );
    expect(found).toEqual([
      { key: 'poison-json', error: expect.stringContaining('invalid event envelope') },
      { key: 'poison-order', error: 'order_id must be a UUID' },
    ]);
    await reader.disconnect();
  });
});
