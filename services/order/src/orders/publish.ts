import {
  TYPE_ORDER_CANCELLED,
  TYPE_ORDER_COMPLETED,
  TYPE_ORDER_CREATED,
  TYPE_ORDER_PLACED,
  TYPE_ORDER_SHIPPED,
  type CancelReason,
  type OrderCancelledV1,
  type OrderCompletedV1,
  type OrderCreatedV1,
  type OrderPlacedV1,
  type OrderShippedV1,
} from '../events/catalog';
import { newEnvelope } from '../events/envelope';
import { enqueue } from '../events/outbox';
import { Prisma } from '../generated/prisma/client';
import { OrderWithLines } from './order.dto';

type Tx = Prisma.TransactionClient;

function parties(order: OrderWithLines): {
  order_id: string;
  buyer_id: string;
  shop_id: string;
  seller_id: string;
} {
  return {
    order_id: order.id,
    buyer_id: order.buyerId,
    shop_id: order.shopId,
    seller_id: order.sellerId,
  };
}

export async function publishCreated(tx: Tx, order: OrderWithLines): Promise<void> {
  const data: OrderCreatedV1 = {
    ...parties(order),
    currency: order.currency,
    payment_method: order.paymentMethod,
    items: [...order.lines]
      .sort((a, b) => a.position - b.position)
      .map((line) => ({
        product_id: line.productId,
        variant_id: line.variantId,
        quantity: line.quantity,
        unit_price: Number(line.unitPriceSeen),
      })),
    created_at: order.createdAt.toISOString(),
  };
  await enqueue(tx, newEnvelope(TYPE_ORDER_CREATED, 1, order.id, data, order.createdAt));
}

export async function publishPlaced(tx: Tx, order: OrderWithLines, at: Date): Promise<void> {
  const data: OrderPlacedV1 = {
    ...parties(order),
    currency: order.currency,
    total: Number(order.total ?? 0n),
    payment_method: order.paymentMethod,
    placed_at: at.toISOString(),
    expires_at: order.expiresAt?.toISOString() ?? null,
  };
  await enqueue(tx, newEnvelope(TYPE_ORDER_PLACED, 1, order.id, data, at));
}

export async function publishCancelled(
  tx: Tx,
  order: OrderWithLines,
  reason: CancelReason,
  paid: boolean,
  at: Date,
): Promise<void> {
  const data: OrderCancelledV1 = {
    ...parties(order),
    reason,
    paid,
    cancelled_at: at.toISOString(),
  };
  await enqueue(tx, newEnvelope(TYPE_ORDER_CANCELLED, 1, order.id, data, at));
}

export async function publishShipped(tx: Tx, order: OrderWithLines, at: Date): Promise<void> {
  const data: OrderShippedV1 = {
    ...parties(order),
    tracking: order.tracking,
    shipped_at: at.toISOString(),
  };
  await enqueue(tx, newEnvelope(TYPE_ORDER_SHIPPED, 1, order.id, data, at));
}

export async function publishCompleted(tx: Tx, order: OrderWithLines, at: Date): Promise<void> {
  const data: OrderCompletedV1 = {
    ...parties(order),
    currency: order.currency,
    total: Number(order.total ?? 0n),
    payment_method: order.paymentMethod,
    completed_at: at.toISOString(),
  };
  await enqueue(tx, newEnvelope(TYPE_ORDER_COMPLETED, 1, order.id, data, at));
}

/** Locks the order row and returns it with its lines, or null when it does not exist. */
export async function lockOrder(tx: Tx, orderId: string): Promise<OrderWithLines | null> {
  await tx.$queryRaw`SELECT 1 FROM orders WHERE id = ${orderId}::uuid FOR UPDATE`;
  return tx.order.findUnique({ where: { id: orderId }, include: { lines: true } });
}
