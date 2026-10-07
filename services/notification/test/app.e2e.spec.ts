import { NestExpressApplication } from '@nestjs/platform-express';
import { Client } from 'pg';
import request from 'supertest';
import type { App } from 'supertest/types';

import { uuidv7 } from '../src/common/uuid';
import { newEnvelope } from '../src/events/envelope';
import { NotificationDispatcher } from '../src/notifications/dispatcher';
import { MAX_DEVICES_PER_USER } from '../src/notifications/notifications.service';
import { createApp } from './support/app';
import { JwksServer, SigningKey, signToken, signingKey } from './support/jwt';
import { TestDatabase, startDatabase } from './support/postgres';
import { FakePushSender } from './support/push';

/** The items of a list response. */
function items<T>(res: { body: { data: { items: T[] } } }): T[] {
  return res.body.data.items;
}

const TOKEN = 'fcm-token-aaaaaaaaaaaaaaaaaaaa:APA91b_x-1';

describe('notification service HTTP API', () => {
  let db: TestDatabase;
  let pg: Client;
  let jwks: JwksServer;
  let key: SigningKey;
  let app: NestExpressApplication;
  let server: App;
  let dispatcher: NotificationDispatcher;

  beforeAll(async () => {
    db = await startDatabase();
    pg = new Client({ connectionString: db.url });
    await pg.connect();
    key = signingKey();
    jwks = new JwksServer();
    jwks.keys = [key.publicJwk];
    app = await createApp(
      { DATABASE_URL: db.url, JWKS_URL: await jwks.start() },
      new FakePushSender(),
    );
    server = app.getHttpServer();
    dispatcher = app.get(NotificationDispatcher);
  });

  afterAll(async () => {
    await (app as NestExpressApplication | undefined)?.close();
    await (pg as Client | undefined)?.end();
    await jwks.stop();
    await (db as TestDatabase | undefined)?.container.stop();
  });

  async function as(userId: string = uuidv7()): Promise<{ userId: string; auth: string }> {
    return { userId, auth: `Bearer ${await signToken(key, { sub: userId })}` };
  }

  async function follow(follower: string, following: string): Promise<void> {
    await dispatcher.handle(
      newEnvelope('poro.social.follow.created', 1, follower, {
        follower_id: follower,
        following_id: following,
        created_at: new Date().toISOString(),
      }),
    );
  }

  it('requires a valid access token', async () => {
    for (const [method, path] of [
      ['get', '/api/v1/notifications'],
      ['get', '/api/v1/notifications/unread-count'],
      ['put', '/api/v1/notifications/devices'],
      ['patch', '/api/v1/notifications/preferences'],
    ] as const) {
      const res = await request(server)[method](path).set('Authorization', 'Bearer nope');
      expect(res.status).toBe(401);
      expect(res.body.error).toEqual({ code: 'unauthorized', message: 'unauthorized' });
    }
  });

  it('serves health without a token, under /health and /api/v1/health', async () => {
    for (const path of ['/health', '/health/live', '/health/ready', '/api/v1/health']) {
      const res = await request(server).get(path);
      expect(res.status).toBe(200);
      expect(res.body).toMatchObject({ status: 'ok', service: 'notification' });
    }
    const metrics = await request(server).get('/metrics');
    expect(metrics.status).toBe(200);
    expect(metrics.text).toContain('notifications_created_total');
  });

  it('lists notifications newest first with actors and a cursor', async () => {
    const me = await as();
    const followers = [uuidv7(), uuidv7(), uuidv7()];
    await dispatcher.handle(
      newEnvelope('poro.user.profile.updated', 1, followers[2], {
        user_id: followers[2],
        username: 'awa',
        display_name: 'Awa',
        avatar_url: 'https://cdn.poro.test/awa.jpg',
        is_creator: false,
        updated_at: new Date().toISOString(),
      }),
    );
    for (const follower of followers) {
      await follow(follower, me.userId);
    }

    const first = await request(server)
      .get('/api/v1/notifications?limit=2')
      .set('Authorization', me.auth);
    expect(first.status).toBe(200);
    expect(first.body.data.items).toHaveLength(2);
    expect(first.body.data.items[0]).toEqual({
      id: expect.any(String),
      type: 'follow',
      title: 'Nouvel abonné',
      body: 'Awa a commencé à te suivre',
      data: { user_id: followers[2] },
      actor: {
        id: followers[2],
        username: 'awa',
        display_name: 'Awa',
        avatar_url: 'https://cdn.poro.test/awa.jpg',
      },
      actor_count: 1,
      entity_type: 'user',
      entity_id: followers[2],
      read: false,
      read_at: null,
      created_at: expect.any(String),
      last_activity_at: expect.any(String),
    });
    expect(first.body.data.items[1].actor).toEqual({
      id: followers[1],
      username: null,
      display_name: null,
      avatar_url: null,
    });

    const second = await request(server)
      .get(`/api/v1/notifications?limit=2&cursor=${first.body.data.next_cursor as string}`)
      .set('Authorization', me.auth);
    expect(items<{ entity_id: string }>(second).map((item) => item.entity_id)).toEqual([
      followers[0],
    ]);
    expect(second.body.data.next_cursor).toBeNull();

    const other = await as();
    const theirs = await request(server)
      .get('/api/v1/notifications')
      .set('Authorization', other.auth);
    expect(theirs.body.data).toEqual({ items: [], next_cursor: null });
  });

  it('validates list parameters', async () => {
    const me = await as();
    for (const [query, code] of [
      ['cursor=bad', 'invalid_cursor'],
      [`cursor=${Buffer.from(`x|${uuidv7()}`).toString('base64url')}`, 'invalid_cursor'],
      ['limit=0', 'invalid_request'],
      ['limit=51', 'invalid_request'],
      ['limit=abc', 'invalid_request'],
      ['unknown=1', 'invalid_request'],
    ]) {
      const res = await request(server)
        .get(`/api/v1/notifications?${query}`)
        .set('Authorization', me.auth);
      expect(res.status).toBe(400);
      expect(res.body.error.code).toBe(code);
    }
  });

  it('counts unread notifications and marks them read', async () => {
    const me = await as();
    await follow(uuidv7(), me.userId);
    await follow(uuidv7(), me.userId);
    await follow(uuidv7(), me.userId);
    const count = () =>
      request(server)
        .get('/api/v1/notifications/unread-count')
        .set('Authorization', me.auth)
        .then((res) => res.body.data as { count: number });
    expect(await count()).toEqual({ count: 3 });

    const list = await request(server).get('/api/v1/notifications').set('Authorization', me.auth);
    const id = list.body.data.items[0].id as string;
    const read = await request(server)
      .patch(`/api/v1/notifications/${id}/read`)
      .set('Authorization', me.auth);
    expect(read.status).toBe(200);
    expect(read.body.data).toEqual({ id, read_at: expect.any(String) });
    const again = await request(server)
      .patch(`/api/v1/notifications/${id}/read`)
      .set('Authorization', me.auth);
    expect(again.body.data.read_at).toBe(read.body.data.read_at);
    expect(await count()).toEqual({ count: 2 });

    const stranger = await as();
    for (const target of [id, uuidv7(), 'not-a-uuid']) {
      const res = await request(server)
        .patch(`/api/v1/notifications/${target}/read`)
        .set('Authorization', stranger.auth);
      expect(res.status).toBe(404);
      expect(res.body.error.code).toBe('notification_not_found');
    }

    const all = await request(server)
      .post('/api/v1/notifications/read-all')
      .set('Authorization', me.auth);
    expect(all.status).toBe(200);
    expect(all.body.data).toEqual({ updated: 2 });
    expect(await count()).toEqual({ count: 0 });

    // Reading does not reorder the list.
    const after = await request(server).get('/api/v1/notifications').set('Authorization', me.auth);
    expect(items<{ id: string }>(after).map((item) => item.id)).toEqual(
      items<{ id: string }>(list).map((item) => item.id),
    );
  });

  it('registers, moves and removes devices', async () => {
    const me = await as();
    const put = (auth: string, body: object) =>
      request(server).put('/api/v1/notifications/devices').set('Authorization', auth).send(body);

    const created = await put(me.auth, { token: TOKEN, platform: 'android' });
    expect(created.status).toBe(200);
    expect(created.body.data).toEqual({
      token: TOKEN,
      platform: 'android',
      created_at: expect.any(String),
      updated_at: expect.any(String),
    });

    // The phone changed hands: the token now belongs to the new account only.
    const next = await as();
    await put(next.auth, { token: TOKEN, platform: 'android' });
    const owners = await pg.query<{ user_id: string }>(
      'SELECT user_id FROM devices WHERE fcm_token = $1',
      [TOKEN],
    );
    expect(owners.rows).toEqual([{ user_id: next.userId }]);

    const foreign = await request(server)
      .delete(`/api/v1/notifications/devices/${TOKEN}`)
      .set('Authorization', me.auth);
    expect(foreign.status).toBe(204);
    expect((await pg.query('SELECT 1 FROM devices WHERE fcm_token = $1', [TOKEN])).rowCount).toBe(
      1,
    );

    const own = await request(server)
      .delete(`/api/v1/notifications/devices/${TOKEN}`)
      .set('Authorization', next.auth);
    expect(own.status).toBe(204);
    expect((await pg.query('SELECT 1 FROM devices WHERE fcm_token = $1', [TOKEN])).rowCount).toBe(
      0,
    );

    for (const body of [
      { token: 'short', platform: 'android' },
      { token: `${TOKEN}<script>`, platform: 'android' },
      { token: TOKEN, platform: 'windows' },
      { token: TOKEN },
    ]) {
      const res = await put(me.auth, body);
      expect(res.status).toBe(400);
      expect(res.body.error.code).toBe('invalid_request');
    }
  });

  it(`keeps the ${MAX_DEVICES_PER_USER} most recent devices`, async () => {
    const me = await as();
    const userId = me.userId;
    for (let i = 0; i <= MAX_DEVICES_PER_USER; i++) {
      await pg.query(
        `INSERT INTO devices (id, user_id, fcm_token, platform, updated_at)
         VALUES ($1, $2, $3, 'ios', now() - make_interval(mins => $4))`,
        [uuidv7(), userId, `old-token-${String(i).padStart(20, '0')}`, 100 - i],
      );
    }
    const res = await request(server)
      .put('/api/v1/notifications/devices')
      .set('Authorization', me.auth)
      .send({ token: TOKEN, platform: 'ios' });
    expect(res.status).toBe(200);
    const { rows } = await pg.query<{ fcm_token: string }>(
      'SELECT fcm_token FROM devices WHERE user_id = $1 ORDER BY updated_at DESC',
      [userId],
    );
    expect(rows).toHaveLength(MAX_DEVICES_PER_USER);
    expect(rows[0].fcm_token).toBe(TOKEN);
    expect(rows.map((row) => row.fcm_token)).not.toContain(`old-token-${'0'.repeat(20)}`);
    expect(rows.map((row) => row.fcm_token)).not.toContain(`old-token-${'1'.padStart(20, '0')}`);
  });

  it('rate-limits device registration', async () => {
    const me = await as();
    const statuses: number[] = [];
    for (let i = 0; i < 11; i++) {
      const res = await request(server)
        .put('/api/v1/notifications/devices')
        .set('Authorization', me.auth)
        .send({ token: `${TOKEN}-${i}`, platform: 'web' });
      statuses.push(res.status);
    }
    expect(statuses.slice(0, 10).every((status) => status === 200)).toBe(true);
    expect(statuses[10]).toBe(429);
  });

  it('reads and updates preferences', async () => {
    const me = await as();
    const get = () =>
      request(server).get('/api/v1/notifications/preferences').set('Authorization', me.auth);
    expect((await get()).body.data).toEqual({
      push_enabled: true,
      likes: true,
      comments: true,
      follows: true,
      video_ready: true,
    });

    const patch = (body: object) =>
      request(server)
        .patch('/api/v1/notifications/preferences')
        .set('Authorization', me.auth)
        .send(body);
    const first = await patch({ likes: false, push_enabled: false });
    expect(first.status).toBe(200);
    expect(first.body.data).toEqual({
      push_enabled: false,
      likes: false,
      comments: true,
      follows: true,
      video_ready: true,
    });
    const second = await patch({ likes: true });
    expect(second.body.data).toMatchObject({ push_enabled: false, likes: true });
    expect((await get()).body.data).toEqual(second.body.data);

    for (const body of [{ likes: 'no' }, { email_enabled: true }]) {
      const res = await patch(body);
      expect(res.status).toBe(400);
      expect(res.body.error.code).toBe('invalid_request');
    }

    // A muted type writes nothing.
    await patch({ follows: false });
    await follow(uuidv7(), me.userId);
    const list = await request(server).get('/api/v1/notifications').set('Authorization', me.auth);
    expect(list.body.data.items).toEqual([]);
  });
});
