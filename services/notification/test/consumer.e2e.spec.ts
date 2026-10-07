import { NestExpressApplication } from '@nestjs/platform-express';
import { RedpandaContainer, StartedRedpandaContainer } from '@testcontainers/redpanda';
import { Kafka, Producer, logLevel } from 'kafkajs';
import { Client } from 'pg';

import { uuidv7 } from '../src/common/uuid';
import { CONSUMED_TYPES } from '../src/events/catalog';
import { HEADER_CONSUMER_GROUP, HEADER_ERROR } from '../src/events/event-consumer';
import { createApp } from './support/app';
import { TestDatabase, startDatabase } from './support/postgres';
import { FakePushSender } from './support/push';

/** An envelope as shared-go/events writes it, nanosecond timestamps included. */
function goEnvelope(type: string, subject: string, data: object, id = uuidv7()): string {
  return JSON.stringify({
    id,
    type,
    version: 1,
    source: 'poro-social',
    subject,
    occurred_at: '2026-10-07T10:00:00.123456789Z',
    data,
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

describe('poro-notification-dispatcher consumer', () => {
  let redpanda: StartedRedpandaContainer;
  let db: TestDatabase;
  let pg: Client;
  let producer: Producer;
  let kafka: Kafka;
  let app: NestExpressApplication;
  const push = new FakePushSender();

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
      topics: CONSUMED_TYPES.flatMap((topic) => [topic, `${topic}.dlq`]).map((topic) => ({
        topic,
        numPartitions: 3,
      })),
    });
    await admin.disconnect();
    producer = kafka.producer();
    await producer.connect();

    pg = new Client({ connectionString: db.url });
    await pg.connect();
    app = await createApp(
      { DATABASE_URL: db.url, KAFKA_BROKERS: broker, KAFKA_CONSUMER_ENABLED: 'true' },
      push,
    );
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

  it('turns a profile and a like into an in-app notification and a push', async () => {
    const [actor, owner, video] = [uuidv7(), uuidv7(), uuidv7()];
    await pg.query(
      `INSERT INTO devices (id, user_id, fcm_token, platform, updated_at)
       VALUES ($1, $2, 'token-e2e-0000000000001', 'android', now())`,
      [uuidv7(), owner],
    );
    await producer.send({
      topic: 'poro.user.profile.updated',
      messages: [
        {
          key: actor,
          value: goEnvelope('poro.user.profile.updated', actor, {
            user_id: actor,
            username: 'fatou',
            display_name: 'Fatou',
            avatar_url: null,
            is_creator: false,
            updated_at: '2026-10-07T09:00:00.123456789Z',
          }),
        },
      ],
    });
    await eventually(async () => {
      const { rowCount } = await pg.query('SELECT 1 FROM user_projections WHERE user_id = $1', [
        actor,
      ]);
      return rowCount === 1 ? true : undefined;
    });

    const like = goEnvelope('poro.social.like.created', video, {
      like_id: uuidv7(),
      user_id: actor,
      video_id: video,
      video_owner_id: owner,
      created_at: '2026-10-07T10:00:00.123456789Z',
    });
    await producer.send({
      topic: 'poro.social.like.created',
      messages: [
        { key: video, value: like },
        { key: video, value: like },
      ],
    });

    const body = await eventually(async () => {
      const { rows } = await pg.query<{ body: string }>(
        'SELECT body FROM notifications WHERE user_id = $1',
        [owner],
      );
      return rows.at(0)?.body;
    });
    expect(body).toBe('Fatou a aimé ta vidéo');
    await eventually(() => Promise.resolve(push.sent.length >= 1 ? true : undefined));
    expect(push.sent).toHaveLength(1);
    expect(push.sent[0].tokens).toEqual(['token-e2e-0000000000001']);
  });

  it('dead-letters events it cannot apply', async () => {
    const topic = 'poro.social.follow.created';
    const consumer = kafka.consumer({ groupId: `dlq-reader-${uuidv7()}` });
    await consumer.connect();
    await consumer.subscribe({ topic: `${topic}.dlq`, fromBeginning: true });
    const parked: { error: string; group: string }[] = [];
    await consumer.run({
      eachMessage: ({ message }) => {
        parked.push({
          error: message.headers?.[HEADER_ERROR]?.toString() ?? '',
          group: message.headers?.[HEADER_CONSUMER_GROUP]?.toString() ?? '',
        });
        return Promise.resolve();
      },
    });
    await producer.send({
      topic,
      messages: [
        {
          key: 'poison',
          value: goEnvelope(topic, 'x', { follower_id: 'x', following_id: uuidv7() }),
        },
      ],
    });
    const found = await eventually(() => Promise.resolve(parked.at(0)));
    expect(found).toEqual({
      error: 'follower_id must be a UUID',
      group: 'poro-notification-dispatcher',
    });
    await consumer.disconnect();
  });
});
