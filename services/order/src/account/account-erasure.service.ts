import { Injectable } from '@nestjs/common';

import { claim } from '../events/outbox';
import { Prisma } from '../generated/prisma/client';
import { PrismaService } from '../prisma/prisma.service';
import { CANCELLABLE, cancelOrder } from '../orders/orders.service';
import { lockOrder } from '../orders/publish';
import { eraseOrderContacts } from '../orders/saga.service';

export const ACCOUNT_ERASURE_GROUP = 'poro-order-account-erasure';

/**
 * Erases what order holds about a deleted account (ADR-0018), in one
 * transaction with the inbox claim:
 * - the address book, the cart and the checkout previews are deleted;
 * - every order not shipped yet where the account buys or sells is cancelled
 *   with reason account_deleted: shop releases the stock and payment refunds
 *   the paid ones;
 * - the delivery contact of every ended order it bought is erased now.
 * Orders keep their ids, amounts, lines and statuses (accounting). Shipped
 * orders complete as usual; the retention task erases their contact then.
 * Checkouts are kept: each one is referenced by its orders and holds only the
 * buyer id, the preview id and a hash of the cart lines, no contact.
 */
@Injectable()
export class AccountErasureService {
  constructor(private readonly prisma: PrismaService) {}

  async erase(eventId: string, userId: string): Promise<void> {
    await this.prisma.$transaction(async (tx) => {
      if (!(await claim(tx, ACCOUNT_ERASURE_GROUP, eventId))) {
        return;
      }
      const now = new Date();
      await tx.address.deleteMany({ where: { buyerId: userId } });
      await tx.cartItem.deleteMany({ where: { buyerId: userId } });
      await tx.checkoutPreview.deleteMany({ where: { buyerId: userId } });

      // Locked in id order so two erasures sharing orders never deadlock.
      const open = await tx.$queryRaw<{ id: string }[]>`
        SELECT id::text FROM orders
        WHERE (buyer_id = ${userId}::uuid OR seller_id = ${userId}::uuid)
          AND status IN (${Prisma.join(CANCELLABLE)})
        ORDER BY id FOR UPDATE`;
      for (const { id } of open) {
        const order = await lockOrder(tx, id);
        if (order) {
          await cancelOrder(tx, order, 'account_deleted', now);
        }
      }

      await eraseOrderContacts(
        tx,
        now,
        Prisma.sql`buyer_id = ${userId}::uuid AND status IN ('completed', 'cancelled')`,
      );
    });
  }
}
