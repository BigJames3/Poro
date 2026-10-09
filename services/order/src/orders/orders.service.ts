import { Injectable } from '@nestjs/common';

import { ApiError } from '../common/api-error';
import { decodeCursor, encodeCursor, pageSize } from '../common/cursor';
import { cleanLine } from '../common/text';
import type { CancelReason } from '../events/catalog';
import { Prisma } from '../generated/prisma/client';
import { PrismaService } from '../prisma/prisma.service';
import {
  OrderListQueryDto,
  OrderStatus,
  OrderView,
  OrderWithLines,
  Page,
  toOrderView,
} from './order.dto';
import { lockOrder, publishCancelled, publishCompleted, publishShipped } from './publish';

/** Statuses an order can still be cancelled from: nothing has left the shop yet. */
export const CANCELLABLE: OrderStatus[] = ['pending', 'awaiting_payment', 'confirmed', 'paid'];
const SHIPPABLE: OrderStatus[] = ['confirmed', 'paid'];

type Side = 'buyer' | 'seller';
type Tx = Prisma.TransactionClient;

function statusConflict(order: OrderWithLines): ApiError {
  return new ApiError(409, 'order_status_conflict', `the order is ${order.status}`);
}

@Injectable()
export class OrdersService {
  constructor(private readonly prisma: PrismaService) {}

  async list(side: Side, userId: string, query: OrderListQueryDto): Promise<Page<OrderView>> {
    const where: Prisma.OrderWhereInput =
      side === 'buyer' ? { buyerId: userId } : { sellerId: userId };
    if (query.status !== undefined) {
      where.status = query.status;
    }
    const limit = pageSize(query.limit);
    const after = decodeCursor(query.cursor);
    const orders = await this.prisma.order.findMany({
      where: after
        ? {
            AND: [
              where,
              {
                OR: [
                  { createdAt: { lt: after.createdAt } },
                  { createdAt: after.createdAt, id: { lt: after.id } },
                ],
              },
            ],
          }
        : where,
      orderBy: [{ createdAt: 'desc' }, { id: 'desc' }],
      take: limit + 1,
      include: { lines: true },
    });
    const items = orders.slice(0, limit);
    const last = items.at(-1);
    return {
      items: items.map(toOrderView),
      next_cursor:
        orders.length > limit && last
          ? encodeCursor({ createdAt: last.createdAt, id: last.id })
          : null,
    };
  }

  async get(side: Side, userId: string, orderId: string): Promise<OrderView> {
    const order = await this.prisma.order.findUnique({
      where: { id: orderId },
      include: { lines: true },
    });
    if (!order || (side === 'buyer' ? order.buyerId : order.sellerId) !== userId) {
      throw new ApiError(404, 'order_not_found', 'order not found');
    }
    return toOrderView(order);
  }

  /** The buyer or the seller cancels before shipping; a paid order gets refunded. */
  async cancel(side: Side, userId: string, orderId: string): Promise<OrderView> {
    const reason: CancelReason = side === 'buyer' ? 'buyer_cancelled' : 'seller_cancelled';
    return this.act(side, userId, orderId, async (tx, order, now) => {
      if (order.status === 'cancelled') {
        return order;
      }
      if (!CANCELLABLE.includes(order.status as OrderStatus)) {
        throw statusConflict(order);
      }
      return cancelOrder(tx, order, reason, now);
    });
  }

  async ship(
    sellerId: string,
    orderId: string,
    rawTracking: string | null | undefined,
  ): Promise<OrderView> {
    const tracking =
      rawTracking === undefined || rawTracking === null || rawTracking.trim() === ''
        ? null
        : cleanLine(rawTracking, 200, 'tracking_invalid', 'tracking');
    return this.act('seller', sellerId, orderId, async (tx, order, now) => {
      if (!SHIPPABLE.includes(order.status as OrderStatus)) {
        throw statusConflict(order);
      }
      const shipped = await tx.order.update({
        where: { id: order.id },
        data: { status: 'shipped', shippedAt: now, tracking },
        include: { lines: true },
      });
      await publishShipped(tx, shipped, now);
      return shipped;
    });
  }

  async confirmReceipt(buyerId: string, orderId: string): Promise<OrderView> {
    return this.act('buyer', buyerId, orderId, async (tx, order, now) => {
      if (order.status === 'completed') {
        return order;
      }
      if (order.status !== 'shipped') {
        throw statusConflict(order);
      }
      return completeOrder(tx, order, now);
    });
  }

  private async act(
    side: Side,
    userId: string,
    orderId: string,
    change: (tx: Tx, order: OrderWithLines, now: Date) => Promise<OrderWithLines>,
  ): Promise<OrderView> {
    const order = await this.prisma.$transaction(async (tx) => {
      const current = await lockOrder(tx, orderId);
      if (!current || (side === 'buyer' ? current.buyerId : current.sellerId) !== userId) {
        throw new ApiError(404, 'order_not_found', 'order not found');
      }
      return change(tx, current, new Date());
    });
    return toOrderView(order);
  }
}

export async function cancelOrder(
  tx: Tx,
  order: OrderWithLines,
  reason: CancelReason,
  now: Date,
): Promise<OrderWithLines> {
  const cancelled = await tx.order.update({
    where: { id: order.id },
    data: { status: 'cancelled', cancelReason: reason, cancelledAt: now, expiresAt: null },
    include: { lines: true },
  });
  await publishCancelled(tx, cancelled, reason, cancelled.paid, now);
  return cancelled;
}

export async function completeOrder(
  tx: Tx,
  order: OrderWithLines,
  now: Date,
): Promise<OrderWithLines> {
  const completed = await tx.order.update({
    where: { id: order.id },
    data: { status: 'completed', completedAt: now },
    include: { lines: true },
  });
  await publishCompleted(tx, completed, now);
  return completed;
}
