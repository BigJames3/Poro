import {
  DeleteObjectCommand,
  GetObjectCommand,
  NoSuchKey,
  PutObjectCommand,
  S3Client,
} from '@aws-sdk/client-s3';
import { NestExpressApplication } from '@nestjs/platform-express';
import { sdkStreamMixin } from '@smithy/util-stream';
import { mockClient } from 'aws-sdk-client-mock';
import { Readable } from 'node:stream';
import { Client } from 'pg';
import sharp from 'sharp';
import request from 'supertest';
import type { App } from 'supertest/types';

import { uuidv7 } from '../src/common/uuid';
import { createApp } from './support/app';
import { JwksServer, SigningKey, signToken, signingKey } from './support/jwt';
import { TestDatabase, startDatabase } from './support/postgres';

const s3 = mockClient(S3Client);

describe('user service HTTP API', () => {
  let db: TestDatabase;
  let pg: Client;
  let jwks: JwksServer;
  let key: SigningKey;
  let app: NestExpressApplication;
  let server: App;

  beforeAll(async () => {
    db = await startDatabase();
    pg = new Client({ connectionString: db.url });
    await pg.connect();
    key = signingKey();
    jwks = new JwksServer();
    jwks.keys = [key.publicJwk];
    const jwksUrl = await jwks.start();
    app = await createApp({
      DATABASE_URL: db.url,
      JWKS_URL: jwksUrl,
      S3_ENDPOINT: 'http://seaweedfs:8333',
      S3_PUBLIC_ENDPOINT: 'http://localhost:9000',
      AVATAR_PUBLIC_BASE_URL: 'https://cdn.poro.test/avatars',
    });
    server = app.getHttpServer();
  });

  afterAll(async () => {
    await (app as NestExpressApplication | undefined)?.close();
    await (pg as Client | undefined)?.end();
    await jwks.stop();
    await (db as TestDatabase | undefined)?.container.stop();
  });

  beforeEach(() => {
    s3.reset();
  });

  async function as(userId: string = uuidv7()): Promise<{ userId: string; auth: string }> {
    return { userId, auth: `Bearer ${await signToken(key, { sub: userId })}` };
  }

  it('requires a valid access token', async () => {
    const res = await request(server).get('/api/v1/users/me');
    expect(res.status).toBe(401);
    expect(res.body).toEqual({
      data: null,
      error: { code: 'unauthorized', message: 'unauthorized' },
      meta: { request_id: expect.any(String) },
    });

    const expired = await signToken(key, { expiresIn: Math.floor(Date.now() / 1000) - 60 });
    for (const header of [
      `Bearer ${expired}`,
      'Bearer not-a-jwt',
      'Basic abc',
      `Bearer ${await signToken(key, { audience: 'other' })}`,
    ]) {
      expect(
        (await request(server).get('/api/v1/users/me').set('Authorization', header)).status,
      ).toBe(401);
    }
  });

  it('creates the profile lazily and echoes a valid request id', async () => {
    const { userId, auth } = await as();
    const res = await request(server)
      .get('/api/v1/users/me')
      .set('Authorization', auth)
      .set('X-Request-ID', 'req-42');
    expect(res.status).toBe(200);
    expect(res.headers['x-request-id']).toBe('req-42');
    expect(res.body.meta).toEqual({ request_id: 'req-42' });
    expect(res.body.error).toBeNull();
    expect(res.body.data).toMatchObject({
      user_id: userId,
      username: null,
      display_name: null,
      is_creator: false,
      avatar_url: null,
      country_code: null,
    });

    const unsafe = await request(server)
      .get('/api/v1/users/me')
      .set('Authorization', auth)
      .set('X-Request-ID', '<script>');
    expect(unsafe.headers['x-request-id']).toMatch(/^[0-9a-f-]{36}$/);
  });

  it('updates the profile and enforces unique, valid usernames', async () => {
    const awa = await as();
    const res = await request(server)
      .patch('/api/v1/users/me')
      .set('Authorization', awa.auth)
      .send({ username: ' @Awa.Kone ', display_name: '  Awa   Koné ', bio: 'Danseuse\r\nAbidjan' });
    expect(res.status).toBe(200);
    expect(res.body.data).toMatchObject({
      username: 'awa.kone',
      display_name: 'Awa Koné',
      bio: 'Danseuse\nAbidjan',
    });

    const other = await as();
    const taken = await request(server)
      .patch('/api/v1/users/me')
      .set('Authorization', other.auth)
      .send({ username: 'AWA.KONE' });
    expect(taken.status).toBe(409);
    expect(taken.body.error.code).toBe('username_taken');

    const cases: [Record<string, unknown>, number, string][] = [
      [{ username: 'a' }, 422, 'username_invalid'],
      [{ username: '0701020304' }, 422, 'username_invalid'],
      [{ username: 'awa..kone' }, 422, 'username_invalid'],
      [{ username: 'Poro_Official' }, 422, 'username_reserved'],
      [{ display_name: 'x'.repeat(51) }, 422, 'display_name_invalid'],
      [{ display_name: 'Awa\u202eenok' }, 422, 'display_name_invalid'],
      [{ bio: 'b'.repeat(161) }, 422, 'bio_invalid'],
      [{ bio: '1\n2\n3\n4\n5\n6' }, 422, 'bio_invalid'],
      [{ username: 42 }, 400, 'invalid_request'],
      [{ is_creator: true }, 400, 'invalid_request'],
    ];
    for (const [body, status, code] of cases) {
      const r = await request(server)
        .patch('/api/v1/users/me')
        .set('Authorization', other.auth)
        .send(body);
      expect({ body, status: r.status, code: r.body.error?.code }).toEqual({ body, status, code });
    }

    const cleared = await request(server)
      .patch('/api/v1/users/me')
      .set('Authorization', awa.auth)
      .send({ bio: null, display_name: '   ' });
    expect(cleared.body.data).toMatchObject({
      bio: null,
      display_name: null,
      username: 'awa.kone',
    });
  });

  it('reports username availability and serves public profiles', async () => {
    const owner = await as();
    await request(server)
      .patch('/api/v1/users/me')
      .set('Authorization', owner.auth)
      .send({ username: 'moussa_d', bio: 'Tailleur' });
    const asker = await as();
    const check = (username: string) =>
      request(server)
        .get('/api/v1/users/username-availability')
        .query({ username })
        .set('Authorization', asker.auth);

    expect((await check('Moussa_D')).body.data).toEqual({
      username: 'moussa_d',
      available: false,
      reason: 'taken',
    });
    expect((await check('fatou.sn')).body.data).toEqual({
      username: 'fatou.sn',
      available: true,
      reason: null,
    });
    expect((await check('support')).body.data).toEqual({
      username: 'support',
      available: false,
      reason: 'reserved',
    });
    expect((await check('x')).body.data.reason).toBe('invalid');
    expect(
      (
        await request(server)
          .get('/api/v1/users/username-availability')
          .set('Authorization', asker.auth)
      ).status,
    ).toBe(400);

    const pub = await request(server).get('/api/v1/users/by-username/MOUSSA_D');
    expect(pub.status).toBe(200);
    expect(pub.body.data).toEqual({
      user_id: owner.userId,
      username: 'moussa_d',
      display_name: null,
      bio: 'Tailleur',
      avatar_url: null,
      is_creator: false,
      creator_since: null,
    });
    expect(pub.body.data).not.toHaveProperty('country_code');
    expect(
      (await request(server).get('/api/v1/users/by-username/nobody_here')).body.error.code,
    ).toBe('profile_not_found');
    expect((await request(server).get('/api/v1/users/by-username/%20')).status).toBe(404);
  });

  it('activates creators once and writes the event to the outbox', async () => {
    const { userId, auth } = await as();
    const early = await request(server).post('/api/v1/users/me/creator').set('Authorization', auth);
    expect(early.status).toBe(422);
    expect(early.body.error.code).toBe('username_required');

    await request(server)
      .patch('/api/v1/users/me')
      .set('Authorization', auth)
      .send({ username: 'kofi.creates' });
    const [first, second] = await Promise.all([
      request(server).post('/api/v1/users/me/creator').set('Authorization', auth),
      request(server).post('/api/v1/users/me/creator').set('Authorization', auth),
    ]);
    expect([first.status, second.status]).toEqual([200, 200]);
    expect(first.body.data.is_creator).toBe(true);
    expect(first.body.data.creator_since).toBe(second.body.data.creator_since);

    const { rows } = await pg.query<{
      topic: string;
      event_key: string;
      payload: Record<string, unknown>;
    }>('SELECT topic, event_key, payload FROM outbox_events WHERE event_key = $1 AND topic = $2', [
      userId,
      'poro.user.creator.activated',
    ]);
    expect(rows).toHaveLength(1);
    expect(rows[0].payload).toMatchObject({
      type: 'poro.user.creator.activated',
      version: 1,
      source: 'poro-user',
      subject: userId,
      data: {
        user_id: userId,
        username: 'kofi.creates',
        activated_at: first.body.data.creator_since,
      },
    });
  });

  async function profileSnapshots(userId: string): Promise<Record<string, unknown>[]> {
    const { rows } = await pg.query<{ payload: { data: Record<string, unknown> } }>(
      `SELECT payload FROM outbox_events
        WHERE event_key = $1 AND topic = 'poro.user.profile.updated'
        ORDER BY created_at, id`,
      [userId],
    );
    return rows.map((r) => r.payload.data);
  }

  it('publishes poro.user.profile.updated only when a public field changes', async () => {
    const { userId, auth } = await as();
    await request(server).get('/api/v1/users/me').set('Authorization', auth);
    expect(await profileSnapshots(userId)).toEqual([]);

    await request(server)
      .patch('/api/v1/users/me')
      .set('Authorization', auth)
      .send({ username: 'mariam.d', display_name: 'Mariam' });
    await request(server)
      .patch('/api/v1/users/me')
      .set('Authorization', auth)
      .send({ bio: 'Cheffe à Bamako' });
    await request(server)
      .patch('/api/v1/users/me')
      .set('Authorization', auth)
      .send({ username: 'mariam.d', display_name: 'Mariam' });
    await request(server).post('/api/v1/users/me/creator').set('Authorization', auth);
    await request(server).post('/api/v1/users/me/creator').set('Authorization', auth);

    const snapshots = await profileSnapshots(userId);
    expect(snapshots).toHaveLength(2);
    expect(snapshots[0]).toEqual({
      user_id: userId,
      username: 'mariam.d',
      display_name: 'Mariam',
      avatar_url: null,
      is_creator: false,
      updated_at: expect.any(String),
    });
    expect(snapshots[1]).toMatchObject({ username: 'mariam.d', is_creator: true });

    const { rows } = await pg.query<{ payload: Record<string, unknown> }>(
      `SELECT payload FROM outbox_events WHERE event_key = $1 AND topic = 'poro.user.profile.updated'`,
      [userId],
    );
    expect(rows[0].payload).toMatchObject({
      type: 'poro.user.profile.updated',
      version: 1,
      source: 'poro-user',
      subject: userId,
    });
  });

  it('publishes a re-encoded avatar and deletes the previous one', async () => {
    const { userId, auth } = await as();
    const upload = await request(server)
      .post('/api/v1/users/me/avatar/upload-url')
      .set('Authorization', auth)
      .send({ content_type: 'image/png' });
    expect(upload.status).toBe(201);
    const { upload_url, fields, upload_key, max_bytes } = upload.body.data;
    expect(upload_url).toBe('http://localhost:9000/poro-avatars');
    expect(upload_key).toMatch(new RegExp(`^avatars/uploads/${userId}/[0-9a-f-]{36}$`));
    expect(fields).toMatchObject({
      key: upload_key,
      'Content-Type': 'image/png',
      Policy: expect.any(String),
    });
    const policy = JSON.parse(Buffer.from(String(fields.Policy), 'base64').toString('utf8'));
    expect(policy.conditions).toContainEqual(['content-length-range', 1, max_bytes]);

    const png = await sharp({
      create: { width: 900, height: 600, channels: 3, background: '#e07a1f' },
    })
      .png()
      .toBuffer();
    s3.on(GetObjectCommand, { Key: upload_key }).resolves({
      ContentLength: png.length,
      Body: sdkStreamMixin(Readable.from([png])),
    });
    const set = await request(server)
      .put('/api/v1/users/me/avatar')
      .set('Authorization', auth)
      .send({ upload_key });
    expect(set.status).toBe(200);
    const put = s3.commandCalls(PutObjectCommand)[0].args[0].input;
    expect(put.Key).toMatch(new RegExp(`^avatars/${userId}/[0-9a-f-]{36}\\.webp$`));
    expect(put.ContentType).toBe('image/webp');
    expect(await sharp(put.Body as Buffer).metadata()).toMatchObject({
      format: 'webp',
      width: 512,
      height: 512,
    });
    expect(set.body.data.avatar_url).toBe(`https://cdn.poro.test/avatars/${put.Key}`);
    expect((await profileSnapshots(userId)).map((d) => d.avatar_url)).toEqual([
      `https://cdn.poro.test/avatars/${put.Key}`,
    ]);
    expect(s3.commandCalls(DeleteObjectCommand).map((c) => c.args[0].input.Key)).toEqual([
      upload_key,
    ]);

    s3.reset();
    const removed = await request(server)
      .delete('/api/v1/users/me/avatar')
      .set('Authorization', auth);
    expect(removed.body.data.avatar_url).toBeNull();
    expect((await profileSnapshots(userId)).map((d) => d.avatar_url)).toEqual([
      `https://cdn.poro.test/avatars/${put.Key}`,
      null,
    ]);
    expect(s3.commandCalls(DeleteObjectCommand).map((c) => c.args[0].input.Key)).toEqual([put.Key]);
  });

  it('rejects avatar uploads that are foreign, missing or not images', async () => {
    const { userId, auth } = await as();
    const confirm = (upload_key: string) =>
      request(server)
        .put('/api/v1/users/me/avatar')
        .set('Authorization', auth)
        .send({ upload_key });

    expect((await confirm(`avatars/uploads/${uuidv7()}/${uuidv7()}`)).body.error.code).toBe(
      'avatar_upload_invalid',
    );
    expect((await confirm(`avatars/uploads/${userId}/../../x`)).body.error.code).toBe(
      'avatar_upload_invalid',
    );

    const missing = `avatars/uploads/${userId}/${uuidv7()}`;
    s3.on(GetObjectCommand, { Key: missing }).rejects(
      new NoSuchKey({ message: 'missing', $metadata: {} }),
    );
    expect((await confirm(missing)).body.error.code).toBe('avatar_not_uploaded');

    const html = `avatars/uploads/${userId}/${uuidv7()}`;
    const body = Buffer.from('<html><script>alert(1)</script></html>');
    s3.on(GetObjectCommand, { Key: html }).resolves({
      ContentLength: body.length,
      Body: sdkStreamMixin(Readable.from([body])),
    });
    expect((await confirm(html)).body.error.code).toBe('avatar_invalid');

    const down = `avatars/uploads/${userId}/${uuidv7()}`;
    s3.on(GetObjectCommand, { Key: down }).rejects(new Error('connect ECONNREFUSED'));
    const unavailable = await confirm(down);
    expect(unavailable.status).toBe(503);
    expect(JSON.stringify(unavailable.body)).not.toContain('ECONNREFUSED');

    const badType = await request(server)
      .post('/api/v1/users/me/avatar/upload-url')
      .set('Authorization', auth)
      .send({ content_type: 'image/svg+xml' });
    expect(badType.status).toBe(400);
    expect(s3.commandCalls(PutObjectCommand)).toHaveLength(0);
  });

  it('renders framework errors in the envelope without internals', async () => {
    const { auth } = await as();
    const notFound = await request(server).get('/api/v1/nope').set('Authorization', auth);
    expect(notFound.status).toBe(404);
    expect(notFound.body.error).toEqual({ code: 'not_found', message: 'not found' });

    const badJson = await request(server)
      .patch('/api/v1/users/me')
      .set('Authorization', auth)
      .set('Content-Type', 'application/json')
      .send('{"username":');
    expect(badJson.status).toBe(400);
    expect(badJson.body.error.code).toBe('invalid_json');

    const tooLarge = await request(server)
      .patch('/api/v1/users/me')
      .set('Authorization', auth)
      .send({ bio: 'x'.repeat(20_000) });
    expect(tooLarge.status).toBe(413);
    expect(tooLarge.body.error.code).toBe('payload_too_large');
    expect(notFound.headers['x-powered-by']).toBeUndefined();
    expect(notFound.headers['x-content-type-options']).toBe('nosniff');
  });

  it('serves probes and metrics without a token', async () => {
    const live = await request(server).get('/health/live');
    expect(live.body).toEqual({ status: 'ok', service: 'user', version: '1.0.0' });
    const ready = await request(server).get('/health/ready');
    expect(ready.status).toBe(200);
    expect(ready.body.checks).toEqual({ postgres: 'up' });

    const metrics = await request(server).get('/metrics');
    expect(metrics.status).toBe(200);
    expect(metrics.text).toContain(
      'http_requests_total{method="GET",route="/api/v1/users/me",status="200"',
    );
    expect(metrics.text).toContain('route="unmatched"');
    expect(metrics.text).not.toMatch(/route="\/api\/v1\/users\/by-username\/moussa/);
  });
});
