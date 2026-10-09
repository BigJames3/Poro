import { Type } from 'class-transformer';
import {
  IsIn,
  IsInt,
  IsOptional,
  IsString,
  Matches,
  Max,
  MaxLength,
  Min,
  ValidateNested,
} from 'class-validator';

import { PAYMENT_METHODS, type PaymentMethod } from '../config/app.config';
import type { CancelReason } from '../events/catalog';
import { Order, OrderLine } from '../generated/prisma/client';

export const ORDER_STATUSES = [
  'pending',
  'awaiting_payment',
  'confirmed',
  'paid',
  'shipped',
  'completed',
  'cancelled',
] as const;
export type OrderStatus = (typeof ORDER_STATUSES)[number];
export const MAX_LINES_PER_ORDER = 50;

export class DeliveryDto {
  @IsString()
  @MaxLength(200)
  name!: string;

  /** E.164, for instance +2250700000000. */
  @IsString()
  @Matches(/^\+[1-9]\d{7,14}$/, {
    message: 'phone must be in international format, e.g. +2250700000000',
  })
  phone!: string;

  @IsString()
  @MaxLength(200)
  city!: string;

  @IsString()
  @MaxLength(1000)
  address!: string;
}

export class CheckoutDto {
  @IsIn(PAYMENT_METHODS)
  payment_method!: PaymentMethod;

  @ValidateNested()
  @Type(() => DeliveryDto)
  delivery!: DeliveryDto;
}

export class ShipDto {
  @IsOptional()
  @IsString()
  @MaxLength(400)
  tracking?: string | null;
}

export class OrderListQueryDto {
  @IsOptional()
  @IsString()
  @MaxLength(200)
  cursor?: string;

  @IsOptional()
  @Type(() => Number)
  @IsInt()
  @Min(1)
  @Max(50)
  limit?: number;

  @IsOptional()
  @IsIn(ORDER_STATUSES)
  status?: OrderStatus;
}

export interface OrderLineView {
  product_id: string;
  variant_id: string;
  title: string;
  quantity: number;
  /** Authoritative once the stock is reserved, else the price the buyer saw. */
  unit_price: number;
  unit_price_seen: number;
}

export interface DeliveryView {
  name: string;
  phone: string;
  city: string;
  address: string;
}

export interface OrderView {
  order_id: string;
  checkout_id: string;
  status: OrderStatus;
  shop: { shop_id: string; name: string };
  buyer_id: string;
  seller_id: string;
  currency: string;
  payment_method: PaymentMethod;
  items: OrderLineView[];
  subtotal_seen: number;
  /** Null until the shop reserved the stock. */
  total: number | null;
  /** Null once erased after the retention period. */
  delivery: DeliveryView | null;
  tracking: string | null;
  cancel_reason: CancelReason | null;
  paid: boolean;
  last_payment_error: string | null;
  expires_at: string | null;
  placed_at: string | null;
  paid_at: string | null;
  shipped_at: string | null;
  completed_at: string | null;
  cancelled_at: string | null;
  refunded_at: string | null;
  created_at: string;
}

export interface CheckoutView {
  checkout_id: string;
  orders: OrderView[];
}

export interface Page<T> {
  items: T[];
  next_cursor: string | null;
}

export type OrderWithLines = Order & { lines: OrderLine[] };

const iso = (at: Date | null): string | null => at?.toISOString() ?? null;

export function toOrderView(order: OrderWithLines): OrderView {
  const contact =
    order.contactName !== null &&
    order.contactPhone !== null &&
    order.city !== null &&
    order.address !== null
      ? {
          name: order.contactName,
          phone: order.contactPhone,
          city: order.city,
          address: order.address,
        }
      : null;
  return {
    order_id: order.id,
    checkout_id: order.checkoutId,
    status: order.status as OrderStatus,
    shop: { shop_id: order.shopId, name: order.shopName },
    buyer_id: order.buyerId,
    seller_id: order.sellerId,
    currency: order.currency,
    payment_method: order.paymentMethod as PaymentMethod,
    items: [...order.lines]
      .sort((a, b) => a.position - b.position)
      .map((line) => ({
        product_id: line.productId,
        variant_id: line.variantId,
        title: line.title,
        quantity: line.quantity,
        unit_price: Number(line.unitPrice ?? line.unitPriceSeen),
        unit_price_seen: Number(line.unitPriceSeen),
      })),
    subtotal_seen: Number(order.subtotalSeen),
    total: order.total === null ? null : Number(order.total),
    delivery: contact,
    tracking: order.tracking,
    cancel_reason: order.cancelReason as CancelReason | null,
    paid: order.paid,
    last_payment_error: order.lastPaymentError,
    expires_at: iso(order.expiresAt),
    placed_at: iso(order.placedAt),
    paid_at: iso(order.paidAt),
    shipped_at: iso(order.shippedAt),
    completed_at: iso(order.completedAt),
    cancelled_at: iso(order.cancelledAt),
    refunded_at: iso(order.refundedAt),
    created_at: order.createdAt.toISOString(),
  };
}
