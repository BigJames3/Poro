import { NestExpressApplication } from '@nestjs/platform-express';
import { RedpandaContainer, StartedRedpandaContainer } from '@testcontainers/redpanda';
import { Kafka, Producer, logLevel } from 'kafkajs';
import { Client } from 'pg';
import request from 'supertest';

import { uuidv7 } from '../src/common/uuid';
import { HEADER_ERROR } from '../src/events/event-consumer';
import { createApp } from './support/app';
import { JwksServer, signToken, signingKey } from './support/jwt';
import { TestDatabase, startDatabase } from './support/postgres';

const TOPIC = 'poro.auth.user.created';

function goEnvelope(userId: string, countryCode: string | null, id = uuidv7()): string {
  return JSON.stringify({
    id,
    type: TOPIC,
    version: 1,
    source: 'poro-auth',
    subject: userId,
    occurred_at: '2026-09-30T10:00:00.123456789Z',
    data: {
      user_id: userId,
      signup_method: 'phone',
      country_code: countryCode,
      language: 'fr',
      created_at: '2026-09-30T10:00:00.123456789Z',
    },
  });
}

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

describe('poro.auth.user.created consumer', () => {
  let redpanda: StartedRedpandaContainer;
  let db: TestDatabase;
  let pg: Client;
  let producer: Producer;
  let kafka: Kafka;
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
      topics: [TOPIC, `${TOPIC}.dlq`].map((topic) => ({ topic, numPartitions: 3 })),
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

  const profile = (userId: string) =>
    pg
      .query<{ country_code: string | null }>(
        'SELECT country_code FROM profiles WHERE user_id = $1',
        [userId],
      )
      .then((r) => r.rows.at(0));

  it('creates profiles once per event, even when redelivered', async () => {
    const userId = uuidv7();
    const eventId = uuidv7();
    const value = goEnvelope(userId, 'CI', eventId);
    await producer.send({
      topic: TOPIC,
      messages: [
        { key: userId, value },
        { key: userId, value },
      ],
    });

    expect(await eventually(() => profile(userId))).toEqual({ country_code: 'CI' });
    const claims = await eventually(async () => {
      const { rows } = await pg.query<{ n: string }>(
        'SELECT count(*) AS n FROM processed_events WHERE event_id = $1',
        [eventId],
      );
      return rows[0].n === '1' ? rows[0].n : undefined;
    });
    expect(claims).toBe('1');
  });

  it('completes a profile created early by GET /me', async () => {
    const userId = uuidv7();
    const me = await request(app.getHttpServer())
      .get('/api/v1/users/me')
      .set('Authorization', `Bearer ${await signToken(key, { sub: userId })}`);
    expect(me.body.data.country_code).toBeNull();

    await producer.send({
      topic: TOPIC,
      messages: [{ key: userId, value: goEnvelope(userId, 'SN') }],
    });
    expect(
      await eventually(async () =>
        (await profile(userId))?.country_code === 'SN' ? 'SN' : undefined,
      ),
    ).toBe('SN');
  });

  it('dead-letters events it cannot apply', async () => {
    const consumer = kafka.consumer({ groupId: `dlq-reader-${uuidv7()}` });
    await consumer.connect();
    await consumer.subscribe({ topic: `${TOPIC}.dlq`, fromBeginning: true });
    const parked: { key: string; error: string }[] = [];
    await consumer.run({
      eachMessage: ({ message }) => {
        parked.push({
          key: message.key?.toString() ?? '',
          error: message.headers?.[HEADER_ERROR]?.toString() ?? '',
        });
        return Promise.resolve();
      },
    });

    const badUser = goEnvelope('not-a-uuid', 'CM');
    await producer.send({
      topic: TOPIC,
      messages: [
        { key: 'poison-json', value: '{not json' },
        { key: 'poison-user', value: badUser },
      ],
    });
    const found = await eventually(() =>
      Promise.resolve(
        parked.length >= 2 ? [...parked].sort((a, b) => a.key.localeCompare(b.key)) : undefined,
      ),
    );
    expect(found).toEqual([
      { key: 'poison-json', error: expect.stringContaining('invalid event envelope') },
      { key: 'poison-user', error: 'user_id must be a UUID' },
    ]);
    await consumer.disconnect();
  });
});
