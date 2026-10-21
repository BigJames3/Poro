import { NestExpressApplication } from '@nestjs/platform-express';
import request from 'supertest';
import type { App } from 'supertest/types';

import { uuidv7 } from '../src/common/uuid';
import { Envelope } from '../src/events/envelope';
import { PermanentError } from '../src/events/event-consumer';
import { AccountHidingConsumer } from '../src/profiles/account-hiding.consumer';
import { createApp } from './support/app';
import { JwksServer, SigningKey, signToken, signingKey } from './support/jwt';
import { TestDatabase, startDatabase } from './support/postgres';

function change(type: string, userId: unknown, changedAt: unknown): Envelope {
  return {
    id: uuidv7(),
    type,
    version: 1,
    source: 'poro-auth',
    subject: String(userId),
    occurred_at: '2026-10-21T08:00:00Z',
    data: { user_id: userId, erase_at: '2026-11-20T08:00:00Z', changed_at: changedAt },
  };
}
const scheduled = (u: unknown, at: unknown) => change('poro.auth.user.deletion_scheduled', u, at);
const cancelled = (u: unknown, at: unknown) => change('poro.auth.user.deletion_cancelled', u, at);

describe('account hiding while a deletion is pending', () => {
  let db: TestDatabase;
  let jwks: JwksServer;
  let key: SigningKey;
  let app: NestExpressApplication;
  let server: App;
  let hiding: AccountHidingConsumer;

  beforeAll(async () => {
    db = await startDatabase();
    key = signingKey();
    jwks = new JwksServer();
    jwks.keys = [key.publicJwk];
    app = await createApp({ DATABASE_URL: db.url, JWKS_URL: await jwks.start() });
    server = app.getHttpServer();
    hiding = app.get(AccountHidingConsumer);
  });

  afterAll(async () => {
    await (app as NestExpressApplication | undefined)?.close();
    await jwks.stop();
    await (db as TestDatabase | undefined)?.container.stop();
  });

  async function shown(userId: string, username: string): Promise<[number, number]> {
    const byName = await request(server).get(`/api/v1/users/by-username/${username}`);
    const batch = await request(server).get('/api/v1/users').query({ ids: userId });
    return [byName.status, (batch.body as { data: { items: unknown[] } }).data.items.length];
  }

  it('hides the profile until the deletion is cancelled, latest change winning', async () => {
    const userId = uuidv7();
    const auth = `Bearer ${await signToken(key, { sub: userId })}`;
    await request(server)
      .patch('/api/v1/users/me')
      .set('Authorization', auth)
      .send({ username: 'binta.hide' });
    expect(await shown(userId, 'binta.hide')).toEqual([200, 1]);

    await hiding.handle(scheduled(userId, '2026-10-21T08:00:00Z'));
    expect(await shown(userId, 'binta.hide')).toEqual([404, 0]);

    await hiding.handle(cancelled(userId, '2026-10-21T08:00:00.001Z'));
    await hiding.handle(scheduled(userId, '2026-10-21T08:00:00Z'));
    expect(await shown(userId, 'binta.hide')).toEqual([200, 1]);
  });

  it('sends malformed events to the DLQ', async () => {
    await expect(hiding.handle(scheduled('nope', '2026-10-21T08:00:00Z'))).rejects.toThrow(
      PermanentError,
    );
    await expect(hiding.handle(cancelled(uuidv7(), 'yesterday'))).rejects.toThrow(PermanentError);
    await expect(
      hiding.handle({ ...scheduled(uuidv7(), '2026-10-21T08:00:00Z'), version: 2 }),
    ).rejects.toThrow(PermanentError);
  });
});
