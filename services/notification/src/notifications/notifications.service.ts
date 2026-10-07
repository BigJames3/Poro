import { Injectable } from '@nestjs/common';

import { ApiError } from '../common/api-error';
import { isUuid, uuidv7 } from '../common/uuid';
import { PrismaService } from '../prisma/prisma.service';
import { decodeCursor, encodeCursor } from './cursor';
import {
  DEFAULT_PAGE_SIZE,
  type DeviceView,
  type NotificationPage,
  type NotificationView,
  type Platform,
  type PreferencesView,
  type UpdatePreferencesDto,
} from './notification.dto';
import { DEFAULT_PREFERENCES, type NotificationType, type Preferences } from './notification.types';

/** An account keeps its most recently registered devices only. */
export const MAX_DEVICES_PER_USER = 10;

function notFound(): ApiError {
  return new ApiError(404, 'notification_not_found', 'notification not found');
}

function preferencesView(prefs: Preferences): PreferencesView {
  return {
    push_enabled: prefs.pushEnabled,
    likes: prefs.likes,
    comments: prefs.comments,
    follows: prefs.follows,
    video_ready: prefs.videoReady,
  };
}

@Injectable()
export class NotificationsService {
  constructor(private readonly prisma: PrismaService) {}

  /** Newest activity first. A group that gains an actor moves to the top. */
  async list(
    userId: string,
    cursor?: string,
    limit = DEFAULT_PAGE_SIZE,
  ): Promise<NotificationPage> {
    const after = cursor === undefined ? undefined : decodeCursor(cursor);
    const rows = await this.prisma.notification.findMany({
      where: {
        userId,
        ...(after === undefined
          ? {}
          : {
              OR: [
                { lastActivityAt: { lt: after.at } },
                { lastActivityAt: after.at, id: { lt: after.id } },
              ],
            }),
      },
      orderBy: [{ lastActivityAt: 'desc' }, { id: 'desc' }],
      take: limit + 1,
    });
    const page = rows.slice(0, limit);
    const actorIds = [
      ...new Set(page.flatMap((row) => (row.actorId === null ? [] : [row.actorId]))),
    ];
    const actors = new Map(
      (
        await this.prisma.userProjection.findMany({
          where: { userId: { in: actorIds } },
        })
      ).map((profile) => [profile.userId, profile]),
    );
    const items = page.map((row): NotificationView => {
      const profile = row.actorId === null ? undefined : actors.get(row.actorId);
      return {
        id: row.id,
        type: row.type as NotificationType,
        title: row.title,
        body: row.body,
        data: row.data as Record<string, string>,
        actor:
          row.actorId === null
            ? null
            : {
                id: row.actorId,
                username: profile?.username ?? null,
                display_name: profile?.displayName ?? null,
                avatar_url: profile?.avatarUrl ?? null,
              },
        actor_count: row.actorCount,
        entity_type: row.entityType,
        entity_id: row.entityId,
        read: row.readAt !== null,
        read_at: row.readAt?.toISOString() ?? null,
        created_at: row.createdAt.toISOString(),
        last_activity_at: row.lastActivityAt.toISOString(),
      };
    });
    const last = page.at(-1);
    return {
      items,
      next_cursor:
        rows.length > limit && last !== undefined
          ? encodeCursor({ at: last.lastActivityAt, id: last.id })
          : null,
    };
  }

  async unreadCount(userId: string): Promise<{ count: number }> {
    return {
      count: await this.prisma.notification.count({
        where: { userId, readAt: null },
      }),
    };
  }

  /** Idempotent: reading twice keeps the first read_at. */
  async markRead(userId: string, id: string): Promise<{ id: string; read_at: string }> {
    if (!isUuid(id)) {
      throw notFound();
    }
    const notificationId = id.toLowerCase();
    await this.prisma.notification.updateMany({
      where: { id: notificationId, userId, readAt: null },
      data: { readAt: new Date() },
    });
    const row = await this.prisma.notification.findFirst({
      where: { id: notificationId, userId },
      select: { readAt: true },
    });
    if (row?.readAt == null) {
      throw notFound();
    }
    return { id: notificationId, read_at: row.readAt.toISOString() };
  }

  async markAllRead(userId: string): Promise<{ updated: number }> {
    const { count } = await this.prisma.notification.updateMany({
      where: { userId, readAt: null },
      data: { readAt: new Date() },
    });
    return { updated: count };
  }

  /**
   * Registers token for userId. A token registered by another account moves
   * to this one: the device changed hands. Beyond MAX_DEVICES_PER_USER the
   * least recently registered devices are dropped.
   */
  async registerDevice(userId: string, token: string, platform: Platform): Promise<DeviceView> {
    return this.prisma.$transaction(async (tx) => {
      const device = await tx.device.upsert({
        where: { fcmToken: token },
        create: { id: uuidv7(), userId, fcmToken: token, platform },
        update: { userId, platform },
      });
      const stale = await tx.device.findMany({
        where: { userId },
        orderBy: [{ updatedAt: 'desc' }, { id: 'desc' }],
        skip: MAX_DEVICES_PER_USER,
        select: { id: true },
      });
      if (stale.length > 0) {
        await tx.device.deleteMany({
          where: { id: { in: stale.map((row) => row.id) } },
        });
      }
      return {
        token: device.fcmToken,
        platform: device.platform as Platform,
        created_at: device.createdAt.toISOString(),
        updated_at: device.updatedAt.toISOString(),
      };
    });
  }

  /** Idempotent, and silent about tokens of other accounts. */
  async removeDevice(userId: string, token: string): Promise<void> {
    await this.prisma.device.deleteMany({ where: { userId, fcmToken: token } });
  }

  async preferences(userId: string): Promise<PreferencesView> {
    const row = await this.prisma.preference.findUnique({ where: { userId } });
    return preferencesView(row ?? DEFAULT_PREFERENCES);
  }

  async updatePreferences(userId: string, dto: UpdatePreferencesDto): Promise<PreferencesView> {
    const changes = {
      pushEnabled: dto.push_enabled,
      likes: dto.likes,
      comments: dto.comments,
      follows: dto.follows,
      videoReady: dto.video_ready,
    };
    const row = await this.prisma.preference.upsert({
      where: { userId },
      create: { userId, ...DEFAULT_PREFERENCES, ...stripUndefined(changes) },
      update: changes,
    });
    return preferencesView(row);
  }
}

function stripUndefined<T extends object>(value: T): Partial<T> {
  return Object.fromEntries(
    Object.entries(value).filter(([, field]) => field !== undefined),
  ) as Partial<T>;
}
