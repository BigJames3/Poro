import {
  Inject,
  Injectable,
  Logger,
  OnApplicationBootstrap,
  OnApplicationShutdown,
} from '@nestjs/common';

import { isUuid, uuidv7 } from '../common/uuid';
import { appConfig, type AppConfigType } from '../config/app.config';
import {
  CONSUMED_TYPES,
  type Currency,
  TYPE_AUTH_USER_CREATED,
  TYPE_MODERATION_CONTENT_REMOVED,
  TYPE_ORDER_CANCELLED,
  TYPE_ORDER_COMPLETED,
  TYPE_ORDER_PLACED,
  TYPE_ORDER_SHIPPED,
  TYPE_SHOP_UPDATED,
  TYPE_SOCIAL_COMMENT_CREATED,
  TYPE_SOCIAL_COMMENT_DELETED,
  TYPE_SOCIAL_FOLLOW_CREATED,
  TYPE_SOCIAL_LIKE_CREATED,
  TYPE_USER_PROFILE_UPDATED,
  TYPE_VIDEO_DELETED,
  TYPE_VIDEO_READY,
} from '../events/catalog';
import { Envelope } from '../events/envelope';
import { KafkaEventConsumer, PermanentError } from '../events/event-consumer';
import { claim } from '../events/inbox';
import type { Prisma } from '../generated/prisma/client';
import { MetricsService } from '../metrics/metrics';
import { PrismaService } from '../prisma/prisma.service';
import type { PushMessage } from '../push/push.sender';
import { PushService } from '../push/push.service';
import { actorName, formatAmount, messages, UNKNOWN_SHOP, type Message } from './messages.fr';
import {
  DEFAULT_PREFERENCES,
  type NotificationType,
  PREFERENCE_OF,
  PUSH_ONLY_PREFERENCES,
} from './notification.types';

export const DISPATCHER_GROUP = 'poro-notification-dispatcher';

type Tx = Prisma.TransactionClient;
type Data = Record<string, unknown>;

/** A notification to write for one recipient. */
interface Draft {
  userId: string;
  type: NotificationType;
  actorId: string | null;
  entityType: 'video' | 'comment' | 'user' | 'order';
  entityId: string;
  videoId: string | null;
  groupKey: string;
  data: Record<string, string>;
  /** Grouped drafts fold every actor of the same group_key into one row. */
  grouped: boolean;
  render: (actor: string, others: number) => Message;
}

/** A push to send once the transaction has committed. */
interface PendingPush {
  userId: string;
  message: PushMessage;
}

function uuidField(data: Data, field: string): string {
  const value = data[field];
  if (!isUuid(value)) {
    throw new PermanentError(`${field} must be a UUID`);
  }
  return value.toLowerCase();
}

function optionalUuidField(data: Data, field: string): string | null {
  const value = data[field];
  return value === null || value === undefined ? null : uuidField(data, field);
}

function dateField(data: Data, field: string): Date {
  const value = data[field];
  const date = typeof value === 'string' ? new Date(value) : undefined;
  if (date === undefined || Number.isNaN(date.getTime())) {
    throw new PermanentError(`${field} must be a date-time`);
  }
  return date;
}

function optionalString(data: Data, field: string, max: number): string | null {
  const value = data[field];
  if (value === null || value === undefined) {
    return null;
  }
  if (typeof value !== 'string' || Array.from(value).length > max) {
    throw new PermanentError(`${field} must be a string of at most ${max} characters`);
  }
  return value;
}

function requiredString(data: Data, field: string, max: number): string {
  const value = optionalString(data, field, max);
  if (value === null || value.trim() === '') {
    throw new PermanentError(`${field} must be a non-empty string`);
  }
  return value;
}

const CURRENCIES: readonly Currency[] = ['XOF', 'XAF', 'NGN'];

function currencyField(data: Data, field: string): Currency {
  const value = data[field];
  if (!CURRENCIES.includes(value as Currency)) {
    throw new PermanentError(`${field} must be one of ${CURRENCIES.join(', ')}`);
  }
  return value as Currency;
}

/** A total in minor units: a non-negative safe integer. */
function amountField(data: Data, field: string): number {
  const value = data[field];
  if (typeof value !== 'number' || !Number.isSafeInteger(value) || value < 0) {
    throw new PermanentError(`${field} must be a non-negative integer`);
  }
  return value;
}

/** yyyyMMddHH in UTC: likes of one video are grouped per hour of the like. */
function hourBucket(at: Date): string {
  return at.toISOString().slice(0, 13).replace(/[-T]/g, '');
}

interface OrderParties {
  orderId: string;
  buyerId: string;
  sellerId: string;
  shopId: string;
}

function orderParties(data: Data): OrderParties {
  return {
    orderId: uuidField(data, 'order_id'),
    buyerId: uuidField(data, 'buyer_id'),
    sellerId: uuidField(data, 'seller_id'),
    shopId: uuidField(data, 'shop_id'),
  };
}

/**
 * One notification per recipient and step of an order. The buyer is named
 * through the actor projection; the buyer side names the shop instead.
 */
function orderDraft(
  order: OrderParties,
  role: 'buyer' | 'seller',
  type: NotificationType,
  step: string,
  actorId: string | null,
  render: (actor: string) => Message,
): Draft {
  return {
    userId: role === 'buyer' ? order.buyerId : order.sellerId,
    type,
    actorId,
    entityType: 'order',
    entityId: order.orderId,
    videoId: null,
    groupKey: `order:${order.orderId}:${step}`,
    data: { order_id: order.orderId, role },
    grouped: false,
    render,
  };
}

/** What the buyer reads when the order ends without them. */
const BUYER_CANCEL_MESSAGES: Partial<Record<string, (shop: string, refund: boolean) => Message>> = {
  stock_rejected: (shop, refund) => messages.orderRejected(shop, refund),
  seller_cancelled: (shop, refund) => messages.orderCancelledBySeller(shop, refund),
  payment_timeout: (shop, refund) => messages.orderPaymentTimeout(shop, refund),
  shop_suspended: (shop, refund) => messages.orderCancelledShopSuspended(shop, refund),
};

/**
 * Turns social, video and order events into notifications, and keeps the actor
 * and shop projections. The notification and the inbox row commit together; pushes are
 * sent after the commit, at most once.
 */
@Injectable()
export class NotificationDispatcher implements OnApplicationBootstrap, OnApplicationShutdown {
  private readonly logger = new Logger(NotificationDispatcher.name);
  private consumer: KafkaEventConsumer | undefined;

  constructor(
    private readonly prisma: PrismaService,
    private readonly push: PushService,
    private readonly metrics: MetricsService,
    @Inject(appConfig.KEY) private readonly config: AppConfigType,
  ) {}

  onApplicationBootstrap(): void {
    if (!this.config.kafkaConsumerEnabled) {
      this.logger.warn('kafka consumer disabled by KAFKA_CONSUMER_ENABLED=false');
      return;
    }
    if (!this.push.enabled) {
      this.logger.warn('FCM_CREDENTIALS_B64 is not set: push disabled, in-app only');
    }
    this.consumer = new KafkaEventConsumer(
      this.config.kafkaBrokers,
      { group: DISPATCHER_GROUP, topics: [...CONSUMED_TYPES], records: this.metrics.kafkaRecords },
      (env) => this.handle(env),
    );
    this.consumer.start();
  }

  async onApplicationShutdown(): Promise<void> {
    await this.consumer?.stop();
  }

  async handle(env: Envelope): Promise<void> {
    if (env.version !== 1) {
      throw new PermanentError(`unsupported event ${env.type} v${env.version}`);
    }
    const data = env.data as Data;
    const pushes: PendingPush[] = [];
    await this.prisma.$transaction(async (tx) => {
      if (!(await claim(tx, DISPATCHER_GROUP, env.id))) {
        return;
      }
      switch (env.type) {
        case TYPE_AUTH_USER_CREATED:
          await this.userCreated(tx, data);
          return;
        case TYPE_USER_PROFILE_UPDATED:
          await this.profileUpdated(tx, data);
          return;
        case TYPE_VIDEO_DELETED:
          await tx.notification.deleteMany({
            where: { videoId: uuidField(data, 'video_id') },
          });
          return;
        case TYPE_SOCIAL_COMMENT_DELETED:
          await tx.notification.deleteMany({
            where: {
              entityType: 'comment',
              entityId: uuidField(data, 'comment_id'),
            },
          });
          return;
        case TYPE_MODERATION_CONTENT_REMOVED:
          await this.contentRemoved(tx, data);
          return;
        case TYPE_SHOP_UPDATED:
          await this.shopUpdated(tx, data);
          return;
        default:
          for (const draft of await this.drafts(tx, env.type, data)) {
            const pending = await this.write(tx, draft);
            if (pending) {
              pushes.push(pending);
            }
          }
      }
    });
    for (const pending of pushes) {
      await this.push.deliver(pending.userId, pending.message);
    }
  }

  private async drafts(tx: Tx, type: string, data: Data): Promise<Draft[]> {
    switch (type) {
      case TYPE_ORDER_PLACED:
        return this.orderPlacedDrafts(tx, data);
      case TYPE_ORDER_SHIPPED:
        return this.orderShippedDrafts(tx, data);
      case TYPE_ORDER_CANCELLED:
        return this.orderCancelledDrafts(tx, data);
      case TYPE_ORDER_COMPLETED:
        return this.orderCompletedDrafts(data);
      case TYPE_SOCIAL_LIKE_CREATED:
        return this.likeDrafts(data);
      case TYPE_SOCIAL_COMMENT_CREATED:
        return this.commentDrafts(data);
      case TYPE_SOCIAL_FOLLOW_CREATED:
        return this.followDrafts(data);
      case TYPE_VIDEO_READY:
        return this.videoReadyDrafts(data);
      default:
        throw new PermanentError(`unsupported event ${type}`);
    }
  }

  private likeDrafts(data: Data): Draft[] {
    const actorId = uuidField(data, 'user_id');
    const videoId = uuidField(data, 'video_id');
    const createdAt = dateField(data, 'created_at');
    return [
      {
        userId: uuidField(data, 'video_owner_id'),
        type: 'like',
        actorId,
        entityType: 'video',
        entityId: videoId,
        videoId,
        groupKey: `like:${videoId}:${hourBucket(createdAt)}`,
        data: { video_id: videoId },
        grouped: true,
        render: (actor, others) => messages.like(actor, others),
      },
    ];
  }

  private commentDrafts(data: Data): Draft[] {
    const actorId = uuidField(data, 'user_id');
    const commentId = uuidField(data, 'comment_id');
    const videoId = uuidField(data, 'video_id');
    const ownerId = uuidField(data, 'video_owner_id');
    const parentId = optionalUuidField(data, 'parent_id');
    const parentAuthorId = optionalUuidField(data, 'parent_author_id');
    const excerpt = optionalString(data, 'excerpt', 140) ?? '';
    const base = {
      actorId,
      entityType: 'comment' as const,
      entityId: commentId,
      videoId,
      groupKey: `comment:${commentId}`,
      data: {
        video_id: videoId,
        comment_id: commentId,
        ...(parentId === null ? {} : { parent_id: parentId }),
      },
      grouped: false,
    };
    const drafts: Draft[] = [];
    // The author of the parent hears about a reply; the video owner hears
    // about it once, as a reply when they wrote the parent.
    if (parentAuthorId !== null) {
      drafts.push({
        ...base,
        userId: parentAuthorId,
        type: 'reply',
        render: (actor) => messages.reply(actor, excerpt),
      });
    }
    if (ownerId !== parentAuthorId) {
      drafts.push({
        ...base,
        userId: ownerId,
        type: 'comment',
        render: (actor) => messages.comment(actor, excerpt),
      });
    }
    return drafts;
  }

  private followDrafts(data: Data): Draft[] {
    const followerId = uuidField(data, 'follower_id');
    return [
      {
        userId: uuidField(data, 'following_id'),
        type: 'follow',
        actorId: followerId,
        entityType: 'user',
        entityId: followerId,
        videoId: null,
        groupKey: `follow:${followerId}`,
        data: { user_id: followerId },
        grouped: false,
        render: (actor) => messages.follow(actor),
      },
    ];
  }

  private videoReadyDrafts(data: Data): Draft[] {
    const videoId = uuidField(data, 'video_id');
    const title = optionalString(data, 'title', 100) ?? undefined;
    return [
      {
        userId: uuidField(data, 'user_id'),
        type: 'video_ready',
        actorId: null,
        entityType: 'video',
        entityId: videoId,
        videoId,
        groupKey: `video_ready:${videoId}`,
        data: { video_id: videoId },
        grouped: false,
        render: () => messages.videoReady(title),
      },
    ];
  }

  /**
   * Seller hears of a new order, buyer of its confirmation. Only cash on
   * delivery is notified at placement: a paid order is announced at payment,
   * which this service does not follow yet.
   */
  private async orderPlacedDrafts(tx: Tx, data: Data): Promise<Draft[]> {
    const order = orderParties(data);
    const amount = formatAmount(amountField(data, 'total'), currencyField(data, 'currency'));
    if (data.payment_method !== 'cash_on_delivery') {
      this.logger.log(`order ${order.orderId} awaits payment: not notified`);
      return [];
    }
    const shop = await this.shopName(tx, order.shopId);
    return [
      orderDraft(order, 'seller', 'order_new', 'placed', order.buyerId, (buyer) =>
        messages.orderNew(buyer, amount),
      ),
      orderDraft(order, 'buyer', 'order_confirmed', 'placed', null, () =>
        messages.orderConfirmed(shop, amount),
      ),
    ];
  }

  private async orderShippedDrafts(tx: Tx, data: Data): Promise<Draft[]> {
    const order = orderParties(data);
    const tracking = optionalString(data, 'tracking', 200);
    const shop = await this.shopName(tx, order.shopId);
    return [
      orderDraft(order, 'buyer', 'order_shipped', 'shipped', null, () =>
        messages.orderShipped(shop, tracking),
      ),
    ];
  }

  /** The party who cancelled is never told; a refused order never reached the seller. */
  private async orderCancelledDrafts(tx: Tx, data: Data): Promise<Draft[]> {
    const order = orderParties(data);
    const paid = data.paid === true;
    const reason = data.reason;
    if (reason === 'account_deleted') {
      // The event does not say which party was erased: both are told, and
      // write() skips the erased one. Neither is named.
      const shop = await this.shopName(tx, order.shopId);
      return [
        orderDraft(order, 'buyer', 'order_cancelled', 'cancelled', null, () =>
          messages.orderCancelledAccountDeleted(shop, paid),
        ),
        orderDraft(order, 'seller', 'order_cancelled', 'cancelled', null, () =>
          messages.orderCancelledAccountDeletedSeller(),
        ),
      ];
    }
    if (reason === 'buyer_cancelled') {
      return [
        orderDraft(order, 'seller', 'order_cancelled', 'cancelled', order.buyerId, (buyer) =>
          messages.orderCancelledByBuyer(buyer),
        ),
      ];
    }
    const render = typeof reason === 'string' ? BUYER_CANCEL_MESSAGES[reason] : undefined;
    if (render === undefined) {
      throw new PermanentError(
        'reason must be stock_rejected, payment_timeout, buyer_cancelled, seller_cancelled, account_deleted or shop_suspended',
      );
    }
    const shop = await this.shopName(tx, order.shopId);
    return [
      orderDraft(order, 'buyer', 'order_cancelled', 'cancelled', null, () => render(shop, paid)),
    ];
  }

  private orderCompletedDrafts(data: Data): Draft[] {
    const order = orderParties(data);
    const amount = formatAmount(amountField(data, 'total'), currencyField(data, 'currency'));
    return [
      orderDraft(order, 'seller', 'order_completed', 'completed', order.buyerId, (buyer) =>
        messages.orderCompleted(buyer, amount),
      ),
    ];
  }

  private async shopName(tx: Tx, shopId: string): Promise<string> {
    const shop = await tx.shopProjection.findUnique({
      where: { shopId },
      select: { name: true },
    });
    return shop?.name ?? UNKNOWN_SHOP;
  }

  /** Writes draft and returns the push to send, if any. */
  private async write(tx: Tx, draft: Draft): Promise<PendingPush | undefined> {
    if (draft.actorId === draft.userId) {
      return undefined;
    }
    // An event that arrives after an account erasure (late, or replayed from
    // the DLQ) must not write anything for or about that account again.
    const parties = draft.actorId === null ? [draft.userId] : [draft.userId, draft.actorId];
    if ((await tx.erasedUser.count({ where: { userId: { in: parties } } })) > 0) {
      return undefined;
    }
    const prefs =
      (await tx.preference.findUnique({ where: { userId: draft.userId } })) ?? DEFAULT_PREFERENCES;
    const preference = PREFERENCE_OF[draft.type];
    const pushOnly = PUSH_ONLY_PREFERENCES.has(preference);
    if (!pushOnly && !prefs[preference]) {
      return undefined;
    }
    const actor = actorName(
      draft.actorId === null
        ? null
        : await tx.userProjection.findUnique({
            where: { userId: draft.actorId },
            select: { displayName: true, username: true },
          }),
    );
    const now = new Date();
    const id = uuidv7(now.getTime());
    const message = draft.render(actor, 0);
    const { count } = await tx.notification.createMany({
      data: [
        {
          id,
          userId: draft.userId,
          type: draft.type,
          title: message.title,
          body: message.body,
          data: draft.data,
          actorId: draft.actorId,
          actorCount: 1,
          entityType: draft.entityType,
          entityId: draft.entityId,
          videoId: draft.videoId,
          groupKey: draft.groupKey,
          createdAt: now,
          lastActivityAt: now,
        },
      ],
      skipDuplicates: true,
    });
    if (count === 1) {
      if (draft.grouped && draft.actorId !== null) {
        await tx.notificationActor.create({
          data: { notificationId: id, actorId: draft.actorId, createdAt: now },
        });
      }
      this.metrics.notificationCreated(draft.type);
      if (!prefs.pushEnabled || !prefs[preference]) {
        return undefined;
      }
      return {
        userId: draft.userId,
        message: {
          ...message,
          data: { ...draft.data, notification_id: id, type: draft.type },
        },
      };
    }
    if (draft.grouped && draft.actorId !== null) {
      await this.addActor(tx, draft, draft.actorId, actor, now);
    }
    return undefined;
  }

  /**
   * Folds one more actor into an existing group: the newest actor is named,
   * the group becomes unread again and moves to the top. No second push.
   */
  private async addActor(
    tx: Tx,
    draft: Draft,
    actorId: string,
    actor: string,
    now: Date,
  ): Promise<void> {
    const existing = await tx.notification.findUnique({
      where: {
        userId_groupKey: { userId: draft.userId, groupKey: draft.groupKey },
      },
      select: { id: true },
    });
    if (existing === null) {
      return;
    }
    const added = await tx.notificationActor.createMany({
      data: [{ notificationId: existing.id, actorId, createdAt: now }],
      skipDuplicates: true,
    });
    if (added.count === 0) {
      return;
    }
    const actorCount = await tx.notificationActor.count({
      where: { notificationId: existing.id },
    });
    const message = draft.render(actor, actorCount - 1);
    await tx.notification.update({
      where: { id: existing.id },
      data: {
        title: message.title,
        body: message.body,
        actorId,
        actorCount,
        readAt: null,
        lastActivityAt: now,
      },
    });
  }

  /** Forgets every notification about removed content; a restore brings none back. */
  private async contentRemoved(tx: Tx, data: Data): Promise<void> {
    const targetId = uuidField(data, 'target_id');
    switch (data.target_type) {
      case 'video':
        await tx.notification.deleteMany({ where: { videoId: targetId } });
        return;
      case 'comment':
        await tx.notification.deleteMany({ where: { entityType: 'comment', entityId: targetId } });
        return;
      default:
        throw new PermanentError('target_type must be video or comment');
    }
  }

  private async userCreated(tx: Tx, data: Data): Promise<void> {
    const userId = uuidField(data, 'user_id');
    const updatedAt = dateField(data, 'created_at');
    // A profile snapshot may already be here: it is newer, keep it. An erased
    // account gets no projection back.
    await tx.$executeRaw`
      INSERT INTO user_projections (user_id, updated_at)
      SELECT ${userId}::uuid, ${updatedAt}
       WHERE NOT EXISTS (SELECT 1 FROM erased_users WHERE user_id = ${userId}::uuid)
      ON CONFLICT (user_id) DO NOTHING`;
  }

  private async shopUpdated(tx: Tx, data: Data): Promise<void> {
    const shopId = uuidField(data, 'shop_id');
    const name = requiredString(data, 'name', 60);
    const handle = requiredString(data, 'handle', 30);
    const updatedAt = dateField(data, 'updated_at');
    await tx.$executeRaw`
      INSERT INTO shop_projections (shop_id, name, handle, updated_at)
      VALUES (${shopId}::uuid, ${name}, ${handle}, ${updatedAt})
      ON CONFLICT (shop_id) DO UPDATE
        SET name = EXCLUDED.name,
            handle = EXCLUDED.handle,
            updated_at = EXCLUDED.updated_at
        WHERE shop_projections.updated_at < EXCLUDED.updated_at`;
  }

  private async profileUpdated(tx: Tx, data: Data): Promise<void> {
    const userId = uuidField(data, 'user_id');
    const updatedAt = dateField(data, 'updated_at');
    const username = optionalString(data, 'username', 30);
    const displayName = optionalString(data, 'display_name', 50);
    const avatarUrl = optionalString(data, 'avatar_url', 2048);
    await tx.$executeRaw`
      INSERT INTO user_projections (user_id, username, display_name, avatar_url, updated_at)
      SELECT ${userId}::uuid, ${username}, ${displayName}, ${avatarUrl}, ${updatedAt}
       WHERE NOT EXISTS (SELECT 1 FROM erased_users WHERE user_id = ${userId}::uuid)
      ON CONFLICT (user_id) DO UPDATE
        SET username = EXCLUDED.username,
            display_name = EXCLUDED.display_name,
            avatar_url = EXCLUDED.avatar_url,
            updated_at = EXCLUDED.updated_at
        WHERE user_projections.updated_at < EXCLUDED.updated_at`;
  }
}
