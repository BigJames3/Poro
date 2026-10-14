import { NestExpressApplication } from '@nestjs/platform-express';
import Redis from 'ioredis';
import request from 'supertest';
import type { App } from 'supertest/types';
import { GenericContainer, StartedTestContainer } from 'testcontainers';

import { uuidv7 } from '../src/common/uuid';
import { createApp } from './support/app';
import { JwksServer, SigningKey, signToken, signingKey } from './support/jwt';
import { TestDatabase, startDatabase } from './support/postgres';

/** Any authenticated route: the guard runs before the handler. */
const ROUTE = '/api/v1/shops/me';

/**
 * Auth writes the sessions it revokes to a Redis registry shared by every
 * service (ADR-0016). This service must refuse their access tokens, and answer
 * 503 rather than accept a token it cannot check.
 */
describe('revoked sessions', () => {
  let db: TestDatabase;
  let redisContainer: StartedTestContainer;
  let redisStopped = false;
  let registryUrl: string;
  let jwks: JwksServer;
  let key: SigningKey;
  let app: NestExpressApplication;
  let server: App;

  beforeAll(async () => {
    [db, redisContainer] = await Promise.all([
      startDatabase(),
      new GenericContainer('redis:7-alpine').withExposedPorts(6379).start(),
    ]);
    registryUrl = `redis://127.0.0.1:${redisContainer.getMappedPort(6379)}/15`;
    key = signingKey();
    jwks = new JwksServer();
    jwks.keys = [key.publicJwk];
    app = await createApp({
      DATABASE_URL: db.url,
      JWKS_URL: await jwks.start(),
      REVOCATION_REDIS_URL: registryUrl,
    });
    server = app.getHttpServer();
  });

  afterAll(async () => {
    await (app as NestExpressApplication | undefined)?.close();
    await jwks.stop();
    await (db as TestDatabase | undefined)?.container.stop();
    if (!redisStopped) {
      await (redisContainer as StartedTestContainer | undefined)?.stop();
    }
  });

  const call = async (sid: string) =>
    request(server)
      .get(ROUTE)
      .set('Authorization', `Bearer ${await signToken(key, { sub: uuidv7(), sid })}`);

  it('refuses the access tokens of a session auth revoked', async () => {
    const revoked = uuidv7();
    expect((await call(revoked)).status).not.toBe(401);

    // As auth does when a session is revoked.
    const registry = new Redis(registryUrl);
    await registry.set(`poro:revoked:sid:${revoked}`, '1', 'EX', 960);
    await registry.quit();
    const refused = await call(revoked);
    expect(refused.status).toBe(401);
    expect(refused.body.error.code).toBe('unauthorized');

    expect((await call(uuidv7())).status).not.toBe(401);
  });

  it('answers 503 when the registry cannot be read', async () => {
    await redisContainer.stop();
    redisStopped = true;
    const res = await call(uuidv7());
    expect(res.status).toBe(503);
    expect(res.body.error.code).toBe('unavailable');
  });
});
