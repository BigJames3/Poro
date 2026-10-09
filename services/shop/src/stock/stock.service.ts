import { Injectable, Logger } from '@nestjs/common';

import {
  TYPE_STOCK_REJECTED,
  TYPE_STOCK_RESERVED,
  type Currency,
  type OrderCancelledV1,
  type OrderCompletedV1,
  type OrderCreatedV1,
  type ReservedLineV1,
  type StockRejectedReason,
  type StockRejectedV1,
  type StockReservedV1,
} from '../events/catalog';
import { newEnvelope } from '../events/envelope';
import { PermanentError } from '../events/event-consumer';
import { claim, enqueue } from '../events/outbox';
import { Prisma } from '../generated/prisma/client';
import { MediaService } from '../media/media.service';
import { PrismaService } from '../prisma/prisma.service';
import { available, publishProduct } from '../products/publish';

export const STOCK_CONSUMER_GROUP = 'poro-shop-stock';
const RESERVED_TITLE_MAX = 200;

type Tx = Prisma.TransactionClient;

interface Need {
  productId: string;
  quantity: number;
}

/**
 * Locks the products, then the variants, each in id order: the same order as
 * the seller routes, so a reservation never deadlocks with an edit.
 */
async function lockStock(tx: Tx, variantIds: string[]): Promise<void> {
  const rows = await tx.$queryRaw<{ product_id: string }[]>`
    SELECT DISTINCT product_id::text FROM variants WHERE id = ANY(${variantIds}::uuid[])`;
  const productIds = rows.map((row) => row.product_id);
  await tx.$queryRaw`
    SELECT 1 FROM products WHERE id = ANY(${productIds}::uuid[]) ORDER BY id FOR UPDATE`;
  await tx.$queryRaw`
    SELECT 1 FROM variants WHERE id = ANY(${variantIds}::uuid[]) ORDER BY id FOR UPDATE`;
}

function reservedLines(lines: Prisma.JsonValue): ReservedLineV1[] {
  return lines as unknown as ReservedLineV1[];
}

/**
 * The stock side of the order saga. Each order gets exactly one answer:
 * stock.reserved with authoritative prices, or stock.rejected. A cancellation
 * releases held stock; a completion takes it out of the shop for good.
 */
@Injectable()
export class StockService {
  private readonly logger = new Logger(StockService.name);

  constructor(
    private readonly prisma: PrismaService,
    private readonly media: MediaService,
  ) {}

  async reserve(eventId: string, order: OrderCreatedV1): Promise<void> {
    await this.prisma.$transaction(async (tx) => {
      if (!(await claim(tx, STOCK_CONSUMER_GROUP, eventId))) {
        return;
      }
      // A second order.created for the same order, or a cancellation that
      // arrived first, already has its answer.
      if (await tx.reservation.findUnique({ where: { orderId: order.order_id } })) {
        return;
      }
      const reject = (reason: StockRejectedReason, variantIds: string[] = []): Promise<void> =>
        this.reject(tx, order, reason, variantIds);

      const shop = await tx.shop.findUnique({ where: { id: order.shop_id } });
      if (!shop || shop.status !== 'active' || shop.ownerId !== order.seller_id) {
        return reject('shop_unavailable');
      }
      if (order.currency !== shop.currency) {
        return reject('currency_mismatch');
      }

      const needs = new Map<string, Need>();
      const mismatched = new Set<string>();
      for (const line of order.items) {
        const need = needs.get(line.variant_id);
        if (need && need.productId !== line.product_id) {
          mismatched.add(line.variant_id);
        }
        needs.set(line.variant_id, {
          productId: line.product_id,
          quantity: (need?.quantity ?? 0) + line.quantity,
        });
      }
      const variantIds = [...needs.keys()];
      await lockStock(tx, variantIds);
      const variants = await tx.variant.findMany({
        where: { id: { in: variantIds } },
        include: { product: true },
      });
      const byId = new Map(variants.map((variant) => [variant.id, variant]));

      const unavailable = variantIds.filter((id) => {
        const variant = byId.get(id);
        return (
          mismatched.has(id) ||
          !variant ||
          variant.deletedAt !== null ||
          variant.product.deletedAt !== null ||
          variant.product.status !== 'active' ||
          variant.product.shopId !== shop.id ||
          variant.productId !== needs.get(id)?.productId
        );
      });
      if (unavailable.length > 0) {
        return reject('product_unavailable', unavailable);
      }
      const short = variantIds.filter((id) => {
        const variant = byId.get(id);
        return !variant || available(variant) < (needs.get(id)?.quantity ?? 0);
      });
      if (short.length > 0) {
        return reject('out_of_stock', short);
      }

      const lines: ReservedLineV1[] = [];
      const flipped = new Set<string>();
      let subtotal = 0n;
      for (const id of variantIds) {
        const variant = byId.get(id);
        const quantity = needs.get(id)?.quantity ?? 0;
        if (!variant) {
          continue;
        }
        await tx.variant.update({
          where: { id },
          data: { stockReserved: { increment: quantity } },
        });
        if (available(variant) - quantity <= 0) {
          flipped.add(variant.productId);
        }
        subtotal += variant.price * BigInt(quantity);
        lines.push({
          product_id: variant.productId,
          variant_id: id,
          quantity,
          unit_price: Number(variant.price),
          title: `${variant.product.title} — ${variant.title}`.slice(0, RESERVED_TITLE_MAX),
        });
      }
      const now = new Date();
      await tx.reservation.create({
        data: {
          orderId: order.order_id,
          shopId: shop.id,
          status: 'held',
          currency: shop.currency,
          subtotal,
          lines: lines as unknown as Prisma.InputJsonArray,
        },
      });
      const data: StockReservedV1 = {
        order_id: order.order_id,
        shop_id: shop.id,
        currency: shop.currency as Currency,
        items: lines,
        subtotal: Number(subtotal),
        reserved_at: now.toISOString(),
      };
      await enqueue(tx, newEnvelope(TYPE_STOCK_RESERVED, 1, order.order_id, data, now));
      for (const productId of [...flipped].sort()) {
        await publishProduct(tx, productId, this.url);
      }
    });
  }

  async release(eventId: string, order: OrderCancelledV1): Promise<void> {
    await this.prisma.$transaction(async (tx) => {
      if (!(await claim(tx, STOCK_CONSUMER_GROUP, eventId))) {
        return;
      }
      const reservation = await tx.reservation.findUnique({ where: { orderId: order.order_id } });
      if (!reservation) {
        // The cancellation overtook order.created: remember it so the late
        // order.created holds nothing.
        await tx.reservation.create({
          data: { orderId: order.order_id, shopId: order.shop_id, status: 'cancelled' },
        });
        return;
      }
      if (reservation.status !== 'held') {
        return;
      }
      const lines = reservedLines(reservation.lines);
      await lockStock(
        tx,
        lines.map((line) => line.variant_id),
      );
      const flipped = new Set<string>();
      for (const line of lines) {
        const after = await tx.variant.update({
          where: { id: line.variant_id },
          data: { stockReserved: { decrement: line.quantity } },
        });
        // Back in stock: nothing was available before this release.
        if (available(after) <= line.quantity) {
          flipped.add(line.product_id);
        }
      }
      await tx.reservation.update({
        where: { orderId: order.order_id },
        data: { status: 'released', reason: order.reason.slice(0, 32) },
      });
      for (const productId of [...flipped].sort()) {
        await publishProduct(tx, productId, this.url);
      }
    });
  }

  async consume(eventId: string, order: OrderCompletedV1): Promise<void> {
    await this.prisma.$transaction(async (tx) => {
      if (!(await claim(tx, STOCK_CONSUMER_GROUP, eventId))) {
        return;
      }
      const reservation = await tx.reservation.findUnique({ where: { orderId: order.order_id } });
      if (reservation?.status === 'consumed') {
        return;
      }
      if (reservation?.status !== 'held') {
        throw new PermanentError(
          `order ${order.order_id} completed without held stock (${reservation?.status ?? 'none'})`,
        );
      }
      const lines = reservedLines(reservation.lines);
      await lockStock(
        tx,
        lines.map((line) => line.variant_id),
      );
      // Available stock does not change: both counters drop by the same amount.
      for (const line of lines) {
        await tx.variant.update({
          where: { id: line.variant_id },
          data: {
            stockOnHand: { decrement: line.quantity },
            stockReserved: { decrement: line.quantity },
          },
        });
      }
      await tx.reservation.update({
        where: { orderId: order.order_id },
        data: { status: 'consumed' },
      });
      this.logger.log({ msg: 'stock consumed', order_id: order.order_id });
    });
  }

  private readonly url = (key: string): string => this.media.publicUrl(key);

  private async reject(
    tx: Tx,
    order: OrderCreatedV1,
    reason: StockRejectedReason,
    variantIds: string[],
  ): Promise<void> {
    const now = new Date();
    await tx.reservation.create({
      data: { orderId: order.order_id, shopId: order.shop_id, status: 'rejected', reason },
    });
    const data: StockRejectedV1 = {
      order_id: order.order_id,
      shop_id: order.shop_id,
      reason,
      variant_ids: variantIds,
      rejected_at: now.toISOString(),
    };
    await enqueue(tx, newEnvelope(TYPE_STOCK_REJECTED, 1, order.order_id, data, now));
  }
}
