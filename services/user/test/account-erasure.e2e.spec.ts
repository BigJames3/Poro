import { DeleteObjectCommand, ListObjectsV2Command, NoSuchKey, S3Client } from '@aws-sdk/client-s3';
import { NestExpressApplication } from '@nestjs/platform-express';
import { mockClient } from 'aws-sdk-client-mock';
import { Client } from 'pg';
import request from 'supertest';
import type { App } from 'supertest/types';

import { uuidv7 } from '../src/common/uuid';
import { Envelope } from '../src/events/envelope';
import { PermanentError } from '../src/events/event-consumer';
import {
  ACCOUNT_ERASURE_GROUP,
  AccountErasureConsumer,
} from '../src/profiles/account-erasure.consumer';
import { UserCreatedConsumer } from '../src/profiles/user-created.consumer';
import { createApp } from './support/app';
import { JwksServer, SigningKey, signToken, signingKey } from './support/jwt';
import { TestDatabase, startDatabase } from './support/postgres';

const s3 = mockClient(S3Client);
const DELETED_AT = '2026-10-15T08:00:00.123456789Z';

function deleted(userId: unknown, id = uuidv7(), deletedAt: unknown = DELETED_AT): Envelope {
  return {
    id,
    type: 'poro.auth.user.deleted',
    version: 1,
    source: 'poro-auth',
    subject: String(userId),
    occurred_at: DELETED_AT,
    data: { user_id: userId, deleted_at: deletedAt },
  };
}

function created(userId: string): Envelope {
  return {
    id: uuidv7(),
    type: 'poro.auth.user.created',
    version: 1,
    source: 'poro-auth',
    subject: userId,
    occurred_at: DELETED_AT,
    data: {
      user_id: userId,
      signup_method: 'phone',
      country_code: 'CI',
      language: 'fr',
      created_at: DELETED_AT,
    },
  };
}

interface ProfileRow {
  user_id: string;
  username: string | null;
  display_name: string | null;
  bio: string | null;
  avatar_key: string | null;
  country_code: string | null;
  is_creator: boolean;
  creator_since: Date | null;
  deleted_at: Date | null;
}

describe('account erasure (poro.auth.user.deleted)', () => {
  let db: TestDatabase;
  let pg: Client;
  let jwks: JwksServer;
  let key: SigningKey;
  let app: NestExpressApplication;
  let server: App;
  let erasure: AccountErasureConsumer;

  beforeAll(async () => {
    db = await startDatabase();
    pg = new Client({ connectionString: db.url });
    await pg.connect();
    key = signingKey();
    jwks = new JwksServer();
    jwks.keys = [key.publicJwk];
    app = await createApp({
      DATABASE_URL: db.url,
      JWKS_URL: await jwks.start(),
      AVATAR_PUBLIC_BASE_URL: 'https://cdn.poro.test/avatars',
    });
    server = app.getHttpServer();
    erasure = app.get(AccountErasureConsumer);
  });

  afterAll(async () => {
    await (app as NestExpressApplication | undefined)?.close();
    await (pg as Client | undefined)?.end();
    await jwks.stop();
    await (db as TestDatabase | undefined)?.container.stop();
  });

  beforeEach(() => {
    s3.reset();
    s3.on(ListObjectsV2Command).resolves({});
    s3.on(DeleteObjectCommand).resolves({});
  });

  async function profileOf(userId: string): Promise<ProfileRow | undefined> {
    const { rows } = await pg.query<ProfileRow>(
      `SELECT user_id, username, display_name, bio, avatar_key, country_code, is_creator,
              creator_since, deleted_at
         FROM profiles WHERE user_id = $1`,
      [userId],
    );
    return rows.at(0);
  }

  async function claims(eventId: string): Promise<number> {
    const { rows } = await pg.query<{ n: string }>(
      'SELECT count(*) AS n FROM processed_events WHERE consumer = $1 AND event_id = $2',
      [ACCOUNT_ERASURE_GROUP, eventId],
    );
    return Number(rows[0].n);
  }

  async function outboxCount(userId: string): Promise<number> {
    const { rows } = await pg.query<{ n: string }>(
      'SELECT count(*) AS n FROM outbox_events WHERE event_key = $1',
      [userId],
    );
    return Number(rows[0].n);
  }

  /** A full creator profile with an avatar, set up through the API. */
  async function creator(username: string): Promise<{ userId: string; auth: string }> {
    const userId = uuidv7();
    const auth = `Bearer ${await signToken(key, { sub: userId })}`;
    await app.get(UserCreatedConsumer).handle(created(userId));
    await request(server)
      .patch('/api/v1/users/me')
      .set('Authorization', auth)
      .send({ username, display_name: 'Awa Koné', bio: 'Danseuse' })
      .expect(200);
    await request(server).post('/api/v1/users/me/creator').set('Authorization', auth).expect(200);
    await pg.query('UPDATE profiles SET avatar_key = $2 WHERE user_id = $1', [
      userId,
      `avatars/${userId}/${uuidv7()}.webp`,
    ]);
    return { userId, auth };
  }

  it('anonymizes the profile, deletes its objects and frees the username', async () => {
    const awa = await creator('awa.erase');
    const other = await creator('other.keeps');
    const otherBefore = await profileOf(other.userId);
    const avatarKey = `avatars/${awa.userId}/${uuidv7()}.webp`;
    const orphanKey = `avatars/${awa.userId}/${uuidv7()}.webp`;
    const uploadKey = `avatars/uploads/${awa.userId}/${uuidv7()}`;
    s3.on(ListObjectsV2Command, { Bucket: 'poro-avatars' })
      .resolvesOnce({
        Contents: [{ Key: avatarKey }],
        IsTruncated: true,
        NextContinuationToken: 'page-2',
      })
      .resolvesOnce({ Contents: [{ Key: orphanKey }] });
    s3.on(ListObjectsV2Command, { Bucket: 'poro-avatar-uploads' }).resolves({
      Contents: [{ Key: uploadKey }],
    });
    s3.on(DeleteObjectCommand, { Key: orphanKey }).rejects(
      new NoSuchKey({ message: 'gone', $metadata: {} }),
    );
    const outboxBefore = await outboxCount(awa.userId);
    const env = deleted(awa.userId.toUpperCase());

    await erasure.handle(env);

    expect(await profileOf(awa.userId)).toEqual({
      user_id: awa.userId,
      username: null,
      display_name: null,
      bio: null,
      avatar_key: null,
      country_code: null,
      is_creator: false,
      creator_since: null,
      deleted_at: new Date('2026-10-15T08:00:00.123Z'),
    });
    expect(await profileOf(other.userId)).toEqual(otherBefore);
    expect(s3.commandCalls(ListObjectsV2Command).map((c) => c.args[0].input)).toEqual([
      { Bucket: 'poro-avatars', Prefix: `avatars/${awa.userId}/`, ContinuationToken: undefined },
      { Bucket: 'poro-avatars', Prefix: `avatars/${awa.userId}/`, ContinuationToken: 'page-2' },
      {
        Bucket: 'poro-avatar-uploads',
        Prefix: `avatars/uploads/${awa.userId}/`,
        ContinuationToken: undefined,
      },
    ]);
    expect(s3.commandCalls(DeleteObjectCommand).map((c) => c.args[0].input)).toEqual([
      { Bucket: 'poro-avatars', Key: avatarKey },
      { Bucket: 'poro-avatars', Key: orphanKey },
      { Bucket: 'poro-avatar-uploads', Key: uploadKey },
    ]);
    expect(await claims(env.id)).toBe(1);
    // Nothing is published: a snapshot would recreate the projections other services drop.
    expect(await outboxCount(awa.userId)).toBe(outboxBefore);

    expect((await request(server).get('/api/v1/users/by-username/awa.erase')).status).toBe(404);
    expect(
      (await request(server).get('/api/v1/users/me').set('Authorization', awa.auth)).status,
    ).toBe(404);
    expect(
      (
        await request(server)
          .patch('/api/v1/users/me')
          .set('Authorization', awa.auth)
          .send({ display_name: 'Back' })
      ).status,
    ).toBe(404);
    expect(
      (
        await request(server)
          .get('/api/v1/users/username-availability')
          .query({ username: 'awa.erase' })
          .set('Authorization', other.auth)
      ).body.data,
    ).toEqual({ username: 'awa.erase', available: true, reason: null });
    expect((await request(server).get('/api/v1/users/by-username/other.keeps')).status).toBe(200);
  });

  it('is a no-op when the same event is replayed', async () => {
    const { userId } = await creator('replay.me');
    const env = deleted(userId);
    await erasure.handle(env);
    const after = await profileOf(userId);
    s3.resetHistory();

    await erasure.handle(env);

    expect(await profileOf(userId)).toEqual(after);
    expect(s3.commandCalls(ListObjectsV2Command)).toHaveLength(0);
    expect(await claims(env.id)).toBe(1);
  });

  it('retries a storage failure without claiming, and tolerates a second erasure', async () => {
    const { userId } = await creator('flaky.storage');
    const avatarKey = `avatars/${userId}/${uuidv7()}.webp`;
    s3.on(ListObjectsV2Command, { Bucket: 'poro-avatars' }).resolves({
      Contents: [{ Key: avatarKey }],
    });
    s3.on(DeleteObjectCommand, { Key: avatarKey })
      .rejectsOnce(new Error('storage unavailable'))
      .resolves({});
    const env = deleted(userId);

    await expect(erasure.handle(env)).rejects.toThrow('storage unavailable');
    // The profile is already anonymized; the event stays unclaimed for the retry.
    expect(await profileOf(userId)).toMatchObject({ username: null, avatar_key: null });
    expect(await claims(env.id)).toBe(0);

    await erasure.handle(env);
    expect(s3.commandCalls(DeleteObjectCommand, { Key: avatarKey })).toHaveLength(2);
    expect(await claims(env.id)).toBe(1);

    const first = await profileOf(userId);
    const again = deleted(userId, uuidv7(), '2026-11-01T00:00:00Z');
    await erasure.handle(again);
    expect(await profileOf(userId)).toEqual(first);
    expect(await claims(again.id)).toBe(1);
  });

  it('keeps an erased profile erased when poro.auth.user.created comes late or again', async () => {
    const known = await creator('late.created');
    await erasure.handle(deleted(known.userId));
    const unknown = uuidv7();
    await erasure.handle(deleted(unknown));
    expect(await profileOf(unknown)).toMatchObject({
      username: null,
      deleted_at: new Date('2026-10-15T08:00:00.123Z'),
    });

    for (const userId of [known.userId, unknown]) {
      const before = await profileOf(userId);
      await app.get(UserCreatedConsumer).handle(created(userId));
      expect(await profileOf(userId)).toEqual(before);
      expect((await profileOf(userId))?.country_code).toBeNull();
    }
  });

  it('rejects malformed events before touching the database', async () => {
    const userId = uuidv7();
    for (const [env, message] of [
      [{ ...deleted(userId), version: 2 }, 'unsupported event poro.auth.user.deleted v2'],
      [{ ...deleted(userId), type: 'poro.auth.user.created' }, 'unsupported event'],
      [deleted('not-a-uuid'), 'user_id must be a UUID'],
      [deleted(userId, uuidv7(), 'yesterday'), 'deleted_at must be a date-time'],
      [deleted(userId, uuidv7(), null), 'deleted_at must be a date-time'],
    ] as const) {
      const err = await erasure.handle(env).catch((e: unknown) => e);
      expect(err).toBeInstanceOf(PermanentError);
      expect((err as Error).message).toContain(message);
    }
    expect(await profileOf(userId)).toBeUndefined();
  });
});
