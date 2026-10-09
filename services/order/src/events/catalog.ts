// Event types. The JSON Schema of each lives in packages/contracts/events.
export const TYPE_SHOP_UPDATED = 'poro.shop.shop.updated';
export const TYPE_PRODUCT_UPDATED = 'poro.shop.product.updated';
export const TYPE_PRODUCT_DELETED = 'poro.shop.product.deleted';
export const TYPE_STOCK_RESERVED = 'poro.shop.stock.reserved';
export const TYPE_STOCK_REJECTED = 'poro.shop.stock.rejected';
export const TYPE_ORDER_CREATED = 'poro.order.order.created';
export const TYPE_ORDER_PLACED = 'poro.order.order.placed';
export const TYPE_ORDER_CANCELLED = 'poro.order.order.cancelled';
export const TYPE_ORDER_SHIPPED = 'poro.order.order.shipped';
export const TYPE_ORDER_COMPLETED = 'poro.order.order.completed';
export const TYPE_PAYMENT_SUCCEEDED = 'poro.payment.payment.succeeded';
export const TYPE_PAYMENT_FAILED = 'poro.payment.payment.failed';
export const TYPE_REFUND_SUCCEEDED = 'poro.payment.refund.succeeded';

export type CancelReason =
  'stock_rejected' | 'payment_timeout' | 'buyer_cancelled' | 'seller_cancelled';

/** Full snapshot of a shop: keep the latest updated_at. */
export interface ShopUpdatedV1 {
  shop_id: string;
  owner_id: string;
  name: string;
  handle: string;
  currency: string;
  status: string;
  updated_at: string;
}

export interface VariantV1 {
  variant_id: string;
  title: string;
  price: number;
  in_stock: boolean;
}

/** Full snapshot of a product without its stock: keep the latest updated_at. */
export interface ProductUpdatedV1 {
  product_id: string;
  shop_id: string;
  title: string;
  currency: string;
  status: string;
  image_urls: string[];
  variants: VariantV1[];
  updated_at: string;
}

export interface ProductDeletedV1 {
  product_id: string;
  shop_id: string;
  deleted_at: string;
}

export interface OrderLineV1 {
  product_id: string;
  variant_id: string;
  quantity: number;
  unit_price: number;
}

/** Published at checkout, one order per shop; shop answers with stock.reserved or .rejected. */
export interface OrderCreatedV1 {
  order_id: string;
  buyer_id: string;
  shop_id: string;
  seller_id: string;
  currency: string;
  payment_method: string;
  items: OrderLineV1[];
  created_at: string;
}

export interface ReservedLineV1 extends OrderLineV1 {
  title: string;
}

/** Every line is held; prices and titles here are authoritative. */
export interface StockReservedV1 {
  order_id: string;
  shop_id: string;
  currency: string;
  items: ReservedLineV1[];
  subtotal: number;
  reserved_at: string;
}

export interface StockRejectedV1 {
  order_id: string;
  shop_id: string;
  reason: string;
  variant_ids: string[];
  rejected_at: string;
}

/** Payment charges total. expires_at is null for cash on delivery. */
export interface OrderPlacedV1 {
  order_id: string;
  buyer_id: string;
  shop_id: string;
  seller_id: string;
  currency: string;
  total: number;
  payment_method: string;
  placed_at: string;
  expires_at: string | null;
}

/** Shop releases held stock; payment refunds when paid is true. */
export interface OrderCancelledV1 {
  order_id: string;
  buyer_id: string;
  shop_id: string;
  seller_id: string;
  reason: CancelReason;
  paid: boolean;
  cancelled_at: string;
}

export interface OrderShippedV1 {
  order_id: string;
  buyer_id: string;
  shop_id: string;
  seller_id: string;
  tracking: string | null;
  shipped_at: string;
}

/** The sale is final: total becomes payable to the seller. */
export interface OrderCompletedV1 {
  order_id: string;
  buyer_id: string;
  shop_id: string;
  seller_id: string;
  currency: string;
  total: number;
  payment_method: string;
  completed_at: string;
}

export interface PaymentSucceededV1 {
  payment_id: string;
  order_id: string;
  buyer_id: string;
  amount: number;
  currency: string;
  provider: string;
  succeeded_at: string;
}

export interface PaymentFailedV1 {
  payment_id: string;
  order_id: string;
  buyer_id: string;
  provider: string;
  reason: string;
  failed_at: string;
}

export interface RefundSucceededV1 {
  refund_id: string;
  payment_id: string;
  order_id: string;
  buyer_id: string;
  amount: number;
  currency: string;
  provider: string;
  refunded_at: string;
}
