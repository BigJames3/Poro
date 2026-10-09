// Event types. The JSON Schema of each lives in packages/contracts/events.
export const TYPE_SHOP_CREATED = 'poro.shop.shop.created';
export const TYPE_SHOP_UPDATED = 'poro.shop.shop.updated';
export const TYPE_PRODUCT_UPDATED = 'poro.shop.product.updated';
export const TYPE_PRODUCT_DELETED = 'poro.shop.product.deleted';
export const TYPE_STOCK_RESERVED = 'poro.shop.stock.reserved';
export const TYPE_STOCK_REJECTED = 'poro.shop.stock.rejected';
export const TYPE_ORDER_CREATED = 'poro.order.order.created';
export const TYPE_ORDER_CANCELLED = 'poro.order.order.cancelled';
export const TYPE_ORDER_COMPLETED = 'poro.order.order.completed';

export const COUNTRIES = ['CI', 'SN', 'CM', 'NG'] as const;
export type CountryCode = (typeof COUNTRIES)[number];
export type Currency = 'XOF' | 'XAF' | 'NGN';
export type ShopStatus = 'active' | 'suspended' | 'closed';
export type ProductStatus = 'draft' | 'active' | 'archived';
export type StockRejectedReason =
  'out_of_stock' | 'product_unavailable' | 'shop_unavailable' | 'currency_mismatch';

/** The currency is set by the country of the shop. */
export const CURRENCY_OF: Record<CountryCode, Currency> = {
  CI: 'XOF',
  SN: 'XOF',
  CM: 'XAF',
  NG: 'NGN',
};

/** Published when an account opens its shop. Auth grants BUSINESS to owner_id. */
export interface ShopCreatedV1 {
  shop_id: string;
  owner_id: string;
  name: string;
  handle: string;
  country_code: CountryCode;
  currency: Currency;
  created_at: string;
}

/** Full snapshot of a shop: consumers keep the latest updated_at. */
export interface ShopUpdatedV1 {
  shop_id: string;
  owner_id: string;
  name: string;
  handle: string;
  description: string | null;
  logo_url: string | null;
  country_code: CountryCode;
  currency: Currency;
  status: ShopStatus;
  updated_at: string;
}

export interface VariantV1 {
  variant_id: string;
  title: string;
  /** Minor units: XOF and XAF have none, NGN is in kobo. */
  price: number;
  /** A hint: the stock reservation decides. */
  in_stock: boolean;
}

/** Full snapshot of a product without its stock: consumers keep the latest updated_at. */
export interface ProductUpdatedV1 {
  product_id: string;
  shop_id: string;
  owner_id: string;
  title: string;
  description: string | null;
  currency: Currency;
  status: ProductStatus;
  image_urls: string[];
  variants: VariantV1[];
  updated_at: string;
}

export interface ProductDeletedV1 {
  product_id: string;
  shop_id: string;
  owner_id: string;
  deleted_at: string;
}

export interface OrderLineV1 {
  product_id: string;
  variant_id: string;
  quantity: number;
  unit_price: number;
}

/** Published by order at checkout, one order per shop. */
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
  currency: Currency;
  items: ReservedLineV1[];
  subtotal: number;
  reserved_at: string;
}

export interface StockRejectedV1 {
  order_id: string;
  shop_id: string;
  reason: StockRejectedReason;
  variant_ids: string[];
  rejected_at: string;
}

/** Shop releases the stock it holds for the order. */
export interface OrderCancelledV1 {
  order_id: string;
  buyer_id: string;
  shop_id: string;
  seller_id: string;
  reason: string;
  paid: boolean;
  cancelled_at: string;
}

/** The sale is final: the held stock leaves the shop. */
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
