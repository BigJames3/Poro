import { Inject, Injectable, Logger } from '@nestjs/common';

import { appConfig, type AppConfigType } from '../config/app.config';
import type { CancelReason } from '../events/catalog';
import { PermanentError } from '../events/event-consumer';
import { Data, amountField, arrayField, stringField, uuidField } from '../events/fields';
import { claim } from '../events/outbox';
import { Prisma } from '../generated/prisma/client';
import { PrismaService } from '../prisma/prisma.service';
import { OrderWithLines } from './order.dto';
import { cancelOrder, completeOrder } from './orders.service';
import { lockOrder, publishCancelled, publishPlaced } from './publish';

export const SAGA_CONSUMER_GROUP = 'poro-order-saga';
const BATCH = 100;
const DAY_MS = 86_400_000;

type Tx = Prisma.TransactionClient;

/**
 * Moves orders along on the answers of shop and payment, and on time:
 * unpaid orders expire, shipped orders complete, old contacts are erased.
 */
@Injectable()
export class SagaService {
  private readonly logger = new Logger(SagaService.name);

  constructor(
    private readonly prisma: PrismaService,
    @Inject(appConfig.KEY) private readonly config: AppConfigType,
  ) {}

  /** The shop holds the stock: fix the authoritative prices and ask for payment. */
  async stockReserved(eventId: string, data: Data): Promise<void> {
    const orderId = uuidField(data, 'order_id');
    const subtotal = amountField(data, 'subtotal');
    const currency = stringField(data, 'currency');
    const prices = new Map(
      arrayField(data, 'items').map((item) => [
        uuidField(item, 'variant_id'),
        { unitPrice: amountField(item, 'unit_price'), title: stringField(item, 'title') },
      ]),
    );
    await this.onOrder(eventId, orderId, async (tx, order, now) => {
      if (order.status !== 'pending') {
        return;
      }
      if (currency !== order.currency || order.lines.some((line) => !prices.has(line.variantId))) {
        throw new PermanentError(`stock.reserved does not match order ${orderId}`);
      }
      for (const line of order.lines) {
        const price = prices.get(line.variantId);
        await tx.orderLine.update({
          where: { orderId_position: { orderId, position: line.position } },
          data: { unitPrice: price?.unitPrice, title: price?.title.slice(0, 200) },
        });
      }
      const cashOnDelivery = order.paymentMethod === 'cash_on_delivery';
      const placed = await tx.order.update({
        where: { id: orderId },
        data: {
          status: cashOnDelivery ? 'confirmed' : 'awaiting_payment',
          total: subtotal,
          placedAt: now,
          expiresAt: cashOnDelivery
            ? null
            : new Date(now.getTime() + this.config.paymentTimeoutMinutes * 60_000),
        },
        include: { lines: true },
      });
      await publishPlaced(tx, placed, now);
    });
  }

  async stockRejected(eventId: string, data: Data): Promise<void> {
    const orderId = uuidField(data, 'order_id');
    await this.onOrder(eventId, orderId, async (tx, order, now) => {
      if (order.status === 'pending') {
        await cancelOrder(tx, order, 'stock_rejected', now);
      }
    });
  }

  async paymentSucceeded(eventId: string, data: Data): Promise<void> {
    const orderId = uuidField(data, 'order_id');
    const amount = amountField(data, 'amount');
    const currency = stringField(data, 'currency');
    await this.onOrder(eventId, orderId, async (tx, order, now) => {
      if (order.paid) {
        return;
      }
      if (amount !== order.total || currency !== order.currency) {
        throw new PermanentError(`payment of order ${orderId} does not match its total`);
      }
      if (order.status === 'awaiting_payment') {
        await tx.order.update({
          where: { id: orderId },
          data: {
            status: 'paid',
            paid: true,
            paidAt: now,
            expiresAt: null,
            lastPaymentError: null,
          },
        });
        return;
      }
      if (order.status === 'cancelled') {
        // The money arrived after the order ended: announce it paid so payment refunds it.
        const paid = await tx.order.update({
          where: { id: orderId },
          data: { paid: true, paidAt: now },
          include: { lines: true },
        });
        await publishCancelled(
          tx,
          paid,
          (order.cancelReason ?? 'payment_timeout') as CancelReason,
          true,
          now,
        );
        this.logger.warn({
          msg: 'payment after cancellation, refund requested',
          order_id: orderId,
        });
        return;
      }
      throw new PermanentError(`order ${orderId} cannot be paid while ${order.status}`);
    });
  }

  async paymentFailed(eventId: string, data: Data): Promise<void> {
    const orderId = uuidField(data, 'order_id');
    const reason = stringField(data, 'reason').slice(0, 32);
    await this.onOrder(eventId, orderId, async (tx, order) => {
      if (order.status === 'awaiting_payment') {
        await tx.order.update({ where: { id: orderId }, data: { lastPaymentError: reason } });
      }
    });
  }

  async refundSucceeded(eventId: string, data: Data): Promise<void> {
    const orderId = uuidField(data, 'order_id');
    await this.onOrder(eventId, orderId, async (tx, order, now) => {
      if (order.refundedAt === null) {
        await tx.order.update({ where: { id: orderId }, data: { refundedAt: now } });
      }
    });
  }

  /** Cancels orders whose payment window closed. Safe on several replicas. */
  async expireUnpaid(now: Date = new Date()): Promise<number> {
    return this.sweep(
      Prisma.sql`status = 'awaiting_payment' AND expires_at <= ${now}`,
      (tx, order) => cancelOrder(tx, order, 'payment_timeout', now),
    );
  }

  /** Completes orders shipped longer ago than AUTO_COMPLETE_DAYS. */
  async autoComplete(now: Date = new Date()): Promise<number> {
    const before = new Date(now.getTime() - this.config.autoCompleteDays * DAY_MS);
    return this.sweep(Prisma.sql`status = 'shipped' AND shipped_at <= ${before}`, (tx, order) =>
      completeOrder(tx, order, now),
    );
  }

  /** Erases delivery contacts of orders that ended longer ago than CONTACT_RETENTION_DAYS. */
  async eraseContacts(now: Date = new Date()): Promise<number> {
    const before = new Date(now.getTime() - this.config.contactRetentionDays * DAY_MS);
    return this.prisma.$executeRaw`
      UPDATE orders SET contact_name = NULL, contact_phone = NULL, city = NULL, address = NULL,
                        contact_erased_at = ${now}, updated_at = ${now}
      WHERE contact_erased_at IS NULL
        AND ((status = 'completed' AND completed_at <= ${before})
          OR (status = 'cancelled' AND cancelled_at <= ${before}))`;
  }

  private async sweep(
    condition: Prisma.Sql,
    change: (tx: Tx, order: OrderWithLines) => Promise<unknown>,
  ): Promise<number> {
    return this.prisma.$transaction(async (tx) => {
      const rows = await tx.$queryRaw<{ id: string }[]>`
        SELECT id::text FROM orders WHERE ${condition}
        ORDER BY id LIMIT ${BATCH} FOR UPDATE SKIP LOCKED`;
      for (const { id } of rows) {
        const order = await tx.order.findUniqueOrThrow({ where: { id }, include: { lines: true } });
        await change(tx, order);
      }
      return rows.length;
    });
  }

  /** Applies change once per event to a locked order; an unknown order is a bug upstream. */
  private async onOrder(
    eventId: string,
    orderId: string,
    change: (tx: Tx, order: OrderWithLines, now: Date) => Promise<void>,
  ): Promise<void> {
    await this.prisma.$transaction(async (tx) => {
      if (!(await claim(tx, SAGA_CONSUMER_GROUP, eventId))) {
        return;
      }
      const order = await lockOrder(tx, orderId);
      if (!order) {
        throw new PermanentError(`unknown order ${orderId}`);
      }
      await change(tx, order, new Date());
    });
  }
}
