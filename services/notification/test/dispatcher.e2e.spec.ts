import { NestExpressApplication } from '@nestjs/platform-express';
import { Client } from 'pg';

import { uuidv7 } from '../src/common/uuid';
import { newEnvelope, type Envelope } from '../src/events/envelope';
import { PermanentError } from '../src/events/event-consumer';
import { DISPATCHER_GROUP, NotificationDispatcher } from '../src/notifications/dispatcher';
import { PurgeService } from '../src/notifications/purge.service';
import { createApp } from './support/app';
import { TestDatabase, startDatabase } from './support/postgres';
import { FakePushSender } from './support/push';

interface Row {
  id: string;
  user_id: string;
  type: string;
  title: string;
  body: string;
  data: Record<string, string>;
  actor_id: string | null;
  actor_count: number;
  entity_type: string;
  entity_id: string;
  video_id: string | null;
  read_at: Date | null;
  last_activity_at: Date;
}

describe('notification dispatcher', () => {
  let db: TestDatabase;
  let pg: Client;
  let app: NestExpressApplication;
  let dispatcher: NotificationDispatcher;
  const push = new FakePushSender();

  beforeAll(async () => {
    db = await startDatabase();
    pg = new Client({ connectionString: db.url });
    await pg.connect();
    app = await createApp({ DATABASE_URL: db.url }, push);
    dispatcher = app.get(NotificationDispatcher);
  });

  afterAll(async () => {
    await (app as NestExpressApplication | undefined)?.close();
    await (pg as Client | undefined)?.end();
    await (db as TestDatabase | undefined)?.container.stop();
  });

  beforeEach(() => {
    push.reset();
  });

  const event = (type: string, data: object, subject = uuidv7()): Envelope =>
    newEnvelope(type, 1, subject, data);

  const notifications = (userId: string): Promise<Row[]> =>
    pg
      .query<Row>(
        'SELECT * FROM notifications WHERE user_id = $1 ORDER BY last_activity_at DESC, id DESC',
        [userId],
      )
      .then((r) => r.rows);

  async function profile(userId: string, displayName: string | null, username: string | null) {
    await dispatcher.handle(
      event('poro.user.profile.updated', {
        user_id: userId,
        username,
        display_name: displayName,
        avatar_url: null,
        is_creator: false,
        updated_at: new Date().toISOString(),
      }),
    );
  }

  async function device(userId: string, token: string) {
    await pg.query(
      `INSERT INTO devices (id, user_id, fcm_token, platform, updated_at)
       VALUES ($1, $2, $3, 'android', now())`,
      [uuidv7(), userId, token],
    );
  }

  const like = (actor: string, owner: string, video: string, at = new Date()) =>
    event('poro.social.like.created', {
      like_id: uuidv7(),
      user_id: actor,
      video_id: video,
      video_owner_id: owner,
      created_at: at.toISOString(),
    });

  const comment = (
    actor: string,
    owner: string,
    video: string,
    parent: { id: string; author: string } | null = null,
    commentId = uuidv7(),
  ) =>
    event('poro.social.comment.created', {
      comment_id: commentId,
      user_id: actor,
      video_id: video,
      video_owner_id: owner,
      parent_id: parent?.id ?? null,
      parent_author_id: parent?.author ?? null,
      excerpt: 'Trop beau ce coucher de soleil sur Abidjan',
      created_at: new Date().toISOString(),
    });

  it('notifies the video owner of a like and pushes to their devices', async () => {
    const [actor, owner, video] = [uuidv7(), uuidv7(), uuidv7()];
    await profile(actor, 'Awa', 'awa');
    await device(owner, 'token-owner-phone-0000001');

    await dispatcher.handle(like(actor, owner, video));

    const [row] = await notifications(owner);
    expect(row).toMatchObject({
      type: 'like',
      title: 'Nouveau j’aime',
      body: 'Awa a aimé ta vidéo',
      actor_id: actor,
      actor_count: 1,
      entity_type: 'video',
      entity_id: video,
      video_id: video,
      data: { video_id: video },
      read_at: null,
    });
    expect(push.sent).toEqual([
      {
        tokens: ['token-owner-phone-0000001'],
        message: {
          title: 'Nouveau j’aime',
          body: 'Awa a aimé ta vidéo',
          data: { video_id: video, notification_id: row.id, type: 'like' },
        },
      },
    ]);
  });

  it('groups likes per video and hour, counting distinct actors once', async () => {
    const [owner, video] = [uuidv7(), uuidv7()];
    const actors = [uuidv7(), uuidv7(), uuidv7()];
    await profile(actors[2], null, 'kofi');
    await device(owner, 'token-owner-group-000001');
    const hour = new Date('2026-10-07T10:05:00Z');

    await dispatcher.handle(like(actors[0], owner, video, hour));
    await pg.query('UPDATE notifications SET read_at = now() WHERE user_id = $1', [owner]);
    await dispatcher.handle(like(actors[1], owner, video, new Date('2026-10-07T10:40:00Z')));
    await dispatcher.handle(like(actors[1], owner, video, new Date('2026-10-07T10:45:00Z')));
    await dispatcher.handle(like(actors[2], owner, video, new Date('2026-10-07T10:59:59Z')));

    const rows = await notifications(owner);
    expect(rows).toHaveLength(1);
    expect(rows[0]).toMatchObject({
      actor_id: actors[2],
      actor_count: 3,
      title: 'Nouveaux j’aime',
      body: '@kofi et 2 autres personnes ont aimé ta vidéo',
      read_at: null,
    });
    expect(push.sent).toHaveLength(1);

    await dispatcher.handle(like(actors[0], owner, video, new Date('2026-10-07T11:00:00Z')));
    expect(await notifications(owner)).toHaveLength(2);
  });

  it('names a second actor as one other person', async () => {
    const [owner, video, first, second] = [uuidv7(), uuidv7(), uuidv7(), uuidv7()];
    await dispatcher.handle(like(first, owner, video));
    await dispatcher.handle(like(second, owner, video));
    const [row] = await notifications(owner);
    expect(row.body).toBe('Quelqu’un et 1 autre personne ont aimé ta vidéo');
  });

  it('never notifies an actor of their own action', async () => {
    const [self, video, other] = [uuidv7(), uuidv7(), uuidv7()];
    await dispatcher.handle(like(self, self, video));
    await dispatcher.handle(comment(self, self, video));
    await dispatcher.handle(
      event('poro.social.follow.created', {
        follower_id: self,
        following_id: self,
        created_at: new Date().toISOString(),
      }),
    );
    // Replying to your own comment on someone else's video notifies the owner only.
    await dispatcher.handle(comment(self, other, video, { id: uuidv7(), author: self }));
    expect(await notifications(self)).toEqual([]);
    expect((await notifications(other)).map((row) => row.type)).toEqual(['comment']);
  });

  it('notifies the owner of a comment and the parent author of a reply', async () => {
    const [actor, owner, parentAuthor, video, parentId] = [
      uuidv7(),
      uuidv7(),
      uuidv7(),
      uuidv7(),
      uuidv7(),
    ];
    await profile(actor, 'Moussa', 'moussa');
    const commentId = uuidv7();
    await dispatcher.handle(
      comment(actor, owner, video, { id: parentId, author: parentAuthor }, commentId),
    );

    const [reply] = await notifications(parentAuthor);
    expect(reply).toMatchObject({
      type: 'reply',
      title: 'Nouvelle réponse',
      body: 'Moussa a répondu à ton commentaire : « Trop beau ce coucher de soleil sur Abidjan »',
      entity_type: 'comment',
      entity_id: commentId,
      data: { video_id: video, comment_id: commentId, parent_id: parentId },
    });
    const [ownerRow] = await notifications(owner);
    expect(ownerRow).toMatchObject({
      type: 'comment',
      body: 'Moussa a commenté ta vidéo : « Trop beau ce coucher de soleil sur Abidjan »',
    });

    // The owner wrote the parent: one notification, as a reply.
    const ownCommentId = uuidv7();
    await dispatcher.handle(
      comment(actor, owner, video, { id: parentId, author: owner }, ownCommentId),
    );
    const ownerRows = await notifications(owner);
    expect(ownerRows.map((row) => row.type)).toEqual(['reply', 'comment']);

    await dispatcher.handle(
      event('poro.social.comment.deleted', {
        comment_id: commentId,
        video_id: video,
        user_id: actor,
        deleted_at: new Date().toISOString(),
      }),
    );
    expect(await notifications(parentAuthor)).toEqual([]);
    expect((await notifications(owner)).map((row) => row.entity_id)).toEqual([ownCommentId]);
  });

  it('notifies follows once per follower', async () => {
    const [follower, followed] = [uuidv7(), uuidv7()];
    const follow = () =>
      event('poro.social.follow.created', {
        follower_id: follower,
        following_id: followed,
        created_at: new Date().toISOString(),
      });
    await dispatcher.handle(follow());
    await dispatcher.handle(follow());
    const rows = await notifications(followed);
    expect(rows).toHaveLength(1);
    expect(rows[0]).toMatchObject({
      type: 'follow',
      body: 'Quelqu’un a commencé à te suivre',
      entity_type: 'user',
      entity_id: follower,
      data: { user_id: follower },
    });
  });

  it('tells authors their video is ready and forgets deleted videos', async () => {
    const [author, video, liker] = [uuidv7(), uuidv7(), uuidv7()];
    await dispatcher.handle(
      event('poro.video.ready', {
        video_id: video,
        user_id: author,
        duration_ms: 1000,
        width: 720,
        height: 1280,
        hls_key: 'k',
        thumbnail_key: 't',
        renditions: [],
        ready_at: new Date().toISOString(),
        title: 'Marché de Treichville',
      }),
    );
    await dispatcher.handle(like(liker, author, video));
    const rows = await notifications(author);
    expect(rows.map((row) => row.type).sort()).toEqual(['like', 'video_ready']);
    expect(rows.find((row) => row.type === 'video_ready')).toMatchObject({
      title: 'Ta vidéo est en ligne',
      body: '« Marché de Treichville » est maintenant visible par tous.',
      actor_id: null,
    });

    await dispatcher.handle(
      event('poro.video.deleted', {
        video_id: video,
        user_id: author,
        deleted_at: new Date().toISOString(),
      }),
    );
    expect(await notifications(author)).toEqual([]);
  });

  it('honours preferences: a muted type is not stored, push off keeps in-app', async () => {
    const [owner, actor, video] = [uuidv7(), uuidv7(), uuidv7()];
    await device(owner, 'token-owner-prefs-0000001');
    await pg.query(
      `INSERT INTO preferences (user_id, push_enabled, likes, comments, follows, video_ready, updated_at)
       VALUES ($1, false, false, true, true, true, now())`,
      [owner],
    );
    await dispatcher.handle(like(actor, owner, video));
    await dispatcher.handle(comment(actor, owner, video));
    expect((await notifications(owner)).map((row) => row.type)).toEqual(['comment']);
    expect(push.sent).toEqual([]);
  });

  it('removes tokens FCM rejects and survives push failures', async () => {
    const [owner, actor] = [uuidv7(), uuidv7()];
    await device(owner, 'token-good-000000000001');
    await device(owner, 'token-dead-000000000001');
    push.invalid.add('token-dead-000000000001');
    await dispatcher.handle(like(actor, owner, uuidv7()));
    const { rows } = await pg.query<{ fcm_token: string }>(
      'SELECT fcm_token FROM devices WHERE user_id = $1',
      [owner],
    );
    expect(rows.map((row) => row.fcm_token)).toEqual(['token-good-000000000001']);

    push.failWith = new Error('fcm unavailable');
    await expect(dispatcher.handle(like(uuidv7(), owner, uuidv7()))).resolves.toBeUndefined();
    expect(await notifications(owner)).toHaveLength(2);
  });

  it('applies each event once', async () => {
    const [owner, actor] = [uuidv7(), uuidv7()];
    await device(owner, 'token-owner-once-00000001');
    const env = like(actor, owner, uuidv7());
    await dispatcher.handle(env);
    await dispatcher.handle(env);
    expect(push.sent).toHaveLength(1);
    const { rows } = await pg.query<{ n: string }>(
      'SELECT count(*) AS n FROM inbox_events WHERE consumer = $1 AND event_id = $2',
      [DISPATCHER_GROUP, env.id],
    );
    expect(rows[0].n).toBe('1');
  });

  it('keeps the newest profile snapshot', async () => {
    const userId = uuidv7();
    const snapshot = (name: string, at: string) =>
      event('poro.user.profile.updated', {
        user_id: userId,
        username: name.toLowerCase(),
        display_name: name,
        avatar_url: 'https://cdn.poro.test/a.jpg',
        is_creator: true,
        updated_at: at,
      });
    await dispatcher.handle(snapshot('Nouveau', '2026-10-07T12:00:00Z'));
    await dispatcher.handle(snapshot('Ancien', '2026-10-07T11:00:00Z'));
    await dispatcher.handle(
      event('poro.auth.user.created', {
        user_id: userId,
        signup_method: 'phone',
        country_code: 'CI',
        language: 'fr',
        created_at: '2026-10-07T10:00:00Z',
      }),
    );
    const created = uuidv7();
    await dispatcher.handle(
      event('poro.auth.user.created', {
        user_id: created,
        signup_method: 'email',
        country_code: null,
        language: 'fr',
        created_at: '2026-10-07T10:00:00Z',
      }),
    );
    const { rows } = await pg.query<{ user_id: string; display_name: string | null }>(
      'SELECT user_id, display_name FROM user_projections WHERE user_id = ANY($1) ORDER BY display_name',
      [[userId, created]],
    );
    expect(rows).toEqual([
      { user_id: userId, display_name: 'Nouveau' },
      { user_id: created, display_name: null },
    ]);
  });

  it.each([
    ['poro.social.like.created', { user_id: 'nope' }, 'user_id must be a UUID'],
    [
      'poro.social.like.created',
      { user_id: uuidv7(), video_id: uuidv7(), video_owner_id: uuidv7(), created_at: 'x' },
      'created_at must be a date-time',
    ],
    [
      'poro.video.ready',
      { video_id: uuidv7(), user_id: uuidv7(), title: 42 },
      'title must be a string of at most 100 characters',
    ],
    ['poro.social.share.created', { user_id: uuidv7() }, 'unsupported event'],
  ])('rejects %s with %p permanently', async (type, data, message) => {
    const err = await dispatcher.handle(event(type, data)).catch((e: unknown) => e);
    expect(err).toBeInstanceOf(PermanentError);
    expect((err as Error).message).toContain(message);
  });

  it('rejects unknown versions permanently', async () => {
    const env = { ...like(uuidv7(), uuidv7(), uuidv7()), version: 2 };
    await expect(dispatcher.handle(env)).rejects.toThrow('unsupported event');
  });

  it('purges notifications without recent activity and old inbox rows', async () => {
    const owner = uuidv7();
    await dispatcher.handle(like(uuidv7(), owner, uuidv7()));
    await dispatcher.handle(like(uuidv7(), owner, uuidv7()));
    const [recent, old] = await notifications(owner);
    await pg.query(
      `UPDATE notifications SET last_activity_at = now() - interval '91 days' WHERE id = $1`,
      [old.id],
    );
    await pg.query(
      `UPDATE inbox_events SET processed_at = now() - interval '31 days'
       WHERE event_id = (SELECT event_id FROM inbox_events ORDER BY processed_at LIMIT 1)`,
    );
    const result = await app.get(PurgeService).purge();
    expect(result).toEqual({ notifications: 1, inbox: 1 });
    expect((await notifications(owner)).map((row) => row.id)).toEqual([recent.id]);
  });
});
