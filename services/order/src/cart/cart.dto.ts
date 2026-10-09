import { IsInt, Max, Min } from 'class-validator';

export const MAX_CART_LINES = 100;
export const MAX_QUANTITY = 99;

export class SetQuantityDto {
  @IsInt()
  @Min(1)
  @Max(MAX_QUANTITY)
  quantity!: number;
}

export interface CartLineView {
  variant_id: string;
  product_id: string | null;
  title: string | null;
  variant_title: string | null;
  image_url: string | null;
  quantity: number;
  unit_price: number | null;
  line_total: number | null;
  in_stock: boolean;
  /** False when the variant, its product or its shop can no longer be bought. */
  available: boolean;
}

export interface CartShopView {
  shop: { shop_id: string; name: string; handle: string; currency: string };
  items: CartLineView[];
  /** Sum of the available lines, in minor units of currency. */
  subtotal: number;
}

export interface CartView {
  shops: CartShopView[];
  /** Lines whose variant the catalogue does not know or no longer sells. */
  unavailable: CartLineView[];
  item_count: number;
}
