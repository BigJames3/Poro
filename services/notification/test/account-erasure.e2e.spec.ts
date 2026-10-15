import { NestExpressApplication } from '@nestjs/platform-express';
import { Client } from 'pg';

import { uuidv7 } from '../src/common/uuid';
import { newEnvelope, type Envelope } from '../src/events/envelope';
import { PermanentError } from '../src/events/event-consumer';
import {
  ACCOUNT_ERASURE_GROUP,
  AccountErasureConsumer,
} from '../src/notifications/account-erasure.consumer';
import { NotificationDispatcher } from '../src/notifications/dispatcher';
import { createApp } from './support/app';
import { TestDatabase, startDatabase } from './support/postgres';
import { FakePushSender } from './support/push';

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

describe('account erasure (poro.auth.user.deleted)', () => {
  let db: TestDatabase;
  let pg: Client;
  let app: NestExpressApplication;
  let dispatcher: NotificationDispatcher;
  let erasure: AccountErasureConsumer;
  const push = new FakePushSender();

  beforeAll(async () => {
    db = await startDatabase();
    pg = new Client({ connectionString: db.url });
    await pg.connect();
    app = await createApp({ DATABASE_URL: db.url }, push);
    dispatcher = app.get(NotificationDispatcher);
    erasure = app.get(AccountErasureConsumer);
  });

  afterAll(async () => {
    await (app as NestExpressApplication | undefined)?.close();
    await (pg as Client | undefined)?.end();
    await (db as TestDatabase | undefined)?.container.stop();
  });

  beforeEach(() => {
    push.reset();
  });

  const event = (type: string, data: object): Envelope => newEnvelope(type, 1, uuidv7(), data);

  const profile = (userId: string, name: string, at = new Date()) =>
    event('poro.user.profile.updated', {
      user_id: userId,
      username: name.toLowerCase(),
      display_name: name,
      avatar_url: 'https://cdn.poro.test/a.webp',
      is_creator: false,
      updated_at: at.toISOString(),
    });

  const userCreated = (userId: string) =>
    event('poro.auth.user.created', {
      user_id: userId,
      signup_method: 'phone',
      country_code: 'CI',
      language: 'fr',
      created_at: '2026-10-01T10:00:00Z',
    });

  const like = (actor: string, owner: string, video: string, at = new Date()) =>
    event('poro.social.like.created', {
      like_id: uuidv7(),
      user_id: actor,
      video_id: video,
      video_owner_id: owner,
      created_at: at.toISOString(),
    });

  const follow = (follower: string, following: string) =>
    event('poro.social.follow.created', {
      follow_id: uuidv7(),
      follower_id: follower,
      following_id: following,
      created_at: new Date().toISOString(),
    });

  const comment = (actor: string, owner: string) =>
    event('poro.social.comment.created', {
      comment_id: uuidv7(),
      user_id: actor,
      video_id: uuidv7(),
      video_owner_id: owner,
      parent_id: null,
      parent_author_id: null,
      excerpt: 'Bravo Awa',
      created_at: new Date().toISOString(),
    });

  async function device(userId: string, token: string): Promise<void> {
    await pg.query(
      `INSERT INTO devices (id, user_id, fcm_token, platform, updated_at)
       VALUES ($1, $2, $3, 'android', now())`,
      [uuidv7(), userId, token],
    );
  }

  async function preferences(userId: string): Promise<void> {
    await pg.query(
      `INSERT INTO preferences (user_id, push_enabled, likes, comments, follows, video_ready, orders, updated_at)
       VALUES ($1, true, true, true, true, true, true, now())`,
      [userId],
    );
  }

  /** Every row of every table that names one of userIds, as comparable data. */
  async function rowsOf(userIds: string[]): Promise<Record<string, unknown[]>> {
    const q = async (sql: string) => (await pg.query<Record<string, unknown>>(sql, [userIds])).rows;
    return {
      devices: await q(
        'SELECT user_id, fcm_token FROM devices WHERE user_id = ANY($1) ORDER BY fcm_token',
      ),
      preferences: await q(
        'SELECT user_id FROM preferences WHERE user_id = ANY($1) ORDER BY user_id',
      ),
      user_projections: await q(
        `SELECT user_id, username, display_name, avatar_url FROM user_projections
          WHERE user_id = ANY($1) ORDER BY user_id`,
      ),
      notifications: await q(
        `SELECT id, user_id, actor_id, actor_count, title, body FROM notifications
          WHERE user_id = ANY($1) OR actor_id = ANY($1) ORDER BY id`,
      ),
      notification_actors: await q(
        `SELECT notification_id, actor_id FROM notification_actors
          WHERE actor_id = ANY($1) ORDER BY notification_id, actor_id`,
      ),
    };
  }

  async function claims(eventId: string): Promise<number> {
    const { rows } = await pg.query<{ n: string }>(
      'SELECT count(*) AS n FROM inbox_events WHERE consumer = $1 AND event_id = $2',
      [ACCOUNT_ERASURE_GROUP, eventId],
    );
    return Number(rows[0].n);
  }

  /**
   * gone is erased; keep and other are not. gone received a like and a
   * follow, followed and commented keep, and liked a video of keep within the
   * hour of other's like, so other is the named actor of that group.
   */
  async function scenario() {
    const [gone, keep, other] = [uuidv7(), uuidv7(), uuidv7()];
    for (const [id, name] of [
      [gone, 'Gone'],
      [keep, 'Keep'],
      [other, 'Other'],
    ]) {
      await dispatcher.handle(profile(id, name));
      await device(id, `token-${id}`);
      await preferences(id);
    }
    const hour = new Date('2026-10-15T07:05:00Z');
    const [groupVideo, keptVideo] = [uuidv7(), uuidv7()];
    for (const env of [
      like(keep, gone, uuidv7()),
      follow(other, gone),
      follow(gone, keep),
      comment(gone, keep),
      like(gone, keep, groupVideo, hour),
      like(other, keep, groupVideo, new Date(hour.getTime() + 60_000)),
      like(other, keep, keptVideo),
      follow(other, keep),
    ]) {
      await dispatcher.handle(env);
    }
    return { gone, keep, other };
  }

  it('forgets the account and every notification it received or acted in', async () => {
    const { gone, keep, other } = await scenario();
    const before = await rowsOf([keep, other]);
    expect(before.notifications.filter((n) => (n as { user_id: string }).user_id === keep)).toEqual(
      [
        expect.objectContaining({ actor_id: gone, body: 'Gone a commencé à te suivre' }),
        expect.objectContaining({
          actor_id: gone,
          body: expect.stringContaining('Gone a commenté'),
        }),
        expect.objectContaining({ actor_id: other, actor_count: 2 }),
        expect.objectContaining({ actor_id: other, actor_count: 1 }),
        expect.objectContaining({ actor_id: other, body: 'Other a commencé à te suivre' }),
      ],
    );
    const env = deleted(gone.toUpperCase());

    await erasure.handle(env);

    expect(await rowsOf([gone])).toEqual({
      devices: [],
      preferences: [],
      user_projections: [],
      notifications: [],
      notification_actors: [],
    });
    const after = await rowsOf([keep, other]);
    const goneRelated = new Set(
      before.notifications
        .filter((n) => {
          const row = n as { user_id: string; actor_id: string | null; actor_count: number };
          return row.user_id === gone || row.actor_id === gone || row.actor_count === 2;
        })
        .map((n) => (n as { id: string }).id),
    );
    // Received by gone: keep's like, other's follow. Acted by gone: a follow, a
    // comment. Grouped: the like of keep's video named after other.
    expect(goneRelated.size).toBe(5);
    expect(after).toEqual({
      ...before,
      notifications: before.notifications.filter((n) => !goneRelated.has((n as { id: string }).id)),
      notification_actors: before.notification_actors.filter(
        (a) => !goneRelated.has((a as { notification_id: string }).notification_id),
      ),
    });
    const { rows } = await pg.query(
      'SELECT user_id, erased_at FROM erased_users WHERE user_id = $1',
      [gone],
    );
    expect(rows).toEqual([{ user_id: gone, erased_at: new Date('2026-10-15T08:00:00.123Z') }]);
    expect(await claims(env.id)).toBe(1);
  });

  it('is a no-op when replayed, and a second erasure is harmless', async () => {
    const { gone, keep, other } = await scenario();
    const env = deleted(gone);
    await erasure.handle(env);
    const after = await rowsOf([gone, keep, other]);
    const erased = (await pg.query('SELECT * FROM erased_users WHERE user_id = $1', [gone])).rows;

    await erasure.handle(env);
    const again = deleted(gone, uuidv7(), '2026-11-01T00:00:00Z');
    await erasure.handle(again);

    expect(await rowsOf([gone, keep, other])).toEqual(after);
    expect((await pg.query('SELECT * FROM erased_users WHERE user_id = $1', [gone])).rows).toEqual(
      erased,
    );
    expect(await claims(env.id)).toBe(1);
    expect(await claims(again.id)).toBe(1);
  });

  it('never writes the account back from late or replayed events', async () => {
    const { gone, keep } = await scenario();
    await erasure.handle(deleted(gone));
    const after = await rowsOf([gone, keep]);
    push.reset();

    for (const env of [
      userCreated(gone),
      profile(gone, 'Revenant', new Date(Date.now() + 60_000)),
      like(gone, keep, uuidv7()),
      like(keep, gone, uuidv7()),
      follow(gone, keep),
    ]) {
      await dispatcher.handle(env);
    }

    expect(await rowsOf([gone, keep])).toEqual(after);
    expect(push.sent).toEqual([]);

    // An erasure processed before the projection was ever created holds too.
    const unknown = uuidv7();
    await erasure.handle(deleted(unknown));
    await dispatcher.handle(userCreated(unknown));
    await dispatcher.handle(profile(unknown, 'Late'));
    expect((await rowsOf([unknown])).user_projections).toEqual([]);
  });

  it('notifies only the party still there when an erasure cancels an order', async () => {
    const [buyer, seller] = [uuidv7(), uuidv7()];
    await erasure.handle(deleted(buyer));
    const order = { order_id: uuidv7(), buyer_id: buyer, shop_id: uuidv7(), seller_id: seller };

    await dispatcher.handle(
      event('poro.order.order.cancelled', {
        ...order,
        reason: 'account_deleted',
        paid: false,
        cancelled_at: new Date().toISOString(),
      }),
    );

    expect((await rowsOf([buyer])).notifications).toEqual([]);
    expect((await rowsOf([seller])).notifications).toEqual([
      expect.objectContaining({ user_id: seller, actor_id: null, title: 'Commande annulée' }),
    ]);
  });

  it('rejects malformed events permanently, before touching the database', async () => {
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
      expect(await claims(env.id)).toBe(0);
    }
    const { rows } = await pg.query('SELECT 1 FROM erased_users WHERE user_id = $1', [userId]);
    expect(rows).toEqual([]);
  });
});
