import { Type } from 'class-transformer';
import {
  IsBoolean,
  IsIn,
  IsInt,
  IsOptional,
  IsString,
  IsUUID,
  Max,
  MaxLength,
  Min,
  ValidateNested,
} from 'class-validator';

import { PAYMENT_METHODS, type PaymentMethod } from '../config/app.config';
import {
  DeliveryInputDto,
  PAYMENT_METHOD_LABELS,
  toDeliveryView,
  type DeliveryView,
} from '../delivery/delivery';
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

/** Step 1: the recap. Nothing is ordered yet. */
export class PreviewDto {
  @ValidateNested()
  @Type(() => DeliveryInputDto)
  delivery!: DeliveryInputDto;

  @IsOptional()
  @IsIn(PAYMENT_METHODS)
  payment_method?: PaymentMethod;

  /** Keep this delivery in the address book once the order is confirmed. */
  @IsOptional()
  @IsBoolean()
  save_address?: boolean;

  @IsOptional()
  @IsString()
  @MaxLength(100)
  address_label?: string | null;
}

/** Step 2: the buyer explicitly confirms the recap they reviewed. */
export class ConfirmDto {
  @IsUUID()
  preview_id!: string;

  @IsBoolean()
  confirmed!: boolean;
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

export interface OrderView {
  order_id: string;
  checkout_id: string;
  status: OrderStatus;
  shop: { shop_id: string; name: string };
  buyer_id: string;
  seller_id: string;
  currency: string;
  payment_method: PaymentMethod;
  /** For instance "Paiement à la livraison". */
  payment_method_label: string;
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
  /** True when the delivery was added to the address book. */
  address_saved: boolean;
}

export interface PreviewLineView {
  product_id: string;
  variant_id: string;
  title: string;
  variant_title: string;
  image_url: string | null;
  quantity: number;
  unit_price: number;
  line_total: number;
}

export interface PreviewShopView {
  shop: { shop_id: string; name: string; handle: string };
  currency: string;
  items: PreviewLineView[];
  subtotal: number;
}

/** The full recap shown before the buyer confirms. */
export interface PreviewView {
  preview_id: string;
  expires_at: string;
  shops: PreviewShopView[];
  /** Total of the items per currency; delivery fees are not included. */
  totals: { currency: string; amount: number }[];
  item_count: number;
  delivery: DeliveryView;
  payment_method: PaymentMethod;
  payment_method_label: string;
  delivery_fee_notice: string;
}

export interface PrefillView {
  /** address_book: from a saved address; account: from the account; empty: nothing known. */
  source: 'address_book' | 'account' | 'empty';
  address_id: string | null;
  delivery: {
    full_name: string | null;
    phone: string | null;
    city: string | null;
    address: string | null;
    landmark: string | null;
    location: DeliveryView['location'];
  };
  /** Whether auth and user answered; skipped when a saved address was used. */
  account_lookup: 'ok' | 'partial' | 'unavailable' | 'skipped';
}

export interface Page<T> {
  items: T[];
  next_cursor: string | null;
}

export type OrderWithLines = Order & { lines: OrderLine[] };

const iso = (at: Date | null): string | null => at?.toISOString() ?? null;

export function toOrderView(order: OrderWithLines): OrderView {
  return {
    order_id: order.id,
    checkout_id: order.checkoutId,
    status: order.status as OrderStatus,
    shop: { shop_id: order.shopId, name: order.shopName },
    buyer_id: order.buyerId,
    seller_id: order.sellerId,
    currency: order.currency,
    payment_method: order.paymentMethod as PaymentMethod,
    payment_method_label: PAYMENT_METHOD_LABELS[order.paymentMethod] ?? order.paymentMethod,
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
    delivery: toDeliveryView(order),
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
